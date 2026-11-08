package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestSpeciesAndColonySavedReferenceOutput(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	data := []byte(`player={{country=0}}
	country={0={adjective={key=Home} owned_planets={0} founder_species_ref=0 built_species_ref=1 owned_species_refs={0 1 1 2 3 4} enslaved_species_refs=unknown}}
	species_db={0={name={key=Primary} base_ref=0 home_planet={type=planet reference=9} uplifter=9 extra_trait_points=00}
	1={name={key=Variant} base_ref=0 home_planet=9} 2={name={key=Other} home_planet={type=ship reference=9}}
	3={name={key=Hidden}} 3=none}
	planets={planet={9={name={key="Carrier World"} owner=0 colony=0}}}
	colony={0={carrier={type=planet reference=9} species_refs={0 1 1} enslaved_species_refs={1} colonizing_species=1
	species_information={0={num_pops=001.00} 1={num_pops=02 num_enslaved=-0} 2={num_pops=3} 3={num_pops=4} 3=none}
	pop_groups={0 0 1 2} pop_jobs={0 0 1 2}}}
	pop_groups={0={key={species=0} planet=0 month_start_size=00 last_month_growth=-0} 1={planet=9} 2={planet=0 planet=9}}
	pop_jobs={0={planet=0} 1={planet=9} 2={planet={unknown=yes}}}`)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", data}})
	species, err := handleSpecies(context.Background(), mcp.CallToolRequest{})
	if err != nil || species.IsError {
		t.Fatalf("get_species failed: %v", err)
	}
	s := resultText(species)
	for _, want := range []string{
		"country.founder_species_ref: 0 (Primary)",
		"country.built_species_ref: 1 (Variant)",
		"Duplicate species reference 1 in country.owned_species_refs",
		"country.enslaved_species_refs: unsupported saved reference shape",
		"base_ref points to this same saved record",
		"Saved typed home planet 9: Carrier World",
		"Legacy scalar home planet ID candidate 9: Carrier World",
		`uplifter: "9" (saved scalar)`,
		`extra_trait_points: "00" (saved scalar)`,
		"Species ID 3: duplicate saved record ID or fields",
		"Species ID 4: record unavailable",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in species output: %s", want, s)
		}
	}
	if strings.Count(s, "home planet 9: Carrier World") != 1 {
		t.Fatal("typed non-planet home was treated as a planet")
	}
	gs, err := gamestate.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	country := gs.Country[0]
	id := 0
	text := resultText(renderColonies(gs, 0, &country, &id))
	for _, want := range []string{
		"colonizing_species: 1 (Variant)",
		"Duplicate species reference 1 in species_refs",
		`species_information[1]: species=1 (Variant) num_pops="02" num_enslaved="-0"`,
		"species_refs membership=2 duplicate references",
		"species_refs membership=missing reverse reference",
		"species_information[3]: duplicate saved record ID or fields",
		"pop_group.planet=0 matches colony ID",
		"pop_group.planet=9 differs from colony ID 0",
		"pop_group.planet: duplicate saved reference fields",
		"pop_job.planet=0 matches colony ID",
		"pop_job.planet=9 differs from colony ID 0",
		"pop_job.planet: unsupported saved reference shape",
		"Duplicate POP group reference 0",
		"Duplicate job reference 0",
		`month_start_size="00" last_month_growth="-0"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in colony output: %s", want, text)
		}
	}
}
