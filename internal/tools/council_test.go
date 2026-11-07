package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestCouncilSavedScopesAndReciprocalReferences(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`country={0={owned_leaders={0 1} government={council_positions={0 1 3 7 8} picked_council_types={first mismatch third missing bad}
	 council_agenda=example council_agenda_progress=0 unlocked_civic_council_slots=0
	 council_agenda_cooldowns={council_agenda=first start_date=0} council_agenda_cooldowns={start_date=0 start_date=1}
	 council_agenda_cooldowns={council_agenda=third}}}}
	 council_positions={council_positions={0={country=0 leader=0 type=first} 1={country=1 leader=1 type=second}
	 2={country=0} 3={country=0 leader=99} 5={country=1} 8={country=0 leader=0 leader=1}}}
	 leaders={0={country=0 council_location={type=council_position id=0 position=0}} 1={country=0 council_location={type=council_position id=9}}
	 2={council_location={type=council_position id=2}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderCouncil(gs, 0, &c))
	for _, want := range []string{
		`council_agenda="example" council_agenda_progress="0" unlocked_civic_council_slots="0"`,
		`Cooldown record 0 (saved order) council_agenda="first" start_date="0"`, "Cooldown record 1 unsupported or duplicate",
		`Cooldown record 2 (saved order) council_agenda="third" start_date=not recorded`,
		"Position 0 country=0", "Reciprocal saved position/leader IDs agree", "Saved position.country differs from selected country",
		"Saved leader.country differs from position.country", "Position and leader council references disagree",
		`Same-index type candidate picked_council_types[1]="mismatch" differs from saved position.type`,
		"Leader 2 council_location points here. Saved position.leader=not recorded",
		"Position 3 country=0", "Referenced leader record unavailable", "Position 7 referenced record unavailable", "Position 8 referenced record unsupported or duplicate",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "Position 5 country=1") {
		t.Fatal("position without selected-country path was included")
	}
}

func TestCouncilAmbiguousGovernmentCannotScopePositions(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`country={0={government={council_positions={7} council_agenda=first} government={council_positions={8} council_agenda=second}}
	 1={government={council_positions={1} picked_council_types={first second}}}}
	 council_positions={council_positions={0={country=0} 1={country=1} 7={country=1}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderCouncil(gs, 0, &c))
	if !strings.Contains(text, "Government shape unsupported or duplicate") || !strings.Contains(text, "Position 0 country=0") || strings.Contains(text, "Position 7") || strings.Contains(text, `council_agenda="first"`) {
		t.Fatal("ambiguous government formed a scope or hid the valid root position")
	}
	c = gs.Country[1]
	text = resultText(renderCouncil(gs, 1, &c))
	if !strings.Contains(text, "Saved council position and type lists have different lengths") || strings.Contains(text, "Same-index type candidate picked_council_types[0]") {
		t.Fatal("unequal lists were aligned")
	}
}

func TestCouncilRegisteredHandler(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={government={council_agenda=example}}}`)}})
	s := server.NewMCPServer("test", "test")
	registerCouncil(s)
	if _, ok := s.ListTools()["get_council"]; !ok {
		t.Fatal("council tool missing")
	}
	result, err := handleCouncil(context.Background(), rawRequest(nil))
	if err != nil || result.IsError || !strings.Contains(resultText(result), `council_agenda="example"`) {
		t.Fatal("normal council handler failed")
	}
}
