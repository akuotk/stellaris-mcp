package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestShipLoadoutLegacyGrowthAndExplicitFleetReference(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`fleet={0={ships={9}} 5={ships={0}} 6={ships={1 1}} 7={ships={2} ships={2}}}
	 ships={
	 0={fleet=5 ship_design=0 upgrade_progress=0.00000001 section={slot=FRONT design=front weapon={component_slot=GUN_A template=laser index=0 cooldown=123456789.123456789 target={id=0 type=7}} strike_craft={name=known}} section={slot=REAR design=rear weapon={component_slot=GUN_B template=beam} weapon={component_slot=DUP template=beam} weapon={component_slot=DUP template=beam}}}
	 1={fleet=6 ship_design_implementation={design=1 growth_stage=7 upgrade=0} section={slot=FRONT design=advanced weapon={component_slot=GUN template=laser}}}
	 2={fleet=7 ship_design=0 ship_design_implementation={design=1}}
	 3={ship_design=0 section={slot=FRONT slot=REAR design=front weapon={component_slot=GUN_A template=laser}}}
	 4=none 04={ship_design=0}
	 5={ship_design=0 ship_design=1 section={known=field anonymous}}
	 6={ship_design=2}
	 } ship_design={
	 0={name={key=legacy} ship_size=legacy_size required_component=core required_component=engine section={slot=FRONT template=front component={slot=GUN_A template=laser}} section={slot=REAR template=rear component={slot=GUN_B template=cannon} component={slot=DUP template=beam}}}
	 1={name={key=modern} growth_stages={{ship_size=small section={slot=FRONT template=basic component={slot=GUN template=laser}}} {ship_size=large section={slot=FRONT template=advanced component={slot=GUN template=laser}}}}}
	 2=none
	 3={growth_stages={} growth_stages={{ship_size=ambiguous}}}
	 }`)}})
	s := server.NewMCPServer("test", "test")
	registerShipLoadout(s)
	if _, ok := s.ListTools()["get_ship_loadout"]; !ok {
		t.Fatal("ship loadout tool missing")
	}
	for _, tc := range []struct {
		args  map[string]any
		want  []string
		avoid string
	}{
		{map[string]any{"ship_id": 0}, []string{
			"Ship 0 (ships table)", "A fleet record also uses this numeric ID", "ship.fleet=5", "Reciprocal saved ship.fleet and fleet.ships IDs agree", "ship_design=0 ship_design_implementation.design=not recorded", "Design 0 (ship_design table)",
			`key="upgrade_progress": "0.00000001"`, "Section[1] (saved order)", "weapon[2] (saved order)", `key="cooldown": "123456789.123456789"`, `key="type": "7"`, `key="index": "0"`, `key="strike_craft": object`, `key="required_component" [duplicate saved key]: "engine"`,
			"Saved legacy design layout", "Section comparison: exact saved section pairs agree", "matches=1 differs=1 unresolved=2",
		}, "Fleet 0:"},
		{map[string]any{"ship_id": 1}, []string{"growth_stage=7 upgrade=0", "Ambiguous fleet ships membership (2 duplicate saved references)", "Saved design growth-stage position 0", "Saved design growth-stage position 1", "Section comparison: saved section pairs differ", "Section comparison: exact saved section pairs agree", "matches=1 differs=0 unresolved=0"}, "Saved design growth-stage position 7"},
		{map[string]any{"ship_id": 2}, []string{"Fleet ships membership unresolved", "Design selection unresolved because saved references are unsupported or disagree"}, "Design 0 (ship_design table)"},
		{map[string]any{"ship_id": 3}, []string{"ship.fleet=not recorded", "section comparison unresolved (unsupported or duplicate slot/template)", "matches=0 differs=0 unresolved=1"}, "exact saved section pairs agree"},
		{map[string]any{"ship_id": 4}, []string{"duplicate saved record ID"}, "ship.fleet="},
		{map[string]any{"ship_id": 5}, []string{"ship_design=unsupported or duplicate", "mixed saved block", "Design selection unresolved"}, "Design 0 (ship_design table)"},
		{map[string]any{"ship_id": 6}, []string{`saved record is "none"`}, "Saved legacy design layout"},
		{map[string]any{"ship_id": 9}, []string{"record unavailable"}, "Saved legacy design layout"},
		{map[string]any{"design_id": 1}, []string{"Design 1 (ship_design table)", "Saved design growth-stage position 0", "Saved design growth-stage position 1", `key="template": "advanced"`}, "Section comparison:"},
		{map[string]any{"design_id": 3}, []string{"growth_stages unsupported or duplicate", "No decoded design layouts"}, "Saved design growth-stage position"},
	} {
		result, err := handleShipLoadout(context.Background(), rawRequest(tc.args))
		if err != nil || result.IsError {
			t.Fatal("normal ship loadout handler failed")
		}
		text := resultText(result)
		for _, want := range tc.want {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in %s", want, text)
			}
		}
		if strings.Contains(text, tc.avoid) {
			t.Fatalf("unexpected %q in %s", tc.avoid, text)
		}
	}
	for _, args := range []map[string]any{nil, {"ship_id": 0, "design_id": 0}, {"ship_id": 0.5}} {
		result, err := handleShipLoadout(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Fatal("invalid ship loadout selection accepted")
		}
	}
}

func TestShipLoadoutReportsDesignOnlySlots(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`ships={0={ship_design=0 section={slot=FRONT design=front weapon={component_slot=A template=laser}}}} ship_design={0={section={slot=FRONT template=front component={slot=A template=laser} component={slot=B template=shield} component={template=unknown}}}}`)}})
	result, err := handleShipLoadout(context.Background(), rawRequest(map[string]any{"ship_id": 0}))
	if err != nil || result.IsError {
		t.Fatal("ship loadout failed")
	}
	if got := resultText(result); !strings.Contains(got, "Design component entries without a matching saved weapon slot=1") {
		t.Fatal(got)
	}
	if got := resultText(result); !strings.Contains(got, "matches=1 differs=0 unresolved=1") {
		t.Fatal(got)
	}
}
