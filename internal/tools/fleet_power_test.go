package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestFleetSummaryCountsOnlyMilitaryClass(t *testing.T) {
	loc = gamestate.NewLocalizer(t.TempDir())
	gs := &gamestate.GameState{Fleet: map[int]gamestate.Fleet{
		1: {ShipClass: "shipclass_military", MilitaryPower: 100},
		2: {ShipClass: "shipclass_starbase", MilitaryPower: 500},
		3: {ShipClass: "shipclass_constructor", MilitaryPower: 10},
	}}
	c := &gamestate.Country{FleetsManager: gamestate.FleetsManager{OwnedFleets: []gamestate.OwnedFleet{{Fleet: 1}, {Fleet: 2}, {Fleet: 3}, {Fleet: 99}}}}
	result, err := fleetSummary(gs, c)
	if err != nil {
		t.Fatal(err)
	}
	content, ok := result.Content[0].(mcp.TextContent)
	if !ok || (!strings.Contains(content.Text, "Military-class fleet power: 100.0") || !strings.Contains(content.Text, "Incomplete: 1 referenced fleet records unavailable.")) {
		t.Fatalf("unexpected summary: %#v", result.Content)
	}
}

func TestFleetDetailPreservesReferencesAndDesignStages(t *testing.T) {
	loc = gamestate.NewLocalizer(t.TempDir())
	gs, err := gamestate.Load([]byte(`fleet={1={ships={1 2 3}}} ships={1={ship_design_implementation={design=0 growth_stage=1 upgrade=0}} 2={}} ship_design={0={growth_stages={ {ship_size=first section={slot=A component={slot=GUN template=weapon_one}}} {ship_size=second section={slot=B component={slot=GUN template=weapon_two}}} }}}`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := fleetDetail(gs, 1)
	if err != nil {
		t.Fatal(err)
	}
	content := result.Content[0].(mcp.TextContent).Text
	for _, want := range []string{"Design reference: 0", "growth_stage: 1 (index interpretation not established)", "implementation upgrade: 0", "Design reference: not recorded", "Ship 3: referenced record unavailable", "Saved design growth stage 0", "Saved design growth stage 1", "Weapon One", "Weapon Two", "actual stage not established"} {
		if !strings.Contains(content, want) {
			t.Errorf("missing %q in %s", want, content)
		}
	}
}
