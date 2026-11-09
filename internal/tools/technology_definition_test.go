package tools

import (
	"archive/zip"
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

func technologyGameFixture(t *testing.T, script string) (root, locale string) {
	t.Helper()
	root = t.TempDir()
	locale = filepath.Join(root, "localisation", "english")
	for _, dir := range []string{locale, filepath.Join(root, "common", "technology"), filepath.Join(root, "events"), filepath.Join(root, "dlc")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	writeTechnologyFixture(t, root, "stellaris.exe", "")
	writeTechnologyFixture(t, root, "launcher-settings.json", "")
	writeTechnologyFixture(t, root, "common/technology/synthetic.txt", script)
	return root, locale
}

func writeTechnologyFixture(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func technologyResult(t *testing.T, source gamestate.TechnologyDefinitionSource, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := handleTechnologyDefinition(context.Background(), rawRequest(args), source)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func technologyContains(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		at := strings.Index(got, want)
		if at < 0 {
			t.Fatalf("missing or out-of-order %q", want)
		}
		got = got[at+len(want):]
	}
}

func TestTechnologyDefinitionLocalSources(t *testing.T) {
	root, locale := technologyGameFixture(t, `@cost=80
tech_sample={area=physics category={research} tier=2 cost=@cost prerequisites={"tech_base" condition={yes=no}}
modifier={bonus=0.25} feature_flags={feature} cost=90}
tech_base={}`)
	f, err := os.Create(filepath.Join(root, "dlc", "synthetic.zip"))
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for _, name := range []string{"bundle/common/technology/duplicate.txt", "events/ignored.txt"} {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("tech_sample={tier=7}")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	oldDir, oldLoc := saveDir, loc
	t.Cleanup(func() { saveDir, loc = oldDir, oldLoc })
	s := server.NewMCPServer("test", "test")
	RegisterWithInstalledScripts(s, filepath.Join(t.TempDir(), "absent-saves"), loc, locale)
	entry, ok := s.ListTools()["get_technology_definition"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only registration")
	}
	result, err := entry.Handler(context.Background(), rawRequest(map[string]any{"technology_id": "tech_sample"}))
	if err != nil || result.IsError {
		t.Fatal("static query failed with no save directory")
	}
	got := resultText(result)
	technologyContains(t, got, "source_status=complete match_status=multiple definition_matches=2",
		`definition[0] source=common/technology field[0] key="area": "physics"`,
		`field[3] key="cost" [duplicate field]: "@cost" (raw scalar; unquoted)`,
		`field[4] key="prerequisites": mixed static block with 2 ordered entries`,
		`entry[0]: "tech_base" (raw scalar; quoted)`,
		`entry[1] key="condition": object static block`,
		`field[5] key="modifier": object static block`,
		`field[6] key="feature_flags": list static block`,
		`field[7] key="cost" [duplicate field]: "90"`,
		`definition[1] source=dlc-zip field[0] key="tier": "7"`)
	if strings.Contains(got, root) || strings.Contains(got, ".txt") || strings.Contains(got, "Saved research key matches") {
		t.Fatal("static response included unrelated source or save details")
	}
	source := gamestate.NewTechnologyDefinitionSource(locale)
	technologyContains(t, resultText(technologyResult(t, source, map[string]any{"technology_id": "tech_base"})),
		"source_status=complete match_status=unique definition_matches=1", "empty static block")
	technologyContains(t, resultText(technologyResult(t, source, map[string]any{"technology_id": "Tech_Sample"})),
		"source_status=complete match_status=unmatched definition_matches=0")
}

func TestTechnologyDefinitionPartialAndBounds(t *testing.T) {
	root, locale := technologyGameFixture(t, "tech_sample={cost=5 prerequisites={"+strings.Repeat(" secret-item", 25)+"}} tech_shape=odd inline_script={script=dynamic}")
	writeTechnologyFixture(t, root, "common/technology/broken.txt", "tech_incomplete={")
	source := gamestate.NewTechnologyDefinitionSource(locale)
	got := resultText(technologyResult(t, source, map[string]any{"technology_id": "tech_sample"}))
	technologyContains(t, got, "source_status=partial match_status=unique-observed definition_matches=1",
		`key="prerequisites": list static block with 25 entries; inspect the local source`)
	if strings.Contains(got, "secret-item") || strings.Contains(got, "read_save_") {
		t.Fatal("unbounded list or invalid raw fallback")
	}
	technologyContains(t, resultText(technologyResult(t, source, map[string]any{"technology_id": "tech_shape"})),
		"source_status=partial match_status=unique-observed", "(unsupported definition)")
	technologyContains(t, resultText(technologyResult(t, source, map[string]any{"technology_id": "tech_incomplete"})),
		"source_status=partial match_status=unresolved definition_matches=0")
	technologyContains(t, resultText(technologyResult(t, nil, map[string]any{"technology_id": "tech_sample"})),
		"source_status=unavailable match_status=unavailable")
}

func TestTechnologyDefinitionSaveMatchesAndContinuation(t *testing.T) {
	var script strings.Builder
	script.WriteString("tech_sample={")
	for i := 0; i < 22; i++ {
		fmt.Fprintf(&script, " field%d=%d", i, i)
	}
	script.WriteString("} tech_other={tier=1}")
	root, locale := technologyGameFixture(t, script.String())
	const fixture = `player={{country=0}} country={
	0={tech_status={technology=tech_sample level=1 technology=tech_sample level=2
		physics_queue={{technology=tech_sample} {special_project=0} {technology=tech_sample technology=tech_other}}
		society_queue={}
		alternatives={physics={tech_sample tech_sample} society={tech_other}}}}
	1={tech_status={technology=tech_other level=1}}}`
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	source := gamestate.NewTechnologyDefinitionSource(locale)
	first := resultText(technologyResult(t, source, map[string]any{"technology_id": "tech_sample", "include_save_status": true}))
	technologyContains(t, first, "fields=22 offset=0", "entry[19]", "next_offset=20",
		"=== Saved research key matches ===", "completed matches=2", "physics_queue partial observed_matches=1 special_project_only=1",
		"society_queue empty", "engineering_queue not recorded", "alternatives.physics matches=2",
		"alternatives.society matches=0", "alternatives.engineering not recorded")
	if strings.Contains(first, "entry[20]") {
		t.Fatal("definition page exceeds 20 fields")
	}
	var token, saveID string
	for _, line := range strings.Split(first, "\n") {
		if value, ok := strings.CutPrefix(line, "page_context="); ok {
			token = value
		}
		if value, ok := strings.CutPrefix(line, "save_id="); ok {
			saveID = value
		}
	}
	if token == "" || saveID == "" {
		t.Fatal("missing saved continuation context")
	}
	args := map[string]any{"technology_id": "tech_sample", "include_save_status": true, "save_id": saveID, "page_context": token, "offset": 20}
	technologyContains(t, resultText(technologyResult(t, source, args)), "offset=20", "entry[20]", "entry[21]", "next_offset=none")
	for _, invalid := range []map[string]any{
		{"technology_id": "tech_sample", "offset": 20},
		{"technology_id": "tech_sample", "offset": 20, "page_context": token},
		{"technology_id": "tech_other", "include_save_status": true, "offset": 20, "page_context": token},
		{"technology_id": "tech_sample", "include_save_status": true, "country_id": 1, "offset": 20, "page_context": token},
		{"technology_id": "tech_sample", "include_save_status": "yes"},
		{"technology_id": "tech_sample", "country_id": 0},
		{"technology_id": "tech_sample", "save_id": saveID},
	} {
		if !technologyResult(t, source, invalid).IsError {
			t.Fatalf("accepted invalid request %v", invalid)
		}
	}
	writeTechnologyFixture(t, root, "common/technology/synthetic.txt", script.String()+" changed={}")
	if !technologyResult(t, gamestate.NewTechnologyDefinitionSource(locale), args).IsError {
		t.Fatal("changed source accepted the continuation")
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(fixture + " changed=yes")}})
	if !technologyResult(t, source, args).IsError {
		t.Fatal("changed save accepted the continuation")
	}
}

func TestTechnologyDefinitionQueueProjectShapes(t *testing.T) {
	for _, tc := range []struct {
		name, queue, want string
	}{
		{"empty queue", `{}`, "empty"},
		{"project only", `{{special_project=0} {special_project=1}}`, "matches=0 special_project_only=2"},
		{"technology only", `{{technology=tech_other}}`, "matches=0"},
		{"matching technology only", `{{technology=tech_sample}}`, "matches=1"},
		{"technology with project ID zero", `{{technology=tech_sample special_project=0}}`, "matches=1 technology_and_special_project=1"},
		{"nonmatching technology with project", `{{technology=tech_other special_project=0}}`, "matches=0 technology_and_special_project=1"},
		{"empty item", `{{}}`, "partial observed_matches=0"},
		{"unknown item", `{{progress=0}}`, "partial observed_matches=0"},
		{"empty project scalar", `{{special_project=""}}`, "partial observed_matches=0"},
		{"noninteger project", `{{special_project=unknown}}`, "partial observed_matches=0"},
		{"negative project", `{{special_project=-1}}`, "partial observed_matches=0"},
		{"empty project block", `{{special_project={}}}`, "partial observed_matches=0"},
		{"duplicate project", `{{special_project=0 special_project=0}}`, "partial observed_matches=0"},
		{"valid and unknown", `{{special_project=0} {progress=0}}`, "partial observed_matches=0 special_project_only=1"},
		{"valid technology with invalid project", `{{technology=tech_sample special_project={}}}`, "partial observed_matches=1"},
		{"valid technology with duplicate project", `{{technology=tech_sample special_project=0 special_project=0}}`, "partial observed_matches=1"},
		{"duplicate technology with valid project", `{{technology=tech_sample technology=tech_sample special_project=0}}`, "partial observed_matches=0"},
		{"invalid technology with valid project", `{{technology={} special_project=0}}`, "partial observed_matches=0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			save := "country={0={tech_status={physics_queue=" + tc.queue + "}}}"
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(save)}})
			args := map[string]any{"technology_id": "tech_sample", "include_save_status": true, "country_id": 0}
			result := technologyResult(t, nil, args)
			if result.IsError || !strings.Contains(resultText(result), "  physics_queue "+tc.want+"\n") {
				t.Fatalf("expected physics_queue %s", tc.want)
			}
		})
	}
}
