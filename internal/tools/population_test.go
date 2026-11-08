package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
)

func TestPopulationIdentityAndDistributions(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`
 species_db={1234567890={name={key="Test species"} class="ART" traits={trait="trait_organic" trait="trait_strong"}}}
 pop_factions={0={country=0 type="totalitarian" name={key="Test faction"} support_percent=0.4 faction_approval=0.9 last_month_pops=11 pops_change=2} 3={country=1 name={key="Foreign faction"}}}
 pop_groups={9={key={species=1234567890 category="worker"} size=20 ethos={ethics={"ethic_authoritarian" "ethic_xenophobe"} pops={13 7}} factions={objects={0} pops={11}}}}
 `))
	if err != nil {
		t.Fatal(err)
	}
	if len(gs.Species.Items[1234567890].Traits.Trait) != 2 {
		t.Fatal("traits not decoded")
	}
	if got := populationSpecies(gs, 1234567890); !strings.Contains(got, "Test species") {
		t.Fatal(got)
	}
	var b strings.Builder
	writePopDistributions(&b, gs, gs.PopGroups[9])
	text := b.String()
	for _, want := range []string{"ethic_authoritarian=13", "ethic_xenophobe=7", "Test faction", "country=0): 11"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %s", want, text)
		}
	}
	b.Reset()
	writeCountryFactions(&b, gs, 0)
	text = b.String()
	if !strings.Contains(text, "support_percent=0.4 approval=0.9 last_month_population=11 change=+2") || strings.Contains(text, "Foreign faction") {
		t.Fatal(text)
	}
}

func TestPopulationMismatchedArraysAndMissingIdentities(t *testing.T) {
	gs := &gamestate.GameState{}
	var b strings.Builder
	group := gamestate.PopGroup{Ethos: gamestate.PopEthos{Ethics: []string{"ethic_one"}, Pops: []int{4, 5}}, Factions: gamestate.PopFactionDistribution{Objects: []int{0, 2}, Pops: []int{4}}}
	writePopDistributions(&b, gs, group)
	if strings.Count(b.String(), "unsupported mismatched arrays") != 2 || !strings.Contains(b.String(), "populations=[4 5]") {
		t.Fatal(b.String())
	}
	b.Reset()
	group.Ethos = gamestate.PopEthos{}
	group.Factions = gamestate.PopFactionDistribution{Objects: []int{0}, Pops: []int{4}}
	writePopDistributions(&b, gs, group)
	if !strings.Contains(b.String(), "0 (identity unavailable): 4") || populationSpecies(gs, 0) != "0 (identity unavailable)" {
		t.Fatal(b.String())
	}
}
