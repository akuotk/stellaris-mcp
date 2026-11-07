package gamestate

import (
	"os"
	"testing"
)

func TestLoadWarDatabase(t *testing.T) {
	gs, err := Load([]byte(`war={ 7=none 9={ name={ key="Test War" } start_date="2263.07.09" attackers={ { country=2 } } defenders={ country=0 } } }`))
	if err != nil {
		t.Fatal(err)
	}
	w := gs.War.Items[9]
	if len(gs.War.Items) != 1 || w.StartDate.Value != "2263.07.09" || w.Name.Key != "Test War" || len(w.Attackers.Items) != 1 || len(w.Defenders.Items) != 1 {
		t.Fatalf("unexpected wars: %#v", gs.War)
	}
	attacker := SavedWarIntRef(w.Attackers.Items[0], "country")
	defender := SavedWarIntRef(w.Defenders.Items[0], "country")
	if attacker.Value == nil || *attacker.Value != 2 || defender.Value == nil || *defender.Value != 0 {
		t.Fatal("war country references did not preserve IDs")
	}
}

// The opt-in audit logs aggregate counts without saved IDs or field values.
func TestWarsLocalCatalogCoverage(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only war coverage")
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
		if !gs.War.Recorded || gs.War.TableIssue || len(gs.War.DecodeErrors) > 0 {
			counts["incomplete_or_missing_tables"]++
		}
		counts["wars"] += len(gs.War.Items)
		for _, war := range gs.War.Items {
			if war.Battles.Recorded {
				counts["battle_lists"]++
			}
			if war.Battles.Unsupported || war.Attackers.Unsupported || war.Defenders.Unsupported || war.NameUnsupported {
				counts["unsupported_war_fields"]++
			}
			parties := map[int]bool{}
			for _, side := range []SavedWarList{war.Attackers, war.Defenders} {
				for _, party := range side.Items {
					counts["war_party_rows"]++
					if ref := SavedWarIntRef(party, "country"); ref.Value != nil {
						parties[*ref.Value] = true
					}
				}
			}
			for _, battle := range war.Battles.Items {
				counts["battles"]++
				if !battle.IsObject() {
					counts["unsupported_battle_records"]++
					continue
				}
				for _, side := range []string{"attackers", "defenders"} {
					refs := SavedWarBattleCountries(battle, side)
					if refs.Unsupported {
						counts["unsupported_battle_side_lists"]++
					}
					for _, id := range refs.Values {
						counts["battle_country_refs"]++
						if _, ok := gs.Country[id]; ok {
							counts["battle_country_id_candidates"]++
						}
						if parties[id] {
							counts["battle_country_ids_also_in_war_parties"]++
						}
					}
				}
				for _, key := range []string{"system", "planet", "colony"} {
					ref := SavedWarIntRef(battle, key)
					if ref.Unsupported {
						counts["unsupported_battle_"+key+"_refs"]++
					}
					if ref.Value == nil {
						continue
					}
					counts["battle_"+key+"_refs"]++
					id := *ref.Value
					var found bool
					switch key {
					case "system":
						_, found = gs.Systems[id]
					case "planet":
						_, found = gs.Planets.Planet[id]
					case "colony":
						_, found = gs.Colonies[id]
					}
					if found {
						counts["battle_"+key+"_id_candidates"]++
					} else {
						counts["unmatched_battle_"+key+"_refs"]++
					}
				}
			}
		}
	}
	t.Logf("local war coverage: saves=%d counts=%v", len(files), counts)
}
