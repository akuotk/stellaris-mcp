package gamestate

import (
	"os"
	"slices"
	"testing"
)

func TestSurfaceReferenceShapesAndReciprocalProjection(t *testing.T) {
	gs, err := Load([]byte(`planets={planet={0={colony=0 deposits={0 0} district=type_a district=type_a buildings={0}
	terraform_process={progress=00.00}} 1={deposits={0} deposits={} buildings={unknown=yes}}}}
	colony={0={carrier={type=planet reference=0} districts={0} buildings_cache={0}
	externally_owned_buildings={{owner_type=unknown building_owner=0 buildings={0}} bad}}}
	districts={0={type=type_a level=01 zones={0 99}}}
	zones={0={type=zone_default buildings={0}}}
	buildings={0={type=building_example position=00 ruined=no} 1={} 1=none}
	deposit={0={type=deposit_example deposit_holder={type=0 id=0}}
	1={type=deposit_other deposit_holder={type=0 id=0 id=1}}}`))
	if err != nil {
		t.Fatal(err)
	}
	p := gs.Planets.Planet[0]
	if !slices.Equal(p.DistrictRefs.Values, []int{0}) || !slices.Equal(p.LegacyBuildingRefs.Values, []int{0}) ||
		len(p.DepositRefs.Values) != 2 || len(p.LegacyDistricts) != 2 || p.TerraformProcess == nil ||
		!gs.Planets.Planet[1].DepositRefs.Duplicate || !gs.Planets.Planet[1].LegacyBuildingRefs.Unsupported {
		t.Fatal("surface list presence, zero references, duplicate or legacy values lost")
	}
	b, d := gs.Buildings.Items[0], gs.Deposit.Items[0]
	if b.Position.Value != "00" || b.Ruined.Value != "no" || len(gs.Buildings.DecodeErrors) != 1 ||
		d.Holder.ID.Value == nil || *d.Holder.ID.Value != 0 || !gs.Deposit.Items[1].Holder.Duplicate ||
		!gs.Colonies[0].ExternalBuildings.Items[1].Unsupported {
		t.Fatal("surface records lost raw values or error isolation")
	}
	gs, err = Load([]byte(`date=2201.01.01 buildings={0={}} buildings={}`))
	if err != nil || gs.Date != "2201.01.01" || !gs.Buildings.TableIssue || len(gs.Buildings.Items) != 0 {
		t.Fatal("duplicate surface table remained authoritative or affected other data")
	}
}

func TestSurfaceLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local surface coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	var legacyDistricts, legacyBuildings, legacyMatches, depositRefs, depositMatches, holderMatches, terrain int
	var districtRefs, districtMatches, zoneRefs, zoneMatches, zoneMax, zoneBuildings, zoneBuildingMatches, cacheMatches, externalRefs, externalMatches, unsupported int
	for _, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatal("whole-save decoding failed")
		}
		if gs.Deposit.TableIssue || gs.Districts.TableIssue || gs.Zones.TableIssue || gs.Buildings.TableIssue {
			unsupported++
		}
		unsupported += len(gs.Deposit.DecodeErrors) + len(gs.Districts.DecodeErrors) + len(gs.Zones.DecodeErrors) + len(gs.Buildings.DecodeErrors)
		countExternal := func(list ExternalSurfaceBuildings) {
			if list.Unsupported || list.Duplicate {
				unsupported++
			}
			for _, row := range list.Items {
				if row.Unsupported || row.Duplicate || row.BuildingRefs.Unsupported || row.BuildingRefs.Duplicate {
					unsupported++
				}
				for _, id := range row.BuildingRefs.Values {
					externalRefs++
					if _, ok := gs.Buildings.Items[id]; ok {
						externalMatches++
					}
				}
			}
		}
		for id, p := range gs.Planets.Planet {
			legacyDistricts += len(p.LegacyDistricts)
			if p.TerraformProcess != nil {
				terrain++
			}
			for _, ref := range p.LegacyBuildingRefs.Values {
				legacyBuildings++
				if _, ok := gs.Buildings.Items[ref]; ok {
					legacyMatches++
				}
			}
			for _, ref := range p.DepositRefs.Values {
				depositRefs++
				if d, ok := gs.Deposit.Items[ref]; ok {
					depositMatches++
					if d.Holder.ID.Value != nil && *d.Holder.ID.Value == id {
						holderMatches++
					}
				}
			}
			countExternal(p.ExternalBuildings)
		}
		for _, c := range gs.Colonies {
			countExternal(c.ExternalBuildings)
			for _, id := range c.DistrictRefs.Values {
				districtRefs++
				d, ok := gs.Districts.Items[id]
				if !ok {
					continue
				}
				districtMatches++
				for _, zid := range d.ZoneRefs.Values {
					zoneRefs++
					if int64(zid) == 4294967295 {
						zoneMax++
					}
					z, ok := gs.Zones.Items[zid]
					if !ok {
						continue
					}
					zoneMatches++
					for _, bid := range z.BuildingRefs.Values {
						zoneBuildings++
						if _, ok := gs.Buildings.Items[bid]; ok {
							zoneBuildingMatches++
						}
						if slices.Contains(c.BuildingCacheRefs.Values, bid) {
							cacheMatches++
						}
					}
				}
			}
		}
	}
	t.Logf("local surface coverage: saves=%d legacy_district_entries=%d legacy_buildings=%d legacy_matches=%d deposit_refs=%d deposit_matches=%d holder_ids_match=%d terraform_processes=%d districts=%d district_matches=%d zone_refs=%d zone_matches=%d zone_ref_raw_4294967295=%d zone_buildings=%d zone_building_matches=%d zone_buildings_in_cache=%d external_refs=%d external_matches=%d unsupported=%d", len(files), legacyDistricts, legacyBuildings, legacyMatches, depositRefs, depositMatches, holderMatches, terrain, districtRefs, districtMatches, zoneRefs, zoneMatches, zoneMax, zoneBuildings, zoneBuildingMatches, cacheMatches, externalRefs, externalMatches, unsupported)
	if unsupported != 0 {
		t.Fatal("local surface shapes need additional support")
	}
}
