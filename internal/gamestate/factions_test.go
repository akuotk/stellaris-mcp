package gamestate

import (
	"os"
	"slices"
	"testing"
)

func TestFactionShapesAndDuplicateReferencesStayIsolated(t *testing.T) {
	gs, err := Load([]byte(`date=2201.01.01 country={0={modules={standard_pop_factions_module={pop_factions={0} pop_factions={1}}}}
	1={modules={standard_pop_factions_module={} standard_pop_factions_module={}}}
	2={modules=bad}}
	pop_factions={0={country=0 members={0} last_month_colony_pops={{colony=0 num_pops=00.0} {colony=0 colony=1} bad}} 1={} 1=none}
	pop={0={pop_faction=0} 1={pop_faction=0 pop_faction=1} 2={pop_faction={unknown=yes}}}
	pop_groups={0={factions={objects={0} pops={0}}} 1={factions={objects={0} objects={1} pops={0}}}
	2={factions=bad} 3={factions={objects={0} pops={0}}} 3=none}`))
	if err != nil {
		t.Fatal(err)
	}
	c0, c1, c2 := gs.Country[0], gs.Country[1], gs.Country[2]
	faction := gs.PopFactions.Items[0]
	if gs.Date != "2201.01.01" || !c0.FactionModules.Module.FactionRefs.Duplicate ||
		!c1.FactionModules.Duplicate || !c2.FactionModules.Unsupported ||
		len(gs.PopFactions.DecodeErrors) != 1 || len(faction.Members.Values) != 1 || faction.Members.Values[0] != 0 {
		t.Fatal("faction ownership fields lost or duplicate paths remained authoritative")
	}
	history := faction.ColonyHistory.Items
	if len(history) != 3 || history[0].Colony.Value == nil || *history[0].Colony.Value != 0 ||
		history[0].NumPops == nil || *history[0].NumPops != "00.0" || !history[1].Duplicate || !history[2].Unsupported {
		t.Fatal("faction colony history lost zero, raw precision or unsupported state")
	}
	if p := gs.LegacyFactionPops.Items[0]; p.FactionRef.Value == nil || *p.FactionRef.Value != 0 ||
		!gs.LegacyFactionPops.Items[1].Duplicate || !gs.LegacyFactionPops.Items[2].FactionRef.Unsupported ||
		!gs.PopGroups[1].Factions.Duplicate || !gs.PopGroups[2].Factions.Unsupported || !gs.PopGroups[3].Factions.Duplicate {
		t.Fatal("population back-references lost zero or ambiguities")
	}
}

func TestFactionsLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local faction coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	var modules, factions, ownerRefs, ownerMatches, members, legacyReciprocal, groupReciprocal, collisions, historyRows, colonyMatches, unsupported int
	for _, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatal("whole-save decoding failed")
		}
		if gs.PopFactions.TableIssue || gs.LegacyFactionPops.TableIssue {
			unsupported++
		}
		unsupported += len(gs.PopFactions.DecodeErrors) + len(gs.LegacyFactionPops.DecodeErrors)
		for id, country := range gs.Country {
			m := country.FactionModules
			if m.Duplicate || m.Unsupported {
				unsupported++
			}
			if m.Module == nil {
				continue
			}
			modules++
			if m.Module.FactionRefs.Duplicate || m.Module.FactionRefs.Unsupported {
				unsupported++
			}
			for _, ref := range m.Module.FactionRefs.Values {
				ownerRefs++
				if faction, ok := gs.PopFactions.Items[ref]; ok && faction.Country != nil && *faction.Country == id {
					ownerMatches++
				}
			}
		}
		for id, faction := range gs.PopFactions.Items {
			factions++
			if faction.Members.Duplicate || faction.Members.Unsupported || faction.ColonyHistory.Duplicate || faction.ColonyHistory.Unsupported {
				unsupported++
			}
			for _, member := range faction.Members.Values {
				members++
				pop, legacy := gs.LegacyFactionPops.Items[member]
				group, modern := gs.PopGroups[member]
				switch {
				case legacy && modern:
					collisions++
				case legacy && !pop.Duplicate && pop.FactionRef.Value != nil && *pop.FactionRef.Value == id:
					legacyReciprocal++
				case modern && !group.Factions.Duplicate && !group.Factions.Unsupported &&
					len(group.Factions.Objects) == len(group.Factions.Pops) && slices.Contains(group.Factions.Objects, id):
					groupReciprocal++
				}
			}
			for _, row := range faction.ColonyHistory.Items {
				historyRows++
				if row.Duplicate || row.Unsupported || row.Colony.Unsupported {
					unsupported++
				}
				if row.Colony.Value != nil {
					if _, ok := gs.Colonies[*row.Colony.Value]; ok {
						colonyMatches++
					}
				}
			}
		}
	}
	t.Logf("local faction coverage: saves=%d modules=%d factions=%d country_refs=%d reciprocal_country_refs=%d member_refs=%d legacy_reciprocal=%d pop_group_reciprocal=%d namespace_collisions=%d colony_history_rows=%d matched_colony_records=%d unsupported=%d", len(files), modules, factions, ownerRefs, ownerMatches, members, legacyReciprocal, groupReciprocal, collisions, historyRows, colonyMatches, unsupported)
	if unsupported != 0 {
		t.Fatal("local faction shapes need additional support")
	}
}
