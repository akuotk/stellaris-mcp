package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
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
