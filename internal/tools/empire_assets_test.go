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
	agreements={agreements={1={owner=0 target=2 active_status=active date_changed="2201.02.03" term_data={can_subject_vote=no joins_overlord_wars=defensive discrete_terms={{key=term_a value=option_a} {key=term_a value=option_b}} resource_terms={{key=resource_example value=-0.25}}} subject_specialization={level=2 experience=3.5 subject_conversion_process={progress=5 in_progress=yes done=no}}} 2={owner=2 target=3} 3={target=0}}}
	situations={situations={4={country=0 type="situation_test" progress=12} 5={country=2 type="other"}}}
	megastructures={6={owner=0 type="gateway" coordinate={origin=7}} 8={type="unowned"}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderEmpireAssets(gs, 0, &c))
	for _, want := range []string{"owner Home (0), target Other (2)", "progress=12.00", "Test System (system ID 7)", "date_changed=2201.02.03", "can_subject_vote=no", "joins_overlord_wars=defensive", "discrete_term term_a=option_a", "discrete_term term_a=option_b", "resource_term resource_example=-0.25", "specialization_level=2", "conversion_in_progress=yes"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "can_subject_be_integrated=") {
		t.Fatal("absent term presented as a default")
	}
	for _, bad := range []string{"ID 2:", "ID 3:", "ID 5:", "ID 8:"} {
		if strings.Contains(text, bad) {
			t.Fatalf("unexpected %q: %s", bad, text)
		}
	}
}
