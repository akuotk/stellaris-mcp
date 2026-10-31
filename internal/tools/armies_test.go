package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func armyFixture(t *testing.T) *gamestate.GameState {
	t.Helper()
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Synthetic nation"} owned_armies={0 0 1 99}}}
 army={
  0={name={key="Synthetic army"} type="synthetic" owner=0 species=0 colony=0 spawning_colony=0 ship=0 leader=0 health=0 max_health=10.5 morale=0 experience=0}
  1={name={key="Inconsistent owner"} owner=1}
  2={name={key="Owner-only army"} owner=0}
  3={name={key="Foreign army"} owner=1}
 }
 colony={0={carrier={type=planet reference=7} army={0} spawned_armies={0}}}
 planets={planet={7={colony=0 name={key="Example planet"}}}}
 ships={0={name={key="Example ship"} army=0 fleet=0}}
 fleet={0={name={key="Example fleet"} ships={0}}}
 species_db={0={name={key="Example species"}}}
 leaders={0={name={full_names={key="Example leader"}}}}
 `))
	if err != nil {
		t.Fatal(err)
	}
	return gs
}

func TestArmiesOwnerScopeAndReciprocalDetails(t *testing.T) {
	gs := armyFixture(t)
	country := gs.Country[0]
	summary := resultText(renderArmies(gs, 0, &country, nil))
	for _, want := range []string{"Army 0: Synthetic army", "2 reference(s)", "Ownership mismatch", "army.owner matches country, but country.owned_armies lacks this ID", "Army 99: referenced record unavailable"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in %s", want, summary)
		}
	}
	if strings.Contains(summary, "Foreign army") {
		t.Fatal("unrelated foreign army included")
	}
	id := 0
	text := resultText(renderArmies(gs, 0, &country, &id))
	for _, want := range []string{"health=0 max_health=10.5 morale=0 experience=0", "Example species", "Example leader", "Example planet", "colony.army membership: validated", "colony.spawned_armies membership: validated", "Ship reference 0: Example ship", "army/ship back-reference validated", "Fleet 0: Example fleet; fleet.ships membership: validated", "Both colony and ship location references are saved"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	id = 3
	if !renderArmies(gs, 0, &country, &id).IsError {
		t.Fatal("foreign army accepted")
	}
}

func TestArmyMissingMismatchLegacyAndUnsupportedDetails(t *testing.T) {
	gs := armyFixture(t)
	country := gs.Country[0]
	id := 0
	ship := gs.Ships[0]
	wrong := 8
	ship.ArmyID = &wrong
	ship.FleetID = nil
	gs.Ships[0] = ship
	colony := gs.Colonies[0]
	colony.Armies = []int{}
	colony.SpawnedArmies = nil
	gs.Colonies[0] = colony
	text := resultText(renderArmies(gs, 0, &country, &id))
	for _, want := range []string{"ship.army back-reference mismatch: 8", "ship.fleet: not recorded", "missing reverse reference", "list not recorded"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "Transport ship") || strings.Contains(text, "army/ship back-reference validated") {
		t.Fatal("unvalidated ship relationship asserted")
	}
	ship.ArmyID = nil
	gs.Ships[0] = ship
	text = resultText(renderArmies(gs, 0, &country, &id))
	if !strings.Contains(text, "ship.army: not recorded") {
		t.Fatal("missing ship reverse reference concealed")
	}
	a := gs.Armies.Items[0]
	a.Colony = nil
	a.SpawningColony = nil
	a.Ship = nil
	legacy := 7
	a.Planet = &legacy
	gs.Armies.Items[0] = a
	text = resultText(renderArmies(gs, 0, &country, &id))
	if !strings.Contains(text, "Planet reference (as saved) 7: possible planet ID match Example planet") {
		t.Fatal("saved planet reference not accessible")
	}
	a.Planet = nil
	gs.Armies.Items[0] = a
	gs.Armies.DecodeErrors[0] = "synthetic unsupported shape"
	text = resultText(renderArmies(gs, 0, &country, &id))
	if !strings.Contains(text, "No location reference recorded") || !strings.Contains(text, "Record decoding incomplete") {
		t.Fatal("missing location or unsupported record concealed")
	}
	if armyMembership([]int{0, 0}, 0) != "2 duplicate references" {
		t.Fatal("duplicate reciprocal references concealed")
	}
}
