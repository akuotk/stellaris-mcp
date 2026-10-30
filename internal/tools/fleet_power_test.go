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
	c := &gamestate.Country{FleetsManager: gamestate.FleetsManager{OwnedFleets: []gamestate.OwnedFleet{{Fleet: 1}, {Fleet: 2}, {Fleet: 3}}}}
	result, err := fleetSummary(gs, c)
	if err != nil {
		t.Fatal(err)
	}
	content, ok := result.Content[0].(mcp.TextContent)
	if !ok || !strings.Contains(content.Text, "Military-class fleet power: 100.0") {
		t.Fatalf("unexpected summary: %#v", result.Content)
	}
}
