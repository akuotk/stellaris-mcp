package gamestate

import (
	"os"
	"testing"
)

func TestFirstContactValuesEventsAndHistory(t *testing.T) {
	gs, err := Load([]byte(`first_contacts={contacts={0={owner=0 country=0 name={key="Synthetic contact"} location=0 leader=0 date="2201.01.01" last_roll=0 days_left=0 difficulty=0 clues=0 stage="synthetic_stage" status="synthetic_status"
 event={event_id=0 expired=0 scope={opaque=0 repeated=one repeated=two} owner=0 fleet=0 effect=0 picture="synthetic_picture" index=0 player_event=0}
 events={{event_id="first_event" scope={opaque=0}} {event_id="second_event"}}
 completed={{date="2201.01.01" stage="first_stage"} {date="2201.01.02" stage="second_stage"}}
 flags={example=0}}
 1={} 8=none}}`))
	if err != nil {
		t.Fatal(err)
	}
	store := gs.FirstContacts
	if store.TableIssue || len(store.DecodeErrors) > 0 || len(store.Items) != 2 {
		t.Fatal("supported contacts rejected")
	}
	c := store.Items[0]
	for _, v := range []*int{c.Owner, c.Country, c.Location, c.Leader, c.Event.Owner, c.Event.Fleet, c.Event.Index} {
		if v == nil || *v != 0 {
			t.Fatal("zero reference lost")
		}
	}
	for _, v := range []*float64{c.LastRoll, c.DaysLeft, c.Difficulty, c.Clues} {
		if v == nil || *v != 0 {
			t.Fatal("zero statistic lost")
		}
	}
	if c.Event.Effect == nil || *c.Event.Effect != "0" || c.Event.Scope == nil || len(c.Event.Scope.Value.Object.Pairs) != 3 || len(c.Events) != 2 || *c.Events[1].EventID != "second_event" || len(c.Completed) != 2 || *c.Completed[1].Stage != "second_stage" {
		t.Fatal("event fields or history order lost")
	}
	missing := store.Items[1]
	if missing.Owner != nil || missing.Leader != nil || missing.Clues != nil || missing.Status != nil || missing.Event != nil || missing.Events != nil || missing.Completed != nil {
		t.Fatal("missing fields became recorded values")
	}
}

func TestFirstContactUnsupportedAndDuplicateRecordsIsolated(t *testing.T) {
	gs, err := Load([]byte(`date="2201.01.01" first_contacts={contacts={0={owner=0 days_left={unexpected=shape}} 1={} 1={} invalid_key={}}}`))
	if err != nil {
		t.Fatal("optional unsupported contact broke save")
	}
	if gs.Date != "2201.01.01" || !gs.FirstContacts.TableIssue || len(gs.FirstContacts.DecodeErrors) != 2 {
		t.Fatal("unsupported/duplicate records not flagged")
	}
	gs, err = Load([]byte(`first_contacts={contacts={0={owner=0 event={scope={nested={key=one anonymous}}}}}}`))
	if err != nil || len(gs.FirstContacts.DecodeErrors) != 1 {
		t.Fatal("unknown event scope not isolated")
	}
}

func TestFirstContactsLocalCatalogCoverage(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for local read-only first-contact coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil {
		t.Fatal("save catalog read failed")
	}
	records, unsupported, knownOwner, knownCountry, possibleSystems, possibleLeaders, reverseLeaders, eventRecords, eventHistory, completionHistory := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("save index %d whole-save parsing failed", index)
		}
		if gs.FirstContacts.TableIssue {
			unsupported++
		}
		unsupported += len(gs.FirstContacts.DecodeErrors)
		records += len(gs.FirstContacts.Items)
		for id, c := range gs.FirstContacts.Items {
			if c.Owner != nil {
				if _, ok := gs.Country[*c.Owner]; ok {
					knownOwner++
				}
			}
			if c.Country != nil {
				if _, ok := gs.Country[*c.Country]; ok {
					knownCountry++
				}
			}
			if c.Location != nil {
				if _, ok := gs.Systems[*c.Location]; ok {
					possibleSystems++
				}
			}
			if c.Leader != nil {
				if leader, ok := gs.Leaders[*c.Leader]; ok {
					possibleLeaders++
					if leader.Location.Type == "first_contact" && leader.Location.IDRaw != nil && *leader.Location.IDRaw == id {
						reverseLeaders++
					}
				}
			}
			if c.Event != nil {
				eventRecords++
			}
			eventHistory += len(c.Events)
			completionHistory += len(c.Completed)
		}
	}
	t.Logf("local first-contact coverage: saves=%d records=%d unsupported=%d known_owner_ids=%d known_country_ids=%d possible_system_matches=%d possible_leader_matches=%d explicit_reverse_leader_refs=%d event_records=%d event_history=%d completed_history=%d", len(files), records, unsupported, knownOwner, knownCountry, possibleSystems, possibleLeaders, reverseLeaders, eventRecords, eventHistory, completionHistory)
	if unsupported > 0 {
		t.Fatal("local first-contact shape needs additional support")
	}
}
