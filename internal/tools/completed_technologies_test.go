package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func completedTechnologyCall(t *testing.T, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := handleCompletedTechnologies(context.Background(), rawRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCompletedTechnologiesSavedOrderAndLevels(t *testing.T) {
	const fixture = `unrelated=0 player={{country=0}} country={1={} 0={other=0
tech_status={technology=tech_alpha level=01 technology=tech_alpha other=yes level=9
technology={key=x anonymous} level=4 technology=tech_beta level=bad}
tech_status=unsupported
tech_status={level=7 technology=tech_gamma}
tech_status={}
tech_status={physics_queue={}}}}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "synthetic_l_english.yml"), []byte("l_english:\n tech_alpha:0 \"Localized Alpha\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	old := loc
	loc = gamestate.NewLocalizer(dir)
	t.Cleanup(func() { loc = old })
	result := completedTechnologyCall(t, nil)
	if result.IsError {
		t.Fatal("read failed")
	}
	technologyContains(t, resultText(result),
		"country_id=0",
		"tech_status=duplicate tech_status_occurrences=5 unsupported_tech_status_occurrences=1 technology_entries=5 saved_entries=10 offset=0",
		`entry[0] path="/2/1/1/0" kind=technology repeated_saved_key=true technology_key="tech_alpha" localized_name="Localized Alpha"`,
		`level=1 level_path="/2/1/1/1" (adjacent saved integer)`,
		`entry[1] path="/2/1/1/2" kind=technology repeated_saved_key=true technology_key="tech_alpha"`,
		"level_status=not-adjacent-or-not-recorded",
		`entry[2] path="/2/1/1/4" kind=unpaired-level: "9" (saved scalar)`,
		`entry[3] path="/2/1/1/5" kind=technology technology_status=unsupported: mixed saved block`,
		`level_status=unresolved level_path="/2/1/1/6": "4" (saved scalar)`,
		`entry[4] path="/2/1/1/7" kind=technology technology_key="tech_beta" localization=unavailable`,
		`level_status=unresolved level_path="/2/1/1/8": "bad" (saved scalar)`,
		`entry[5] path="/2/1/2" kind=unsupported-tech-status: "unsupported"`,
		`entry[6] path="/2/1/3/0" kind=unpaired-level: "7"`,
		`entry[7] path="/2/1/3/1" kind=technology technology_key="tech_gamma"`,
		"level_status=not-adjacent-or-not-recorded",
		`entry[8] path="/2/1/4" kind=empty-tech-status: empty object`,
		`entry[9] path="/2/1/5" kind=no-completed-fields`,
		"next_offset=none")
}

func TestCompletedTechnologiesTechStatusStates(t *testing.T) {
	for _, fixture := range []struct{ name, country, want string }{
		{"missing", "", "tech_status=not-recorded tech_status_occurrences=0"},
		{"empty", "tech_status={}", "tech_status=empty tech_status_occurrences=1"},
		{"no completed fields", "tech_status={physics_queue={}}", "tech_status=no-completed-technologies"},
		{"unknown", "tech_status={x=y anonymous}", "tech_status=unsupported tech_status_occurrences=1"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			save := "country={0={" + fixture.country + "}}"
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(save)}})
			result := completedTechnologyCall(t, map[string]any{"country_id": 0})
			if result.IsError || !strings.Contains(resultText(result), fixture.want) {
				t.Fatalf("expected %q", fixture.want)
			}
		})
	}
}

func TestCompletedTechnologiesPagination(t *testing.T) {
	var b strings.Builder
	b.WriteString("player={{country=0} {country=0}} country={0={tech_status={")
	for i := 0; i < 53; i++ {
		fmt.Fprintf(&b, " technology=tech_%d level=%d", i, i)
	}
	b.WriteString("}} 1={tech_status={technology=other}}}")
	fixture := b.String()
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	s := server.NewMCPServer("test", "test")
	Register(s, dir, loc)
	entry, ok := s.ListTools()["get_completed_technologies"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only registration")
	}
	first := completedTechnologyCall(t, nil)
	if first.IsError {
		t.Fatal("first page failed")
	}
	got := resultText(first)
	technologyContains(t, got, "saved_entries=53 offset=0", "entry[49]", "next_offset=50")
	if strings.Contains(got, "entry[50]") {
		t.Fatal("first page exceeds 50 entries")
	}
	var token, saveID string
	for _, line := range strings.Split(got, "\n") {
		if value, ok := strings.CutPrefix(line, "page_context="); ok {
			token = value
		}
		if value, ok := strings.CutPrefix(line, "save_id="); ok {
			saveID = value
		}
	}
	if token == "" || saveID == "" {
		t.Fatal("missing continuation context")
	}
	args := map[string]any{"country_id": 0, "save_id": saveID, "offset": 50, "page_context": token}
	second := completedTechnologyCall(t, args)
	if second.IsError {
		t.Fatal("second page failed")
	}
	technologyContains(t, resultText(second), "saved_entries=53 offset=50", "entry[50]", "entry[51]", "entry[52]", "next_offset=none")
	for _, invalid := range []map[string]any{
		{"country_id": 1, "offset": 50, "page_context": token},
		{"country_id": "0"},
		{"country_id": -1},
		{"country_id": 0.5},
		{"save_id": 0},
		{"page_context": 0},
		{"offset": 50},
		{"offset": 54, "page_context": token},
	} {
		if !completedTechnologyCall(t, invalid).IsError {
			t.Fatalf("accepted invalid synthetic request %v", invalid)
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(fixture + " changed=yes")}})
	if stale := completedTechnologyCall(t, args); !stale.IsError || !strings.Contains(resultText(stale), "page_context") {
		t.Fatal("changed save revision accepted")
	}
}

func TestCompletedTechnologiesAmbiguousCountry(t *testing.T) {
	for _, fixture := range []struct {
		name, save, want string
		args             map[string]any
	}{
		{"multiple players", "player={{country=0} {country=1}} country={0={} 1={}}", "unique player country", nil},
		{"duplicate country table", "country={0={}} country={1={}}", "saved country table is ambiguous", map[string]any{"country_id": 0}},
		{"duplicate country key", "country={0={} 00={}}", "selected saved country is ambiguous", map[string]any{"country_id": 0}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture.save)}})
			result := completedTechnologyCall(t, fixture.args)
			if !result.IsError || !strings.Contains(resultText(result), fixture.want) {
				t.Fatalf("expected %q", fixture.want)
			}
		})
	}
}
