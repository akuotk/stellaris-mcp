package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestColonyAmbiguousCarrierNeverProjectsToPlanet(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={owned_planets={0 1 2 3}}} planets={planet={0={colony=0} 1={colony=1} 2={colony=2}}} colony={0={carrier={type=planet reference=0} carrier={type=ship reference=0} num_sapient_pops=40} 1={carrier={type=planet type=ship reference=1} num_sapient_pops=41} 2={carrier={type=planet reference=2 reference=0} num_sapient_pops=42} 3={carrier=bad num_sapient_pops=43}}`))
	if err != nil {
		t.Fatal(err)
	}
	country := gs.Country[0]
	got := resultText(renderColonies(gs, 0, &country, nil))
	if strings.Count(got, "carrier unsupported or duplicate") != 4 {
		t.Fatal(got)
	}
	for _, id := range []int{0, 1, 2} {
		if gs.Planets.Planet[id].NumPopsRaw != nil {
			t.Fatalf("ambiguous carrier projected onto planet %d", id)
		}
	}
}
