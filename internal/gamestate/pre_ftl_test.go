package gamestate

import (
	"os"
	"testing"
)

// The opt-in audit reports aggregate counts without saved IDs or field values.
func TestPreFTLLocalCatalogCoverage(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only pre-FTL coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	counts := map[string]int{}
	for index, file := range files {
		data, err := extractGamestate(file.Path)
		if err != nil {
			t.Fatalf("save index %d archive read failed", index)
		}
		state, err := loadPreFTL(data)
		if err != nil {
			t.Fatalf("save index %d pre-FTL decoding failed", index)
		}
		if state.Countries.TableIssue || state.Planets.TableIssue || state.Fleets.TableIssue || state.Ships.TableIssue {
			counts["incomplete_tables"]++
		}
		counts["unsupported_records"] += len(state.Countries.DecodeErrors) + len(state.Planets.DecodeErrors) + len(state.Fleets.DecodeErrors) + len(state.Ships.DecodeErrors)
		fleetCountries := map[int][]int{}
		capitals := map[int]bool{}
		for id, c := range state.Countries.Items {
			if c.Capital.Value != nil {
				capitals[*c.Capital.Value] = true
			}
			if !c.Fleets.OwnershipAmbiguous {
				for _, owned := range c.Fleets.OwnedFleets {
					if owned.FleetID != nil && state.Fleets.Items[*owned.FleetID].Recorded {
						fleetCountries[*owned.FleetID] = append(fleetCountries[*owned.FleetID], id)
					}
				}
			}
			if !c.Relevant() {
				continue
			}
			counts["country_markers"]++
			if c.Type.Value == "primitive" && !c.Type.Unsupported {
				counts["saved_primitive_type"]++
			} else if c.Age.Recorded {
				counts["age_with_other_country_type"]++
			}
			if c.Age.Recorded && !c.Age.Unsupported {
				counts["age_records"]++
			}
			if c.Awareness.Recorded && !c.Awareness.Unsupported {
				counts["awareness_records"]++
			}
			if c.OldAwareness.Recorded && !c.OldAwareness.Unsupported {
				counts["old_awareness_records"]++
			}
			if c.Capital.Value != nil {
				if p, ok := state.Planets.Items[*c.Capital.Value]; ok {
					counts["capital_candidates"]++
					if p.Owner.Value != nil && *p.Owner.Value == id {
						counts["capital_owner_id_agrees"]++
					}
					if p.Controller.Value != nil && *p.Controller.Value == id {
						counts["capital_controller_id_agrees"]++
					}
				}
			}
		}
		for id, fleet := range state.Fleets.Items {
			if !fleet.Recorded {
				continue
			}
			counts["observation_fleets"]++
			if len(fleetCountries[id]) == 1 {
				counts["single_owned_fleets_country_ref"]++
			}
			if fleet.OrbitPlanet.Value != nil {
				planetID := *fleet.OrbitPlanet.Value
				if capitals[planetID] {
					counts["observation_planet_id_also_capital"]++
				}
				if p, ok := state.Planets.Items[planetID]; ok {
					counts["observation_orbit_planet_candidates"]++
					if p.OrbitalStation.Value != nil && *p.OrbitalStation.Value == id {
						counts["reciprocal_observation_orbit_ids"]++
					}
					if p.Owner.Value != nil {
						if c, ok := state.Countries.Items[*p.Owner.Value]; ok && c.Type.Value == "primitive" && !c.Type.Unsupported {
							counts["orbit_owner_country_type_primitive"]++
						}
						if len(fleetCountries[id]) == 1 && fleetCountries[id][0] != *p.Owner.Value {
							counts["orbit_owner_differs_from_fleet_country_ref"]++
						}
					}
				}
			}
			for _, shipID := range fleet.Ships.Values {
				counts["observation_ship_refs"]++
				if ship, ok := state.Ships.Items[shipID]; ok && ship.Fleet.Value != nil && *ship.Fleet.Value == id {
					counts["reciprocal_observation_ship_ids"]++
				}
			}
		}
	}
	t.Logf("local pre-FTL and observation coverage: saves=%d counts=%v", len(files), counts)
}
