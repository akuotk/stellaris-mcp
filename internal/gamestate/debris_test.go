package gamestate

import (
	"os"
	"testing"
)

// The opt-in survey reports counts only. It never logs saved IDs or values.
func TestDebrisLocalCatalogCoverage(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only debris coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	counts := map[string]int{}
	fields := map[string]int{}
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("save index %d whole-save decoding failed", index)
		}
		if !gs.Debris.Recorded || gs.Debris.Unsupported || gs.Debris.InvalidKeys > 0 {
			counts["incomplete_tables"]++
		}
		refs := map[int]int{}
		firstCountry := map[int]int{}
		crossCountry := map[int]bool{}
		for countryID, country := range gs.Country {
			for _, project := range country.SpecialProjects.Items {
				if project.Debris.Unsupported {
					counts["unsupported_project_refs"]++
				}
				if project.Debris.Value == nil {
					continue
				}
				id := *project.Debris.Value
				counts["project_refs"]++
				if refs[id] == 0 {
					firstCountry[id] = countryID
				} else if firstCountry[id] != countryID {
					crossCountry[id] = true
				}
				refs[id]++
				entries := gs.Debris.Items[id]
				if len(entries) != 1 || !entries[0].Value.IsObject() {
					counts["unresolved_project_refs"]++
					continue
				}
				counts["unique_record_candidates"]++
				if ref := entries[0].Country.Value; ref != nil {
					if *ref == countryID {
						counts["candidate_country_id_agrees"]++
					} else {
						counts["candidate_country_id_differs"]++
					}
				}
			}
		}
		counts["records_referenced_by_multiple_countries"] += len(crossCountry)
		for id, entries := range gs.Debris.Items {
			if len(entries) > 1 {
				counts["duplicate_record_ids"]++
			}
			if refs[id] > 1 {
				counts["record_ids_with_multiple_refs"]++
			}
			for _, entry := range entries {
				switch {
				case entry.IsSavedNone():
					counts["saved_none_entries"]++
				case !entry.Value.IsObject():
					counts["unsupported_record_shapes"]++
				default:
					counts["object_records"]++
					if refs[id] == 0 {
						counts["objects_without_project_refs"]++
					}
					if entry.Country.Unsupported {
						counts["unsupported_country_fields"]++
					}
					for _, field := range entry.Value.Object.Pairs {
						switch field.Key {
						case "country", "from_country", "date", "coordinate", "component", "ship_size", "resources", "from_fleet_name", "must_reanimate", "must_research", "must_scavenge":
							fields[field.Key]++
						default:
							fields["other_saved_fields"]++
						}
					}
				}
			}
		}
	}
	t.Logf("local debris coverage: saves=%d totals=%v saved_field_counts=%v", len(files), counts, fields)
}
