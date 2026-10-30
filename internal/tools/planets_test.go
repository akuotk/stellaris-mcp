package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"strings"
	"testing"
)

func resultText(r *mcp.CallToolResult) string {
	var text string
	for _, c := range r.Content {
		if v, ok := c.(mcp.TextContent); ok {
			text += v.Text
		}
	}
	return text
}

func TestPlanetAvailableRequiresSurveyAndNoOwner(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`planets={planet={
  1={planet_class="pc_arid" name={key="Unsurveyed"}}
  2={planet_class="pc_arid" surveyed_by=0 name={key="Available"}}
  3={planet_class="pc_arid" surveyed_by=0 owner=0 name={key="OwnedZero"}}
  4={planet_class="pc_arid" surveyed_by=0 colony=0 name={key="ColonyZero"}}
 }}`))
	if err != nil {
		t.Fatal(err)
	}
	r, err := planetAvailable(gs, 0, &gamestate.Country{})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "Available") || strings.Contains(text, "Unsurveyed") || strings.Contains(text, "OwnedZero") || strings.Contains(text, "ColonyZero") {
		t.Fatal(text)
	}
}

func TestJobBreakdownPreservesFractionalWorkforce(t *testing.T) {
	gs := &gamestate.GameState{PopJobs: map[int]gamestate.PopJob{
		1: {Type: "miner", Planet: 3, Workforce: 1.25},
		2: {Type: "miner", Planet: 3, Workforce: 0.5},
		3: {Type: "unused", Planet: 3, Workforce: -1},
		4: {Type: "other", Planet: 4, Workforce: 100},
	}}
	got := jobBreakdown(gs, 3)
	if len(got) != 1 || got[0].workforce != 1.75 {
		t.Fatalf("workforce truncated or wrong jobs included: %+v", got)
	}
}

func TestPlanetSummaryUsesPlanetOwnerNotColonyIDs(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`planets={planet={
  3={colony=0 owner=0 name={key="OwnedHome"}}
  0={colony=2 owner=1 name={key="ForeignColony"}}
 }} colony={0={carrier={type=planet reference=3} num_sapient_pops=4200}}
 `))
	if err != nil {
		t.Fatal(err)
	}
	r, err := planetSummary(gs, 0, &gamestate.Country{OwnedPlanets: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "OwnedHome") || !strings.Contains(text, "4200") || strings.Contains(text, "ForeignColony") {
		t.Fatal(text)
	}
}

func TestJobBreakdownResolvesColonyID(t *testing.T) {
	colonyID := 0
	gs := &gamestate.GameState{
		Planets: gamestate.PlanetsWrapper{Planet: map[int]gamestate.Planet{3: {ColonyID: &colonyID}}},
		PopJobs: map[int]gamestate.PopJob{1: {Type: "miner", Planet: 0, Workforce: 3.5}, 2: {Type: "other", Planet: 3, Workforce: 99}},
	}
	got := jobBreakdown(gs, 3)
	if len(got) != 1 || got[0].jobType != "miner" || got[0].workforce != 3.5 {
		t.Fatalf("job colony ID treated as physical planet: %+v", got)
	}
}
