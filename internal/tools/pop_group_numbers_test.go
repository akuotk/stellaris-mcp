package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestPopGroupNumberStatesAcrossTools(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	fixture := "date=2300.01.01 country={0={owned_planets={0}}} species_db={0={name={key=synthetic_species}}} " +
		"planets={planet={0={owner=0 colony=0}}} colony={0={carrier={type=planet reference=0} pop_groups={0 1 2 3 4 5 6 7}}} " +
		"pop_groups={" +
		"0={key={species=0}} " +
		"1={key={species=0} size=00 last_month_growth=-0 happiness=00.000 habitability=0.0} " +
		"2={key={species=0} size=4 size=5 last_month_growth=3 last_month_growth=3 happiness=0.5 happiness={} habitability=0.1 habitability=0.2} " +
		"3={key={species=0} size={broken=shape anonymous} last_month_growth=oops happiness=NaN habitability=+Inf} " +
		"4={key={species=0} size=9223372036854775807 last_month_growth=-9223372036854775808 happiness=-0.500 habitability=00.7500} " +
		"5={key={species=0} size=9223372036854775807 happiness=1e999 habitability={1 2}} " +
		"6={key={species=0} size=123} 6={key={species=0} size=124} " +
		"7={key={species=0} size=9223372036854775808 last_month_growth={} happiness={} habitability={bad=yes}}}"
	gs, err := gamestate.Load([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if gs.Date != "2300.01.01" {
		t.Fatal("unrelated fields failed to decode")
	}
	planet, err := planetDetail(gs, 0)
	if err != nil || planet.IsError {
		t.Fatalf("planet detail failed: %v", err)
	}
	check := func(text string, fragments ...string) {
		t.Helper()
		for _, want := range fragments {
			if !strings.Contains(text, want) {
				t.Errorf("missing %q from synthetic output:\n%s", want, text)
			}
		}
	}
	check(resultText(planet),
		"population=not recorded growth=not recorded happiness=not recorded habitability=not recorded",
		"population=00 growth=-0 happiness=00.000 habitability=0.0",
		"population=duplicate saved fields [\"4\", \"5\"] growth=duplicate saved fields [\"3\", \"3\"] happiness=duplicate saved fields [\"0.5\", empty block] habitability=duplicate saved fields [\"0.1\", \"0.2\"]",
		"population=unsupported saved value mixed block growth=unsupported saved value \"oops\" happiness=unsupported saved value \"NaN\" habitability=unsupported saved value \"+Inf\"",
		"population=9223372036854775807 growth=-9223372036854775808 happiness=-0.500 habitability=00.7500",
		"happiness=unsupported saved value \"1e999\" habitability=unsupported saved value list",
		"Group 6: duplicate saved group records or tables",
		"population=unsupported saved value \"9223372036854775808\" growth=unsupported saved value empty block happiness=unsupported saved value empty block habitability=unsupported saved value object")
	country := gs.Country[0]
	colonyID := 0
	colony := resultText(renderColonies(gs, 0, &country, &colonyID))
	check(colony, "population=00", "last_month_growth=\"-0\"", "last_month_growth=duplicate saved fields", "Group 6: duplicate saved group records or tables")
	species := resultText(renderSpecies(gs, 0, &country, nil))
	check(species, "population sum (raw save units): 18446744073709551614", "1 group sizes missing", "1 groups with duplicate sizes", "2 groups with unsupported sizes", "1 referenced groups have duplicate saved records or tables and were excluded")
}
