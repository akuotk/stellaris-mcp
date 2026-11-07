package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestSpeciesLinksReferencesWithoutGuessingMissingValues(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Home"}}} species_db={7={name={key="Example"} class="ABC" portrait="p" traits={trait="trait_fast"}}} planets={planet={1={owner=0 colony=1} 2={owner=0 colony=2}}} colony={1={carrier={type=planet reference=1} pop_groups={4 5 6 99}} 2={carrier={type=planet reference=2} pop_groups={4}}} pop_groups={4={key={species=7} size=123} 5={key={species=7}} 6={size=1}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderSpecies(gs, 0, &c, nil))
	for _, v := range []string{"Species ID 7: Example", "Trait: Fast [trait_fast]", "Owned planets referencing groups: 2; groups: 2; population sum (raw save units): 123 (incomplete: 1 group sizes missing) (location ambiguous for 1 groups)", "missing groups=1, groups without species ID=1, duplicate group references=1, groups referenced by multiple owned planets=1"} {
		if !strings.Contains(text, v) {
			t.Fatalf("missing %q: %s", v, text)
		}
	}
	for i := 0; i < 50; i++ {
		if next := resultText(renderSpecies(gs, 0, &c, nil)); next != text {
			t.Fatal("species results vary with map iteration order")
		}
	}
	if strings.Contains(text, "Species ID 0:") {
		t.Fatal("missing species treated as zero")
	}
}

func TestSpeciesZeroSizeIsRecorded(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={}} species_db={0={name={key="Example"}}} planets={planet={1={owner=0 pop_groups={0}}}} pop_groups={0={key={species=0} size=0}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderSpecies(gs, 0, &c, nil))
	for _, want := range []string{"Species ID 0:", "groups: 1; population sum (raw save units): 0"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "group sizes missing") {
		t.Fatal("zero size treated as missing")
	}
}

func TestSpeciesDuplicateOnSamePlanetIsNotAmbiguous(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={}} species_db={0={name={key="Example"}}} planets={planet={1={owner=0 pop_groups={0 0}}}} pop_groups={0={key={species=0} size=12}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderSpecies(gs, 0, &c, nil))
	if !strings.Contains(text, "groups: 1; population sum (raw save units): 12") || !strings.Contains(text, "duplicate group references=1, groups referenced by multiple owned planets=0") || strings.Contains(text, "location ambiguous") {
		t.Fatal(text)
	}
}
