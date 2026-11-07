package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"strings"
	"testing"
	"unicode/utf8"
)

func resultText(r *mcp.CallToolResult) string {
	var text string
	for _, c := range r.Content {
		if v, ok := c.(mcp.TextContent); ok {
			text += v.Text
		}
	}
	return text
}

func TestPlanetAvailableRequiresSurveyAndNoOwner(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`planets={planet={
  1={planet_class="pc_arid" name={key="Unsurveyed"}}
  2={planet_class="pc_arid" surveyed_by=0 name={key="Available"}}
  3={planet_class="pc_arid" surveyed_by=0 owner=0 name={key="OwnedZero"}}
  4={planet_class="pc_arid" surveyed_by=0 colony=0 name={key="ColonyZero"}}
 }}`))
	if err != nil {
		t.Fatal(err)
	}
	r, err := planetAvailable(gs, 0, &gamestate.Country{})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "Available") || strings.Contains(text, "Unsurveyed") || strings.Contains(text, "OwnedZero") || strings.Contains(text, "ColonyZero") {
		t.Fatal(text)
	}
}

func TestJobBreakdownPreservesFractionalWorkforce(t *testing.T) {
	gs, err := gamestate.Load([]byte(`planets={planet={3={}}} pop_jobs={1={type=miner planet=3 workforce=1.25} 2={type=miner planet=3 workforce=0.5} 3={type=unused planet=3 workforce=-1} 4={type=other planet=4 workforce=100}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := jobBreakdown(gs, 3)
	if len(got) != 1 || got[0].workforce != 1.75 {
		t.Fatalf("workforce truncated or wrong jobs included: %+v", got)
	}
}

func TestPlanetSummaryUsesPlanetOwnerNotColonyIDs(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`planets={planet={
  3={colony=0 owner=0 name={key="OwnedHome"}}
  0={colony=2 owner=1 name={key="ForeignColony"}}
 }} colony={0={carrier={type=planet reference=3} num_sapient_pops=4200}}
 `))
	if err != nil {
		t.Fatal(err)
	}
	r, err := planetSummary(gs, 0, &gamestate.Country{OwnedPlanets: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(r)
	if !strings.Contains(text, "OwnedHome") || !strings.Contains(text, "4200") || strings.Contains(text, "ForeignColony") {
		t.Fatal(text)
	}
}

func TestJobBreakdownResolvesColonyID(t *testing.T) {
	gs, err := gamestate.Load([]byte(`planets={planet={3={colony=0}}} colony={0={carrier={type=planet reference=3} pop_jobs={1 1 2 4}}} pop_jobs={1={type=miner planet=0 workforce=3.5} 2={type=other planet=3 workforce=99} 3={type=unlisted planet=0 workforce=12} 4={type=missing_planet workforce=5}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := jobBreakdown(gs, 3)
	if len(got) != 1 || got[0].jobType != "miner" || got[0].workforce != 3.5 {
		t.Fatalf("unverified or duplicate job included: %+v", got)
	}
	gs.Colonies = map[int]gamestate.Colony{}
	if got := jobBreakdown(gs, 3); len(got) != 0 {
		t.Fatalf("unverified colony matched: %+v", got)
	}
}

func TestPlanetMissingAndZeroStats(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`planets={planet={0={colony=0 owner=0} 1={colony=1 owner=0 num_sapient_pops=9} 2={owner=0}}} colony={0={carrier={type=planet reference=0} num_sapient_pops=0 stability=0} 1={carrier={type=planet reference=99} num_sapient_pops=100}}`))
	if err != nil {
		t.Fatal(err)
	}
	detail, _ := planetDetail(gs, 0)
	text := resultText(detail)
	for _, want := range []string{"Population: 0", "Stability: 0", "Crime: not recorded"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s in %s", want, text)
		}
	}
	mismatch, _ := planetDetail(gs, 1)
	text = resultText(mismatch)
	for _, want := range []string{"reciprocal planet link unavailable", "Population: 9", "Stability: not recorded"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s in %s", want, text)
		}
	}
	empty, _ := planetDetail(gs, 2)
	if !strings.Contains(resultText(empty), "Population: not recorded") {
		t.Fatal(resultText(empty))
	}
}

func TestPlanetJobWarningsAndMissingDistricts(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`planets={planet={3={colony=0}}} colony={0={carrier={type=planet reference=3} pop_jobs={1 1 2 3 4 99}}} pop_jobs={1={type=miner planet=0 workforce=3} 2={type=missing_planet workforce=4} 3={type=wrong planet=9 workforce=5} 4={type=missing_workforce planet=0}}`))
	if err != nil {
		t.Fatal(err)
	}
	detail, _ := planetDetail(gs, 3)
	text := resultText(detail)
	for _, want := range []string{"Job 1: duplicate reference", "Job 2: colony back-reference not recorded", "Job 3: colony back-reference mismatch", "Job 4: workforce not recorded", "Job 99: referenced record unavailable", "District references: not recorded"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
}

func TestPlanetMixedLegacyAndColonyValues(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`planets={planet={3={colony=0 num_sapient_pops=77 stability=50 crime=8 final_designation=legacy_designation ascension_tier=2 districts={1} profits={energy=2}}}} colony={0={carrier={type=planet reference=3} stability=0}}`))
	if err != nil {
		t.Fatal(err)
	}
	p := gs.Planets.Planet[3]
	if p.NumPopsRaw == nil || *p.NumPopsRaw != 77 || p.StabilityRaw == nil || *p.StabilityRaw != 0 || p.CrimeRaw == nil || *p.CrimeRaw != 8 || len(p.Districts) != 1 || p.Profits["energy"] != 2 || p.FinalDesignationRaw == nil || *p.FinalDesignationRaw != "legacy_designation" || p.AscensionTierRaw == nil || *p.AscensionTierRaw != 2 {
		t.Fatalf("mixed saved values lost: %+v", p)
	}
	detail, _ := planetDetail(gs, 3)
	text := resultText(detail)
	for _, want := range []string{"Population: 77", "Stability: 0", "Crime: 8"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s in %s", want, text)
		}
	}
}

func TestTruncatePreservesUnicodeCharacters(t *testing.T) {
	for _, test := range []struct {
		text string
		max  int
		want string
	}{
		{"銀河帝国", 10, "銀河帝国"}, {"銀河帝国連合", 4, "銀河帝…"}, {"A🚀星団", 3, "A🚀…"}, {"abcdef", 4, "abc…"}, {"abc", 1, "…"}, {"abc", 0, ""}, {"abc", -1, ""},
	} {
		if got := truncate(test.text, test.max); got != test.want {
			t.Errorf("truncate(%q,%d)=%q, want %q", test.text, test.max, got, test.want)
		}
	}
}
func TestPlanetSummaryRetainsShortJapaneseName(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={}} planets={planet={0={owner=0 colony=0 name={key="日本語の惑星名"}}}} colony={0={carrier={type=planet reference=0}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	result, err := planetSummary(gs, 0, &c)
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(result)
	if !strings.Contains(text, "日本語の惑星名") || !utf8.ValidString(text) {
		t.Fatal("planet name truncated or invalid UTF-8")
	}
}
