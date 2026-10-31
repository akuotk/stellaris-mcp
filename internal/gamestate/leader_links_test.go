package gamestate

import (
	"os"
	"testing"
)

func TestLocalLeaderLocationReferences(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("local catalog not set")
	}
	files, err := ListSaveFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	var total, shipMatch, fleetMatch, both, reciprocal, missingLocationID, missingFleetID int
	for _, f := range files {
		gs, e := LoadFromZip(f.Path)
		if e != nil {
			t.Fatal("local save failed to decode")
		}
		for _, l := range gs.Leaders {
			if l.Location.Type != "ship" {
				continue
			}
			total++
			if l.Location.IDRaw == nil {
				missingLocationID++
				continue
			}
			s, sm := gs.Ships[*l.Location.IDRaw]
			_, fm := gs.Fleet[*l.Location.IDRaw]
			if sm {
				shipMatch++
				if s.FleetID == nil {
					missingFleetID++
					continue
				}
				if fleet, ok := gs.Fleet[*s.FleetID]; ok {
					for _, id := range fleet.Ships {
						if id == *l.Location.IDRaw {
							reciprocal++
							break
						}
					}
				}
			}
			if fm {
				fleetMatch++
			}
			if sm && fm {
				both++
			}
		}
	}
	t.Logf("leader ship locations=%d ship_id_matches=%d fleet_id_matches=%d both=%d reciprocal_ship_fleet=%d", total, shipMatch, fleetMatch, both, reciprocal)
	t.Logf("missing_location_id=%d missing_ship_fleet_id=%d", missingLocationID, missingFleetID)
}
