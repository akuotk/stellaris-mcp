package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
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

func TestNeighborsDetailKeepsSavedDirectionsAndAmbiguity(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	data := []byte(`player={{country=1}}
	country={0={adjective={key=Other} relations_manager={relation={owner=0 country=1 contact=yes trust=2.000 truce=2205.01.01}}}
	1={adjective={key=Home} relations_manager={
	relation={owner=1 country=0 contact=no relation_current=00.125 relation_last_month=-0.00 trust=-0.0000
	truce=2204.01.01 truce=2204.02.01 embassy=unknown research_agreement=yes closed_borders=yes
	modifier={modifier=first value=00.5} modifier={modifier=second value=-0.5}
	diplo_action_dates={2200.01.01 0} flags={saved_example=0} wars={1} alliance=yes}
	relation={owner=9 country=0 contact={unknown=yes}}
	relation={country=0 country=2}
	relation={country={unknown=yes}}
	relation={owner=1 trust=1}
	relation={owner=1 country=99 truce=unknown}
	relation=bad}}
	2={relations_manager={} relations_manager={relation={owner=2 country=1}}}}
	`)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", data}})
	s := server.NewMCPServer("test", "test")
	registerNeighbors(s)
	entry, ok := s.ListTools()["get_neighbors"]
	if !ok {
		t.Fatal("get_neighbors unavailable")
	}
	if _, ok := entry.Tool.InputSchema.Properties["neighbor_id"]; !ok {
		t.Fatal("neighbor_id parameter unavailable")
	}
	result, err := handleNeighbors(context.Background(), rawRequest(map[string]any{"neighbor_id": float64(0)}))
	if err != nil || result.IsError {
		t.Fatalf("get_neighbors detail failed: %v", err)
	}
	text := resultText(result)
	for _, want := range []string{
		"Country 1 relations_manager targeting country 0",
		"Country 0 relations_manager targeting country 1",
		"Unscoped rows: unsupported=2 duplicate country fields=1 country not recorded=1",
		"Duplicate target rows at saved positions [0 1]",
		"Row 0 owner=1 country=0 contact=no",
		"Saved relation_current=00.125 trust=-0.0000",
		`relation_last_month: "-0.00" (saved scalar)`,
		`truce: "2204.01.01" (saved scalar)`,
		`truce [duplicate saved key]: "2204.02.01"`,
		"modifier [additional saved modifier]",
		`research_agreement: "yes" (saved scalar)`,
		"diplo_action_dates: list with 2 saved entries",
		"Saved owner differs from the containing country.",
		"contact=has an unsupported saved shape",
		"truce: not recorded",
		"Row 0 owner=0 country=1 contact=yes",
		"trust=2.000",
		`truce: "2205.01.01" (saved scalar)`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in synthetic detail: %s", want, text)
		}
	}
	if strings.Contains(text, "alliance:") || strings.Contains(text, "wars:") {
		t.Fatal("records from separately scoped domains leaked into the detail")
	}
	missing, err := handleNeighbors(context.Background(), rawRequest(map[string]any{"neighbor_id": float64(99)}))
	if err != nil || missing.IsError || !strings.Contains(resultText(missing), "Reverse direction unavailable. Country record 99 is unavailable.") {
		t.Fatal("outgoing record to unavailable country was hidden")
	}
	ambiguous, err := handleNeighbors(context.Background(), rawRequest(map[string]any{"neighbor_id": float64(2)}))
	if err != nil || ambiguous.IsError || !strings.Contains(resultText(ambiguous), "relations_manager has duplicate saved fields") {
		t.Fatal("duplicate reverse manager was treated as authoritative")
	}
	bad, err := handleNeighbors(context.Background(), rawRequest(map[string]any{"neighbor_id": 0.5}))
	if err != nil || !bad.IsError {
		t.Fatal("fractional neighbor ID accepted")
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
