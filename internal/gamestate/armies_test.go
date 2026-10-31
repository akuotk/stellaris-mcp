package gamestate

import (
	"os"
	"testing"
)

func TestArmyReferencesAndZeroPresence(t *testing.T) {
	gs, err := Load([]byte(`army={0={name={key="Synthetic army"} type="synthetic_assault" owner=0 species=0 colony=0 spawning_colony=0 ship=0 leader=0 planet=0 health=0 max_health=10.5 morale=0 experience=0 jump_drive_cooldown="2201.01.01"} 1={} 8=none}`))
	if err != nil {
		t.Fatal(err)
	}
	if gs.Armies.TableIssue || len(gs.Armies.DecodeErrors) != 0 || len(gs.Armies.Items) != 2 {
		t.Fatal("supported army records not parsed")
	}
	a := gs.Armies.Items[0]
	for _, v := range []*int{a.Owner, a.Species, a.Colony, a.SpawningColony, a.Ship, a.Leader, a.Planet} {
		if v == nil || *v != 0 {
			t.Fatal("valid zero reference lost")
		}
	}
	for _, v := range []*float64{a.Health, a.Morale, a.Experience} {
		if v == nil || *v != 0 {
			t.Fatal("valid zero statistic lost")
		}
	}
	if a.MaxHealth == nil || *a.MaxHealth != 10.5 || a.JumpDriveCooldown != "2201.01.01" {
		t.Fatal("raw statistic/date changed")
	}
	empty := gs.Armies.Items[1]
	if empty.Owner != nil || empty.Ship != nil || empty.Health != nil || empty.Colony != nil || empty.Planet != nil {
		t.Fatal("missing references became zero")
	}
}

func TestArmiesUnsupportedAndDuplicateRecordsDoNotBreakSave(t *testing.T) {
	gs, err := Load([]byte(`date="2201.01.01" army={0={owner=0 health={unexpected=shape}} 1={owner=0} 1={owner=1} not_an_id={owner=0}}`))
	if err != nil {
		t.Fatal("optional army shape broke full save")
	}
	if gs.Date != "2201.01.01" || !gs.Armies.TableIssue || len(gs.Armies.DecodeErrors) != 2 || len(gs.Armies.Items) != 2 {
		t.Fatal("unsupported/duplicate records not flagged")
	}
	gs, err = Load([]byte(`army=unsupported date="2201.01.02"`))
	if err != nil || !gs.Armies.TableIssue || gs.Date != "2201.01.02" {
		t.Fatal("unsupported table shape not isolated")
	}
}

func TestArmyLocalCatalogReferenceCoverage(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for local read-only army coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil {
		t.Fatal("save catalog read failed")
	}
	records, unsupported, transportRefs, transportReciprocal, colonyRefs, colonyReciprocal, spawnRefs, spawnReciprocal, legacyPlanetRefs, missingLocation := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("save index %d whole-save parsing failed", index)
		}
		if gs.Armies.TableIssue {
			unsupported++
		}
		unsupported += len(gs.Armies.DecodeErrors)
		records += len(gs.Armies.Items)
		for id, a := range gs.Armies.Items {
			if a.Ship != nil {
				transportRefs++
				if ship, ok := gs.Ships[*a.Ship]; ok && ship.ArmyID != nil && *ship.ArmyID == id {
					transportReciprocal++
				}
			}
			if a.Colony != nil {
				colonyRefs++
				if c, ok := gs.Colonies[*a.Colony]; ok {
					for _, ref := range c.Armies {
						if ref == id {
							colonyReciprocal++
							break
						}
					}
				}
			}
			if a.SpawningColony != nil {
				spawnRefs++
				if c, ok := gs.Colonies[*a.SpawningColony]; ok {
					for _, ref := range c.SpawnedArmies {
						if ref == id {
							spawnReciprocal++
							break
						}
					}
				}
			}
			if a.Planet != nil {
				legacyPlanetRefs++
			}
			if a.Planet == nil && a.Colony == nil && a.Ship == nil {
				missingLocation++
			}
		}
	}
	t.Logf("local army coverage: saves=%d records=%d unsupported=%d ship_refs=%d reciprocal_ship_refs=%d colony_refs=%d reciprocal_colony_refs=%d spawn_refs=%d reciprocal_spawn_refs=%d planet_refs=%d missing_location=%d", len(files), records, unsupported, transportRefs, transportReciprocal, colonyRefs, colonyReciprocal, spawnRefs, spawnReciprocal, legacyPlanetRefs, missingLocation)
	if unsupported > 0 {
		t.Fatal("local army shape needs additional support")
	}
}

func TestArmyMixedRecordIsIsolated(t *testing.T) {
	gs, err := Load([]byte(`army={1={owner=0 mixed_unknown={a=1 anonymous} health=10} 2={owner=0 health=20} 3={owner={id=0 anonymous}}} date="2201.01.01"`))
	if err != nil {
		t.Fatal(err)
	}
	if gs.Date != "2201.01.01" || gs.Armies.Items[2].Health == nil || *gs.Armies.Items[2].Health != 20 {
		t.Fatal("unrelated records lost")
	}
	if _, bad := gs.Armies.DecodeErrors[3]; !bad {
		t.Fatal("unsupported known mixed value was silently accepted")
	}
}
