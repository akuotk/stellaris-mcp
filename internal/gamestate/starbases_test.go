package gamestate

import (
	"os"
	"testing"
)

func TestStarbasesKeepReferencesSlotsAndMissingValues(t *testing.T) {
	gs, err := Load([]byte(`starbase_mgr={starbases={
	 0={station=0 build_queue=0 shipyard_build_queue=0 level=outpost type=starbase construction_type=starbase
	 ship_design_implementation={design=0 upgrade=0 growth_stage=0}
	 modules={2=none 0=shipyard 0=anchorage} buildings={} orbitals={4=0}}
	 1={} 2=none
	}}`))
	if err != nil {
		t.Fatal(err)
	}
	bases := gs.Starbases
	if !bases.Recorded || bases.TableIssue || len(bases.DecodeErrors) != 0 || len(bases.Items) != 2 {
		t.Fatalf("unexpected starbase table state: %+v", bases)
	}
	base := bases.Items[0]
	for _, ref := range []*int{base.Station, base.BuildQueue, base.ShipyardBuildQueue, base.ShipDesign.Design, base.ShipDesign.Upgrade, base.ShipDesign.GrowthStage} {
		if ref == nil || *ref != 0 {
			t.Fatal("explicit zero reference lost")
		}
	}
	if base.Level == nil || *base.Level != "outpost" || !base.Modules.Recorded || base.Modules.Unsupported || len(base.Modules.Entries) != 3 {
		t.Fatal("saved metadata or slots lost")
	}
	if base.Modules.Entries[0] != (StarbaseSlot{Key: "2", Value: "none"}) || base.Modules.Entries[1].Key != "0" || base.Modules.Entries[2].Value != "anchorage" {
		t.Fatal("slot order, duplicate keys or explicit none lost")
	}
	if !base.Buildings.Recorded || len(base.Buildings.Entries) != 0 || base.Orbitals.Entries[0].Value != "0" {
		t.Fatal("empty or raw orbital values lost")
	}
	missing := bases.Items[1]
	if missing.Station != nil || missing.Level != nil || missing.Modules.Recorded || missing.Buildings.Recorded || missing.Orbitals.Recorded {
		t.Fatal("missing values acquired defaults")
	}
}

func TestStarbasesUnsupportedRecordsStayIsolated(t *testing.T) {
	for _, body := range []string{
		`starbase_mgr=none`,
		`starbase_mgr={starbases=none}`,
		`starbase_mgr={starbases={unknown={}}}`,
		`starbase_mgr={unexpected={}}`,
		`starbase_mgr={starbases={} anonymous}`,
		`starbase_mgr={starbases={} starbases={}}`,
	} {
		gs, err := Load([]byte(`date=2201.01.01 ` + body + ` country={0={num_sapient_pops=7}}`))
		if err != nil || !gs.Starbases.Recorded || !gs.Starbases.TableIssue || gs.Date != "2201.01.01" || gs.Country[0].NumPops != 7 {
			t.Fatalf("unsupported optional table affected unrelated data for %q", body)
		}
	}
	gs, err := Load([]byte(`starbase_mgr={starbases={
	 0={station=invalid} 1={station=7 modules={unknown=value 0=none 1={nested=yes}} buildings={0=lab}}
	 2={station=8} 2=none 3=none 3={station=9} 4={modules={0=none anonymous} orbitals={}} 5=invalid
	}}`))
	if err != nil {
		t.Fatal("optional record errors broke whole save")
	}
	if len(gs.Starbases.DecodeErrors) != 4 || gs.Starbases.DecodeErrors[0] == "" || gs.Starbases.DecodeErrors[2] == "" || gs.Starbases.DecodeErrors[3] == "" || gs.Starbases.DecodeErrors[5] == "" {
		t.Fatal("malformed or duplicate record not flagged")
	}
	base := gs.Starbases.Items[1]
	if !base.Modules.Unsupported || len(base.Modules.Entries) != 1 || base.Modules.Entries[0].Value != "none" || len(base.Buildings.Entries) != 1 {
		t.Fatal("unsupported slots lost supported neighboring fields")
	}
	if !gs.Starbases.Items[4].Modules.Unsupported || !gs.Starbases.Items[4].Orbitals.Recorded {
		t.Fatal("mixed slots silently accepted or later fields lost")
	}
	gs, err = Load([]byte(`date=2201.01.01`))
	if err != nil || gs.Starbases.Recorded {
		t.Fatal("missing table presented as recorded")
	}
}

// This opt-in survey reads every local save and logs aggregate counts only.
func TestStarbasesLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local starbase coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	decoded, tables, incomplete, records, unsupportedSlots := 0, 0, 0, 0, 0
	stationCandidates, matchingDesigns, differingDesigns, systemRefs, unresolvedSystemRefs := 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("catalog save index %d whole-save decoding failed", index)
		}
		decoded++
		if gs.Starbases.Recorded {
			tables++
		}
		if gs.Starbases.TableIssue || len(gs.Starbases.DecodeErrors) > 0 {
			incomplete++
		}
		records += len(gs.Starbases.Items)
		for _, base := range gs.Starbases.Items {
			if base.Modules.Unsupported || base.Buildings.Unsupported || base.Orbitals.Unsupported {
				unsupportedSlots++
			}
			if base.Station == nil {
				continue
			}
			if ship, ok := gs.Ships[*base.Station]; ok {
				stationCandidates++
				if base.ShipDesign.Design != nil && ship.ShipDesignImplementation.Design != nil {
					if *base.ShipDesign.Design == *ship.ShipDesignImplementation.Design {
						matchingDesigns++
					} else {
						differingDesigns++
					}
				}
			}
		}
		for _, system := range gs.Systems {
			for _, id := range system.StarbaseRefs.Values {
				systemRefs++
				if _, ok := gs.Starbases.Items[id]; !ok {
					unresolvedSystemRefs++
				}
			}
		}
	}
	t.Logf("local starbase coverage: saves=%d decoded=%d recorded_tables=%d incomplete_tables=%d records=%d unsupported_slot_records=%d candidate_station_ship_ids=%d agreeing_design_ids=%d differing_design_ids=%d system_references=%d unresolved_system_references=%d", len(files), decoded, tables, incomplete, records, unsupportedSlots, stationCandidates, matchingDesigns, differingDesigns, systemRefs, unresolvedSystemRefs)
	if incomplete > 0 || unsupportedSlots > 0 {
		t.Fatal("local starbase shapes need additional support")
	}
}
