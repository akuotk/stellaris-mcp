package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
)

func TestNeighborsShowSavedValuesAndUnresolvedReferences(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={military_power=100 relations_manager={relation={country=1 contact=yes relation_current=0 trust=0} relation={country=99 contact=yes trust=3.25} relation={contact=yes relation_current=4 trust=5} relation={country=2} relation={country=3 contact=maybe}}} 1={adjective={key="Synthetic neighbour"} military_power=200 economy_power=0} 2={} 3={}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderNeighbors(gs, &c))
	for _, want := range []string{"relation_current=0 trust=0", "military=200 economy=0 tech=not recorded", "ratio (neighbor / selected): 2", "Contact country 99: referenced record unavailable", "country reference not recorded", "relation_current=4 trust=5", "country=2 contact=not recorded", "country=3 contact=maybe"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
	for _, bad := range []string{"Overwhelming", "Superior", "Equivalent", "Inferior", "Pathetic", "No saved contact=yes relations"} {
		if strings.Contains(text, bad) {
			t.Errorf("unexpected %q", bad)
		}
	}
}
func TestNeighborsRatioMissingAndZero(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	for _, src := range []string{`country={0={military_power=0 relations_manager={relation={country=1 contact=yes}}} 1={military_power=5}}`, `country={0={relations_manager={relation={country=1 contact=yes}}} 1={military_power=0}}`, `country={0={military_power=1e-300 relations_manager={relation={country=1 contact=yes}}} 1={military_power=1e300}}`} {
		gs, err := gamestate.Load([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		c := gs.Country[0]
		text := resultText(renderNeighbors(gs, &c))
		if !strings.Contains(text, "Derived military ratio: unavailable") {
			t.Fatal(text)
		}
	}
}

func TestFleetComparisonReportsMissingCountryAndPower(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={relations_manager={relation={country=1 contact=yes} relation={country=99 contact=yes} relation={contact=yes}}} 1={adjective={key="Example"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	result, err := fleetSummary(gs, &c)
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	for _, want := range []string{"Total Military Power: not recorded", "Example: not recorded military power", "Contact country 99: referenced record unavailable", "Contact country reference not recorded"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
}
