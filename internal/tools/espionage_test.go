package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestEspionagePreservesZeroAndUnknownValues(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Home"}} 2={adjective={key="Other"}}} spy_networks={1={owner=0 target=2 power=0 leader=0} 2={target=0} 3={target=2} 4={owner=2 target=0 power=10}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderEspionage(gs, 0, &c))
	for _, want := range []string{"Network ID 1: owner Home (ID 0); target Other (ID 2)", "Leader ID: 0", "Network ID 2: owner not recorded; target Home (ID 0)", "Network power: not recorded", "Network ID 4"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "Network ID 3") {
		t.Fatalf("unowned network included: %s", text)
	}
}

func TestEspionageCountryAndLeaderReferencesAndTimedModifiers(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	data := []byte(`player={{country=0}}
	country={0={adjective={key=Home} espionage_manager={spy_networks={4 0 0 2 3 9}}}
	1={adjective={key=Other} espionage_manager={spy_networks={1 2}}}}
	leaders={0={location={type=spy_network id=0}} 1={location={type=spy_network id=1 id=2}}}
	spy_networks={
	0={owner=0 target=1 leader=0 power=00.1000 timed_modifiers={items={{modifier=first days=-0.00} {days=0 days=1} unknown}}}
	1={owner=1 target=0 leader=1}
	2={owner=1 target=1 timed_modifiers={items=bad}}
	3={owner=0} 3=none 4=unsupported 9=none}`)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", data}})
	result, err := handleEspionage(context.Background(), mcp.CallToolRequest{})
	if err != nil || result.IsError {
		t.Fatalf("get_espionage returned an error: %v", err)
	}
	text := resultText(result)
	for _, want := range []string{
		"country.espionage_manager.spy_networks (saved order): [4 0 0 2 3 9]",
		"Selected country.espionage_manager.spy_networks membership: 2 duplicate references",
		"Saved owner country.espionage_manager.spy_networks membership: validated",
		"Selected country's saved reference disagrees with network.owner.",
		"Reciprocal saved leader.location type and network ID agree.",
		"Duplicate leader.location fields; reverse reference unresolved.",
		"Network power (raw save value; infiltration conversion unknown): 00.1000",
		"Timed modifiers (saved order): 3",
		`Item 0: modifier="first" days="-0.00" (raw save values)`,
		"Item 1: duplicate modifier or days field",
		"Item 2: unsupported saved shape",
		"Unsupported timed_modifiers shape",
		"Network ID 3: duplicate saved record ID or fields",
		"Network ID 4: referenced record has an unsupported saved shape",
		"Network ID 9: referenced record unavailable",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in synthetic output: %s", want, text)
		}
	}
}
