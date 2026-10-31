package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
)

func TestTraditionListsPreserveRawKeysAndMissing(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={traditions={tr_a tr_b tr_a} ascension_perks={} tradition_categories={tree_a tree_b} last_picked_tradition=tr_b}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	if !c.Traditions.Recorded || len(c.Traditions.Values) != 3 || c.Traditions.Values[2] != "tr_a" || !c.AscensionPerks.Recorded || len(c.AscensionPerks.Values) != 0 || !c.LastPickedTradition.Recorded {
		t.Fatal("saved data lost")
	}
	var b strings.Builder
	writeSavedTraditionList(&b, "Traditions", c.Traditions)
	writeSavedTraditionList(&b, "Perks", c.AscensionPerks)
	for _, want := range []string{"[tr_a]", "[tr_b]", "empty"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("missing %s", want)
		}
	}
	gs, err = gamestate.Load([]byte(`country={0={traditions={a=1 mixed} ascension_perks={{unsupported=yes}} last_picked_tradition={bad=yes} num_sapient_pops=12}}`))
	if err != nil {
		t.Fatal(err)
	}
	c = gs.Country[0]
	if !c.Traditions.Unsupported || !c.AscensionPerks.Unsupported || !c.LastPickedTradition.Unsupported || c.TraditionCategories.Recorded || c.NumPops != 12 {
		t.Fatal("unsupported data not isolated")
	}
	b.Reset()
	writeSavedTraditionList(&b, "Traditions", c.Traditions)
	writeSavedTraditionList(&b, "Categories", c.TraditionCategories)
	if !strings.Contains(b.String(), "unsupported shape") || !strings.Contains(b.String(), "not recorded") {
		t.Fatal(b.String())
	}
}
