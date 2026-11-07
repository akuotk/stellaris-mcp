package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestRelicsSavedValuesAndExactLocalization(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "relics_l_english.yml"), []byte("l_english:\n r_first:0 \"Example Relic\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	oldLoc := loc
	loc = gamestate.NewLocalizer(dir)
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`country={0={relics={r_first r_unknown r_first} last_activated_relic=0 last_received_relic=unknown}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderRelics(0, &c))
	for _, want := range []string{
		`Entry 0 key="r_first" name="Example Relic"`,
		"Entry 1 key=\"r_unknown\"\n",
		`Entry 2 key="r_first" name="Example Relic" [duplicate saved key]`,
		`last_activated_relic="0" (saved scalar)`,
		`last_received_relic="unknown" (saved scalar)`,
		"current activation eligibility remain unverified",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}

func TestRelicsRegisteredHandlerAndOptionalStates(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={relics={}}
	1={relics={r_invalid} relics={r_other} last_activated_relic=0 last_activated_relic=1 last_received_relic=2} 2={}}`)}})
	s := server.NewMCPServer("test", "test")
	registerRelics(s)
	if _, ok := s.ListTools()["get_relics"]; !ok {
		t.Fatal("relic tool missing")
	}
	for _, tc := range []struct {
		args map[string]any
		want []string
	}{
		{nil, []string{"relics (saved order)\n  Empty", "last_received_relic not recorded"}},
		{map[string]any{"country_id": 1}, []string{"duplicate relics fields", "last_activated_relic unsupported or duplicate", `last_received_relic="2"`}},
		{map[string]any{"country_id": 2}, []string{"relics (saved order)\n  Not recorded"}},
	} {
		result, err := handleRelics(context.Background(), rawRequest(tc.args))
		if err != nil || result.IsError {
			t.Fatal("normal relic handler failed")
		}
		for _, want := range tc.want {
			if !strings.Contains(resultText(result), want) {
				t.Fatalf("missing handler output %q", want)
			}
		}
	}
}
