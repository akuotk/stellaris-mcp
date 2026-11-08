package gamestate

import (
	"os"
	"slices"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

// The opt-in audit logs aggregate counts without saved IDs or field values.
func TestSystemContentsLocalCatalogCoverage(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only system contents coverage")
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
		root, err := clausewitz.Parse(data)
		if err != nil {
			t.Fatalf("save index %d parse failed", index)
		}
		value := clausewitz.Value{Object: root}
		systems := systemSubsetTable[SavedSystemContents](value, nil, "galactic_object")
		planets := systemSubsetTable[SavedSystemPlanet](value, nil, "planets", "planet")
		deposits := systemSubsetTable[Deposit](value, nil, "deposit")
		counts["system_records"] += len(systems.Items)
		counts["planet_records"] += len(planets.Items)
		counts["record_decode_errors"] += len(systems.DecodeErrors) + len(planets.DecodeErrors) + len(deposits.DecodeErrors)
		for id, system := range systems.Items {
			for _, key := range []string{"star_class", "coordinate", "asteroid_belts", "ambient_object"} {
				if _, recorded, unsupported := savedFieldPath(system.Value, key); recorded && !unsupported {
					counts["system_"+key+"_fields"]++
				}
			}
			for _, body := range system.Bodies {
				counts["system_planet_refs"]++
				if body.ID.Value == nil {
					counts["unsupported_planet_refs"]++
					continue
				}
				if planet, ok := planets.Items[*body.ID.Value]; ok {
					counts["system_planet_candidates"]++
					if planet.CoordinateOrigin.Value != nil && *planet.CoordinateOrigin.Value == id {
						counts["planet_origin_agrees_with_system_id"]++
					}
				}
			}
		}
		for id, planet := range planets.Items {
			if parentID := planet.MoonOf.Value; parentID != nil {
				counts["moon_of_refs"]++
				if parent, ok := planets.Items[*parentID]; ok && slices.Contains(parent.Moons.Values, id) {
					counts["reciprocal_moon_ids"]++
					if parent.CoordinateOrigin.Value != nil && planet.CoordinateOrigin.Value != nil && *parent.CoordinateOrigin.Value == *planet.CoordinateOrigin.Value {
						counts["reciprocal_moons_share_origin"]++
					}
				}
			}
			counts["moons_list_refs"] += len(planet.Moons.Values)
			for _, depositID := range planet.Deposits.Values {
				counts["deposit_refs"]++
				if deposit, ok := deposits.Items[depositID]; ok {
					counts["deposit_candidates"]++
					if deposit.Holder.ID.Value != nil && *deposit.Holder.ID.Value == id {
						counts["deposit_holder_numeric_id_agrees"]++
					}
				}
			}
		}
	}
	t.Logf("local system contents coverage: saves=%d counts=%v", len(files), counts)
}
