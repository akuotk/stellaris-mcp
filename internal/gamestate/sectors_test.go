package gamestate

import (
	"os"
	"testing"
)

func TestSectorsKeepRawValuesAndOptionalReferences(t *testing.T) {
	gs, err := Load([]byte(`country={0={sectors={owned={0 0 2}}} 1={sectors={owned={}}} 2={sectors={}} 3={}}
	 sectors={0={name={key="Synthetic sector"} owner=0 local_capital=0 type=0 auto_expand=unknown systems={0 0 2}} 1={} 9=none}
	 galactic_object={0={sector=0} 1={sector=unsupported} 2={} 3={sector=0 sector=1 name={key="Retained system"} planet={5}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !gs.Sectors.Recorded || gs.Sectors.TableIssue || len(gs.Sectors.DecodeErrors) != 0 || len(gs.Sectors.Items) != 2 {
		t.Fatal("supported sector table rejected")
	}
	s := gs.Sectors.Items[0]
	if s.Owner == nil || *s.Owner != 0 || s.LocalCapital == nil || *s.LocalCapital != 0 || *s.Type != "0" || *s.AutoExpand != "unknown" || !s.Systems.Recorded || len(s.Systems.Values) != 3 || s.Systems.Values[1] != 0 {
		t.Fatal("raw sector metadata, zero refs, duplicates or order lost")
	}
	if gs.Sectors.Items[1].Owner != nil || gs.Sectors.Items[1].Systems.Recorded {
		t.Fatal("missing sector fields acquired defaults")
	}
	refs := gs.Country[0].SectorRefs
	if !refs.Recorded || refs.Unsupported || !refs.Owned.Recorded || len(refs.Owned.Values) != 3 || refs.Owned.Values[1] != 0 || !gs.Country[1].SectorRefs.Owned.Recorded || len(gs.Country[1].SectorRefs.Owned.Values) != 0 || gs.Country[2].SectorRefs.Owned.Recorded || gs.Country[3].SectorRefs.Recorded {
		t.Fatal("country sector refs lost or missing/empty conflated")
	}
	if !gs.Systems[0].SectorRef.Recorded || gs.Systems[0].SectorRef.Value == nil || *gs.Systems[0].SectorRef.Value != 0 || !gs.Systems[1].SectorRef.Unsupported || gs.Systems[1].SectorRef.Value != nil || gs.Systems[2].SectorRef.Recorded {
		t.Fatal("optional system refs lost or unsupported values became zero")
	}
	if !gs.Systems[3].SectorRef.Unsupported || gs.Systems[3].SectorRef.Value != nil || len(gs.Systems[3].Planets) != 1 || gs.Systems[3].Name.Key != "Retained system" {
		t.Fatal("duplicate system sector accepted or neighboring fields lost")
	}
}

func TestSectorsUnsupportedAndDuplicateShapesStayIsolated(t *testing.T) {
	for _, data := range []string{`sectors=none`, `sectors={unexpected={}}`, `sectors={0={} anonymous}`} {
		gs, err := Load([]byte(`date=2201.01.01 ` + data))
		if err != nil || gs.Date != "2201.01.01" || !gs.Sectors.TableIssue {
			t.Fatal("unsupported optional sector table lost unrelated data or was silently accepted")
		}
	}
	gs, err := Load([]byte(`date=2201.01.01 country={0={sectors={owned={0} owned={1}}} 1={sectors=none} 2={sectors={owned={bad}}}}
	 sectors={0={owner=invalid} 1={owner=0 owner=1} 2={systems={0} systems={1}} 3={owner=0} 3=none 4=none 4={owner=0} 5={owner=0 systems={bad}} 6=invalid}`))
	if err != nil || gs.Date != "2201.01.01" {
		t.Fatal("unsupported optional sector record prevented save load")
	}
	if len(gs.Sectors.DecodeErrors) != 6 || len(gs.Sectors.Items) != 1 || !gs.Sectors.Items[5].Systems.Unsupported {
		t.Fatal("malformed or duplicate sector could remain usable as an owner path")
	}
	if !gs.Country[0].SectorRefs.Unsupported || len(gs.Country[0].SectorRefs.Owned.Values) != 0 || !gs.Country[1].SectorRefs.Unsupported || !gs.Country[2].SectorRefs.Owned.Unsupported {
		t.Fatal("unsupported or duplicate country refs accepted")
	}
}

// This opt-in survey reads every local save and logs aggregate counts only.
func TestSectorsLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local sector coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	decoded, tables, sectors, countryRefs, reciprocalOwners, missingSectors, ownerDisagreements, systemRefs, reciprocalSystems, missingSystems, systemDisagreements, reverseWithoutMembership, unresolvedSystemSectors, capitalColonyCandidates, capitalPlanetCandidates, unsupported := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("catalog save index %d whole-save decoding failed", index)
		}
		decoded++
		if gs.Sectors.Recorded {
			tables++
		}
		sectors += len(gs.Sectors.Items)
		if gs.Sectors.TableIssue {
			unsupported++
		}
		unsupported += len(gs.Sectors.DecodeErrors)
		for countryID, c := range gs.Country {
			if c.SectorRefs.Unsupported || c.SectorRefs.Owned.Unsupported {
				unsupported++
			}
			for _, id := range c.SectorRefs.Owned.Values {
				countryRefs++
				sector, ok := gs.Sectors.Items[id]
				if !ok {
					missingSectors++
				} else if sector.Owner != nil && *sector.Owner == countryID {
					reciprocalOwners++
				} else {
					ownerDisagreements++
				}
			}
		}
		for sectorID, sector := range gs.Sectors.Items {
			if sector.Systems.Unsupported {
				unsupported++
			}
			for _, id := range sector.Systems.Values {
				systemRefs++
				system, ok := gs.Systems[id]
				if !ok {
					missingSystems++
				} else if system.SectorRef.Value != nil && !system.SectorRef.Unsupported && *system.SectorRef.Value == sectorID {
					reciprocalSystems++
				} else {
					systemDisagreements++
				}
			}
			if sector.LocalCapital != nil {
				if _, ok := gs.Colonies[*sector.LocalCapital]; ok {
					capitalColonyCandidates++
				}
				if _, ok := gs.Planets.Planet[*sector.LocalCapital]; ok {
					capitalPlanetCandidates++
				}
			}
		}
		for systemID, system := range gs.Systems {
			if system.SectorRef.Unsupported {
				unsupported++
			}
			if system.SectorRef.Value == nil || system.SectorRef.Unsupported {
				continue
			}
			sector, ok := gs.Sectors.Items[*system.SectorRef.Value]
			if !ok {
				unresolvedSystemSectors++
				continue
			}
			found := false
			for _, id := range sector.Systems.Values {
				if id == systemID {
					found = true
					break
				}
			}
			if !found {
				reverseWithoutMembership++
			}
		}
	}
	t.Logf("local sector coverage: saves=%d decoded=%d recorded_tables=%d sector_records=%d country_owned_refs=%d reciprocal_owner_refs=%d missing_sector_records=%d owner_disagreements=%d sector_system_refs=%d reciprocal_system_refs=%d missing_system_records=%d system_disagreements=%d system_reverse_without_membership=%d system_sector_ids_without_record=%d local_capital_colony_id_candidates=%d local_capital_planet_id_candidates=%d unsupported_shapes=%d", len(files), decoded, tables, sectors, countryRefs, reciprocalOwners, missingSectors, ownerDisagreements, systemRefs, reciprocalSystems, missingSystems, systemDisagreements, reverseWithoutMembership, unresolvedSystemSectors, capitalColonyCandidates, capitalPlanetCandidates, unsupported)
	if unsupported > 0 {
		t.Fatal("local sector shapes need additional support")
	}
}
