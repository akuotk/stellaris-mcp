package gamestate

import (
	"bytes"
	"os"
	"testing"
)

func TestColonySavedValuePresence(t *testing.T) {
	gs, err := Load([]byte(`colony={0={carrier={type="synthetic_mobile" reference=0} num_sapient_pops=0 stability=0 crime=0 amenities=0 amenities_usage=0 free_amenities=0 total_housing=0 housing_usage=0 free_housing=0 ascension_tier=0 pop_jobs={0}} 1={carrier={}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Colonies[0]
	if c.NumPopsRaw == nil || *c.NumPopsRaw != 0 || c.Carrier.ReferenceID == nil || *c.Carrier.ReferenceID != 0 || len(c.PopJobs) != 1 || c.AscensionTierRaw == nil {
		t.Fatal("explicit zero values or direct references lost")
	}
	for _, v := range []*float64{c.StabilityRaw, c.CrimeRaw, c.AmenitiesRaw, c.AmenitiesUsageRaw, c.FreeAmenitiesRaw, c.TotalHousingRaw, c.HousingUsageRaw, c.FreeHousingRaw} {
		if v == nil || *v != 0 {
			t.Fatal("explicit zero statistic lost")
		}
	}
	empty := gs.Colonies[1]
	if empty.NumPopsRaw != nil || empty.Carrier.ReferenceID != nil || empty.StabilityRaw != nil || empty.AscensionTierRaw != nil || empty.PopJobs != nil {
		t.Fatal("missing statistics became zero values")
	}
}

func TestNonPlanetColonyLocalCatalogCoverage(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local carrier coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil {
		t.Fatal("save catalog read failed")
	}
	withTables, planet, nonPlanet, missingReference := 0, 0, 0, 0
	for index, file := range files {
		// Full typed parsing is attempted only for saves with colony records; old snapshots
		// without this table are covered by existing catalog/raw-tree integration checks.
		data, err := extractGamestate(file.Path)
		if err != nil {
			t.Fatalf("save index %d read failed", index)
		}
		if !containsColonyTable(data) {
			continue
		}
		gs, err := Load(data)
		if err != nil {
			t.Fatalf("save index %d colony save typed parsing failed", index)
		}
		withTables++
		for _, c := range gs.Colonies {
			if c.Carrier.ReferenceID == nil {
				missingReference++
			}
			if c.Carrier.Type == "planet" {
				planet++
			} else {
				nonPlanet++
			}
		}
	}
	t.Logf("local carrier coverage: catalog_saves=%d colony_table_saves=%d planet_carriers=%d nonplanet_or_missing_type=%d missing_references=%d", len(files), withTables, planet, nonPlanet, missingReference)
}

func containsColonyTable(data []byte) bool {
	// Root save keys begin at column zero; this check avoids interpreting legacy owned_planets as colony IDs.
	return bytes.Contains(data, []byte("\ncolony=")) || bytes.HasPrefix(data, []byte("colony="))
}

func TestMissingCarrierReferenceCannotHydratePlanetZero(t *testing.T) {
	gs, err := Load([]byte(`planets={planet={0={colony=0 num_sapient_pops=5}}} colony={0={carrier={type=planet} num_sapient_pops=99}}`))
	if err != nil {
		t.Fatal(err)
	}
	if gs.Planets.Planet[0].NumPops != 5 {
		t.Fatal("absent carrier reference silently joined planet zero")
	}
}
