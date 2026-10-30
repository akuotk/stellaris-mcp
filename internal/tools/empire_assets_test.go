package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
)

func TestEmpireAssetsFiltersAndResolvesSystem(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Home"}} 2={adjective={key="Other"}}}
	galactic_object={7={name={key="Test System"} planet=9}}
	agreements={agreements={1={owner=0 target=2 active_status=active} 2={owner=2 target=3} 3={target=0}}}
	situations={situations={4={country=0 type="situation_test" progress=12} 5={country=2 type="other"}}}
	megastructures={6={owner=0 type="gateway" coordinate={origin=7}} 8={type="unowned"}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderEmpireAssets(gs, 0, &c))
	for _, want := range []string{"owner Home (0), target Other (2)", "progress=12.00", "Test System (system ID 7)"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	for _, bad := range []string{"ID 2:", "ID 3:", "ID 5:", "ID 8:"} {
		if strings.Contains(text, bad) {
			t.Fatalf("unexpected %q: %s", bad, text)
		}
	}
}
