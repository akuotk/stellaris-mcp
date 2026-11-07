package gamestate

import (
	"os"
	"testing"
)

// This opt-in audit logs aggregate counts only; save data stays local.
func TestLocalUnresolvedStarbaseReferences(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("local catalog not set")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("catalog unavailable or empty")
	}
	var missingTables, incompleteTables, unsupportedLists int
	var saves, unresolved, negative, zero, sentinel, ships, fleets, planets, systems, queues, none int
	for _, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatal("save decoding failed")
		}
		saves++
		if !gs.Starbases.Recorded {
			missingTables++
		}
		if gs.Starbases.TableIssue || len(gs.Starbases.DecodeErrors) > 0 {
			incompleteTables++
		}
		for _, system := range gs.Systems {
			if system.StarbaseRefs.Unsupported {
				unsupportedLists++
			}
			for _, id := range system.StarbaseRefs.Values {
				if _, ok := gs.Starbases.Items[id]; ok {
					continue
				}
				unresolved++
				if id < 0 {
					negative++
				}
				if id == 0 {
					zero++
				}
				if uint64(id) == 4294967295 {
					sentinel++
				}
				matched := false
				if _, ok := gs.Ships[id]; ok {
					ships++
					matched = true
				}
				if _, ok := gs.Fleet[id]; ok {
					fleets++
					matched = true
				}
				if _, ok := gs.Planets.Planet[id]; ok {
					planets++
					matched = true
				}
				if _, ok := gs.Systems[id]; ok {
					systems++
					matched = true
				}
				if _, ok := gs.Construction.QueueManager.Queues[id]; ok {
					queues++
					matched = true
				}
				if !matched {
					none++
				}
			}
		}
	}
	t.Logf("missing_tables=%d incomplete_tables=%d unsupported_system_lists=%d", missingTables, incompleteTables, unsupportedLists)
	if missingTables > 0 || incompleteTables > 0 || unsupportedLists > 0 {
		t.Fatal("reference survey coverage incomplete")
	}
	t.Logf("saves=%d unresolved=%d negative=%d zero=%d max_u32=%d same_id_ship=%d same_id_fleet=%d same_id_planet=%d same_id_system=%d same_id_queue=%d no_id_match=%d", saves, unresolved, negative, zero, sentinel, ships, fleets, planets, systems, queues, none)
}
