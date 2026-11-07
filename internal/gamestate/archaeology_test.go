package gamestate

import (
	"os"
	"testing"
)

func TestArchaeologyZeroValuesHistoryAndOpaqueScope(t *testing.T) {
	gs, err := Load([]byte(`archaeological_sites={sites={0={location={type=0 id=0} last_excavator_country=0 excavator_fleet=0 type="synthetic_site" index=0 clues=0 last_roll=0 days_left=0 difficulty=0 locked=no visible_to={0}
 completed={{country=0 date="2201.01.01"} {country=0 date="2201.01.02"}}
 events={{expired=0 event_id="synthetic_event" scope={unknown_scope={nested=0} repeated=one repeated=two} fleet=0 owner=0 effect="synthetic_effect" picture="synthetic_picture" index=0 player_event=0}}
 log={{title="first" tooltip="example" clues=0 total=0 roll=0 bonus=0 difficulty=0 date="2201.01.01"} {title="second"}}
 flags={synthetic_flag=0}}
 1={} 8=none}}`))
	if err != nil {
		t.Fatal(err)
	}
	store := gs.ArchaeologySites
	if store.TableIssue || len(store.DecodeErrors) > 0 || len(store.Items) != 2 {
		t.Fatal("supported records rejected")
	}
	site := store.Items[0]
	for _, value := range []*int{site.Location.ID, site.LastExcavatorCountry, site.ExcavatorFleet, site.Index, site.Events[0].Fleet, site.Events[0].Owner, site.Events[0].Index} {
		if value == nil || *value != 0 {
			t.Fatal("zero reference lost")
		}
	}
	for _, value := range []*float64{site.Clues, site.LastRoll, site.DaysLeft, site.Difficulty, site.Log[0].Clues, site.Log[0].Total, site.Log[0].Roll, site.Log[0].Bonus, site.Log[0].Difficulty} {
		if value == nil || *value != 0 {
			t.Fatal("zero statistic lost")
		}
	}
	if site.Locked == nil || *site.Locked || len(site.Completed) != 2 || len(site.Log) != 2 || *site.Log[1].Title != "second" {
		t.Fatal("false/history/log order lost")
	}
	if site.Events[0].Scope == nil || len(site.Events[0].Scope.Value.Object.Pairs) != 3 {
		t.Fatal("opaque scope or repeated fields lost")
	}
	missing := store.Items[1]
	if missing.Location.ID != nil || missing.Location.Type != nil || missing.LastExcavatorCountry != nil || missing.Clues != nil || missing.Locked != nil || missing.VisibleTo != nil || missing.Events != nil {
		t.Fatal("missing values became recorded values")
	}
}

func TestArchaeologyUnsupportedAndDuplicateRecordsDoNotBreakSave(t *testing.T) {
	gs, err := Load([]byte(`date="2201.01.01" archaeological_sites={sites={0={visible_to={0} clues={unexpected=yes}} 1={} 1={} invalid_key={}}}`))
	if err != nil {
		t.Fatal("optional unsupported record broke save")
	}
	if gs.Date != "2201.01.01" || !gs.ArchaeologySites.TableIssue || len(gs.ArchaeologySites.DecodeErrors) != 2 {
		t.Fatal("unsupported/duplicate records not flagged")
	}
	gs, err = Load([]byte(`archaeological_sites={sites={0={events={{scope={anonymous_value}}}}}}`))
	if err != nil || len(gs.ArchaeologySites.DecodeErrors) != 1 {
		t.Fatal("unsupported event scope silently discarded")
	}
}

func TestArchaeologyLocalCatalogCoverage(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for local read-only archaeology coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil {
		t.Fatal("save catalog read failed")
	}
	records, unsupported, planetCandidates, fleetCandidates, knownLastCountries, visibleRefs, knownVisibleRefs, completionRows, eventRows, logRows := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	var fleetOrders, reciprocalFleetOrders int
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("save index %d whole-save parsing failed", index)
		}
		if gs.ArchaeologySites.TableIssue {
			unsupported++
		}
		unsupported += len(gs.ArchaeologySites.DecodeErrors)
		records += len(gs.ArchaeologySites.Items)
		for id, fleet := range gs.Fleet {
			if ref := fleet.CurrentOrderRefs.ArchaeologicalSite.Value; ref != nil {
				fleetOrders++
				if site, ok := gs.ArchaeologySites.Items[*ref]; ok && site.ExcavatorFleet != nil && *site.ExcavatorFleet == id {
					reciprocalFleetOrders++
				}
			}
		}
		for _, site := range gs.ArchaeologySites.Items {
			if site.Location.ID != nil {
				if _, ok := gs.Planets.Planet[*site.Location.ID]; ok {
					planetCandidates++
				}
			}
			if site.ExcavatorFleet != nil {
				if _, ok := gs.Fleet[*site.ExcavatorFleet]; ok {
					fleetCandidates++
				}
			}
			if site.LastExcavatorCountry != nil {
				if _, ok := gs.Country[*site.LastExcavatorCountry]; ok {
					knownLastCountries++
				}
			}
			for _, id := range site.VisibleTo {
				visibleRefs++
				if _, ok := gs.Country[id]; ok {
					knownVisibleRefs++
				}
			}
			completionRows += len(site.Completed)
			eventRows += len(site.Events)
			logRows += len(site.Log)
		}
	}
	t.Logf("local archaeology coverage: saves=%d records=%d unsupported=%d possible_planet_matches=%d possible_fleet_matches=%d known_last_country_refs=%d visible_country_refs=%d known_visible_country_refs=%d completed_rows=%d event_rows=%d log_rows=%d", len(files), records, unsupported, planetCandidates, fleetCandidates, knownLastCountries, visibleRefs, knownVisibleRefs, completionRows, eventRows, logRows)
	t.Logf("archaeology fleet order coverage: orders=%d reciprocal=%d", fleetOrders, reciprocalFleetOrders)
	if unsupported > 0 {
		t.Fatal("local archaeology shape needs additional support")
	}
}

func TestArchaeologyNestedMixedScopeRequiresRawFallback(t *testing.T) {
	gs, err := Load([]byte(`archaeological_sites={sites={0={visible_to={0} events={{scope={nested={known=one anonymous_value}}}}}}}`))
	if err != nil {
		t.Fatal("optional mixed scope broke whole save")
	}
	if len(gs.ArchaeologySites.DecodeErrors) != 1 {
		t.Fatal("nested mixed scope silently treated as fully preserved")
	}
}
