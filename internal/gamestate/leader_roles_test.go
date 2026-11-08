package gamestate

import (
	"os"
	"slices"
	"testing"
)

// The opt-in audit reports aggregate counts without saved IDs or values.
func TestLeadershipRolesLocalCatalogCoverage(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only leadership coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	counts := map[string]int{}
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("save index %d whole-save decoding failed", index)
		}
		counts["leader_records"] += len(gs.Leaders)
		counts["leader_decode_errors"] += len(gs.LeaderRecords.DecodeErrors)
		if gs.LeaderRecords.TableIssue {
			counts["incomplete_leader_tables"]++
		}
		for countryID, country := range gs.Country {
			for _, role := range []struct {
				key string
				ref OptionalIntRef
			}{{"ruler", country.RulerRef}, {"heir", country.CouncilGovernment.Heir}} {
				if role.ref.Unsupported {
					counts["unsupported_"+role.key+"_refs"]++
				}
				if role.ref.Value == nil {
					continue
				}
				id := *role.ref.Value
				counts[role.key+"_refs"]++
				if leader, found := gs.Leaders[id]; found {
					counts[role.key+"_leader_candidates"]++
					if leader.CountryID != nil && *leader.CountryID == countryID {
						counts[role.key+"_country_id_agrees"]++
					}
				}
				if slices.Contains(country.OwnedLeaders, id) {
					counts[role.key+"_in_owned_leaders"]++
				}
			}
		}
		for id, leader := range gs.Leaders {
			if leader.SavedExperience.Recorded {
				counts["experience_fields"]++
			}
			if ref := leader.CountryRef.Value; ref != nil {
				counts["leader_country_refs"]++
				if _, found := gs.Country[*ref]; found {
					counts["leader_country_candidates"]++
				}
			}
			if ref := leader.SpeciesRef.Value; ref != nil {
				counts["leader_species_refs"]++
				if _, found := gs.Species.Items[*ref]; found {
					counts["leader_species_candidates"]++
				}
			}
			if ref := leader.PlanetRef.Value; ref != nil {
				counts["leader_planet_refs"]++
				if _, found := gs.Planets.Planet[*ref]; found {
					counts["leader_planet_candidates"]++
				}
			}
			if leader.Location.Type == "army" && leader.Location.IDRaw != nil {
				counts["army_location_refs"]++
				if army, found := gs.Armies.Items[*leader.Location.IDRaw]; found && army.Leader != nil && *army.Leader == id {
					counts["reciprocal_army_leader_ids"]++
				}
			}
			if leader.CouncilLocation.Type == "council_position" && leader.CouncilLocation.IDRaw != nil {
				counts["council_location_refs"]++
				if position, found := gs.CouncilPositions.Items[*leader.CouncilLocation.IDRaw]; found && position.Leader != nil && *position.Leader == id {
					counts["reciprocal_council_leader_ids"]++
					if position.Country != nil && leader.CountryID != nil && *position.Country == *leader.CountryID {
						counts["reciprocal_council_country_ids"]++
					}
				}
			}
		}
	}
	t.Logf("local leadership coverage: saves=%d counts=%v", len(files), counts)
}
