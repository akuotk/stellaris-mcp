package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

// Installed 4.5 ship_sizes/29_nomads_dlc_ships.txt declares carries_colony=pc_ark.
// The synthetic carrier below deliberately does not guess Arkship save serialization.
func TestColoniesIncludeNonPlanetCarrierWithoutInventedShipMapping(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Synthetic nation"} owned_planets={0 1 2 99}} 1={owned_planets={3}}}
 planets={planet={7={name={key="Verified planet"} colony=2 owner=0}}}
 colony={
  0={carrier={type="synthetic_mobile" reference=0} num_sapient_pops=0 stability=0 total_housing=0 free_amenities=-0.25
   districts={0} buildings_cache={} produces={energy=0.0001234} profits={energy=-0.125} pop_groups={0 1 99} pop_jobs={0 1 2 3 99}}
  1={carrier={type="synthetic_unknown"}}
  2={carrier={type=planet reference=7} num_sapient_pops=10}
  3={carrier={type="synthetic_foreign" reference=3}}
 }
 pop_groups={0={key={species=0 category="worker"} size=0} 1={key={species=0}}}
 species_db={0={name={key="Example species"}}}
 pop_jobs={0={type="synthetic_job" workforce=1.25} 1={type="unknown_workforce"} 2={type="zero_workforce" workforce=0} 3={type="negative_workforce" workforce=-1}}
 districts={0={type="synthetic_district" zones={}}}
 `))
	if err != nil {
		t.Fatal(err)
	}
	country := gs.Country[0]
	summary := resultText(renderColonies(gs, 0, &country, nil))
	for _, want := range []string{"Colony 0: carrier type=synthetic_mobile reference=0", "population=0", "Colony 1: carrier type=synthetic_unknown reference=not recorded", "Verified planet, reciprocal planet link validated", "Colony 99: referenced record unavailable", "Arkship serialization is not real-save validated"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in %s", want, summary)
		}
	}
	if strings.Contains(summary, "synthetic_foreign") {
		t.Fatal("unreferenced foreign colony included")
	}
	id := 0
	detail := resultText(renderColonies(gs, 0, &country, &id))
	for _, want := range []string{"stability: 0", "crime: not recorded", "free_amenities: -0.25", "total_housing: 0", "energy=0.0001234", "Building cache references: []", "population=not recorded", "Job 0: type=synthetic_job workforce=1.25", "Job 1: type=unknown_workforce workforce=not recorded", "Job 2: type=zero_workforce workforce=0", "Job 3: type=negative_workforce workforce=-1", "Group 99: unavailable", "Job 99: unavailable"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("missing %q in %s", want, detail)
		}
	}
	id = 3
	if !renderColonies(gs, 0, &country, &id).IsError {
		t.Fatal("foreign colony accepted as country-owned")
	}
}

func TestColoniesDoNotInventCarrierReferenceZero(t *testing.T) {
	gs, err := gamestate.Load([]byte(`colony={0={carrier={type=planet}}} planets={planet={0={colony=0}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(colonyCarrier(gs, 0, gs.Colonies[0]), "reference=not recorded (carrier identity unvalidated)") {
		t.Fatal("missing carrier reference became planet zero")
	}
}

func TestColoniesNoTableReportsLegacyScope(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	text := resultText(renderColonies(&gamestate.GameState{}, 0, &gamestate.Country{}, nil))
	if !strings.Contains(text, "No colony-table records; legacy inline planet data remains available") {
		t.Fatal(text)
	}
}
