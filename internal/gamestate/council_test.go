package gamestate

import (
	"os"
	"reflect"
	"testing"
)

func TestCouncilSavedOrderZerosAndGovernmentIsolation(t *testing.T) {
	gs, err := Load([]byte(`country={0={government={type=synthetic authority=example council_positions={0 1} picked_council_types={first second}
	 council_agenda=agenda_a council_agenda_progress=0.0000001 unlocked_civic_council_slots=0
	 council_agenda_cooldowns={council_agenda=agenda_a start_date=0}
	 council_agenda_cooldowns={council_agenda=agenda_b start_date=0 start_date=1}
	 council_agenda_cooldowns={council_agenda=agenda_c}
	 }} 1={government={type=old council_positions={0}} government={type=ambiguous council_positions={1}}}
	 2={government={type=kept council_agenda=first council_agenda=second council_agenda_progress={bad=yes}}} 3={}}
	 council_positions={council_positions={0={country=0 leader=0 type=first} 1={country=0} 9=none}}
	 leaders={0={country=0 council_location={type=council_position id=0 position=0}} 1={}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	v := c.CouncilGovernment
	if !v.Recorded || v.Unsupported || v.Agenda.Value != "agenda_a" || v.Progress.Value != "0.0000001" || v.UnlockedSlots.Value != "0" || len(v.Cooldowns) != 3 || v.Cooldowns[0].StartDate.Value != "0" || !v.Cooldowns[1].Unsupported || v.Cooldowns[2].Agenda.Value != "agenda_c" || v.Cooldowns[2].StartDate.Recorded {
		t.Fatal("raw values, repeated cooldown order or missing fields lost")
	}
	if c.Government.Type != "synthetic" || c.Government.Authority != "example" || !reflect.DeepEqual(c.Government.CouncilPositions, []int{0, 1}) || !reflect.DeepEqual(c.Government.PickedCouncilTypes, []string{"first", "second"}) {
		t.Fatal("existing government decoding changed")
	}
	if !gs.Country[1].CouncilGovernment.Unsupported || gs.Country[1].Government.Type != "old" || !gs.Country[2].CouncilGovernment.Agenda.Unsupported || !gs.Country[2].CouncilGovernment.Progress.Unsupported || gs.Country[2].Government.Type != "kept" || gs.Country[3].CouncilGovernment.Recorded {
		t.Fatal("government or scalar ambiguity accepted or unaffected fields lost")
	}
	pos := gs.CouncilPositions.Items[0]
	if !gs.CouncilPositions.Recorded || len(gs.CouncilPositions.Items) != 2 || pos.Country == nil || *pos.Country != 0 || pos.Leader == nil || *pos.Leader != 0 || gs.CouncilPositions.Items[1].Leader != nil || gs.Leaders[0].CountryID == nil || *gs.Leaders[0].CountryID != 0 || gs.Leaders[1].CountryID != nil {
		t.Fatal("optional zero and missing position/leader references conflated")
	}
}

func TestCouncilPositionMalformedAndDuplicateRecordsStayIsolated(t *testing.T) {
	gs, err := Load([]byte(`date=2201.01.01 council_positions={council_positions={
	 0={country=0 country=1} 1={leader=0 leader=1} 2={type=first type=second}
	 3={country=invalid} 4=none 4={country=0} 5={country=0} 5=none
	 6={country=0 leader=0} invalid={}
	 }}`))
	if err != nil || gs.Date != "2201.01.01" || len(gs.CouncilPositions.Items) != 1 || len(gs.CouncilPositions.DecodeErrors) != 6 || !gs.CouncilPositions.TableIssue {
		t.Fatal("malformed or duplicate records affected unrelated data or remained usable")
	}
	for _, raw := range []string{`council_positions=none`, `council_positions={council_positions=none}`, `council_positions={unexpected={}}`, `council_positions={council_positions={} council_positions={}}`} {
		gs, err := Load([]byte(raw))
		if err != nil || !gs.CouncilPositions.Recorded || !gs.CouncilPositions.TableIssue || len(gs.CouncilPositions.Items) != 0 {
			t.Fatal("invalid table wrapper silently accepted")
		}
	}
}

// This opt-in survey logs aggregate counts, including reciprocal reference checks.
func TestCouncilLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local council coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	decoded, positions, agendas, progresses, slots, cooldowns, unsupported := 0, 0, 0, 0, 0, 0, 0
	govRefs, ownerMatches, ownerDifferences, missingPositions, alignedTypes, differingTypes, listLengthDifferences := 0, 0, 0, 0, 0, 0, 0
	leaderRefs, reciprocal, missingLeader, reverseIncomplete, reverseMismatch, leaderCountryMismatch, reverseWithoutPosition := 0, 0, 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("save index %d whole-save decoding failed", index)
		}
		decoded++
		positions += len(gs.CouncilPositions.Items)
		unsupported += len(gs.CouncilPositions.DecodeErrors)
		if gs.CouncilPositions.TableIssue {
			unsupported++
		}
		for countryID, c := range gs.Country {
			v := c.CouncilGovernment
			if v.Unsupported || v.Agenda.Unsupported || v.Progress.Unsupported || v.UnlockedSlots.Unsupported {
				unsupported++
			}
			if v.Agenda.Recorded {
				agendas++
			}
			if v.Progress.Recorded {
				progresses++
			}
			if v.UnlockedSlots.Recorded {
				slots++
			}
			cooldowns += len(v.Cooldowns)
			for _, record := range v.Cooldowns {
				if record.Unsupported || record.Agenda.Unsupported || record.StartDate.Unsupported {
					unsupported++
				}
			}
			refs, types := c.Government.CouncilPositions, c.Government.PickedCouncilTypes
			if len(refs) != len(types) {
				listLengthDifferences++
			}
			for i, id := range refs {
				govRefs++
				position, ok := gs.CouncilPositions.Items[id]
				if !ok {
					missingPositions++
					continue
				}
				if position.Country != nil && *position.Country == countryID {
					ownerMatches++
				} else {
					ownerDifferences++
				}
				if len(refs) == len(types) && position.Type != nil {
					if *position.Type == types[i] {
						alignedTypes++
					} else {
						differingTypes++
					}
				}
			}
		}
		for id, pos := range gs.CouncilPositions.Items {
			if pos.Leader == nil {
				continue
			}
			leaderRefs++
			ldr, ok := gs.Leaders[*pos.Leader]
			if !ok {
				missingLeader++
				continue
			}
			loc := ldr.CouncilLocation
			switch {
			case loc.Type == "" || loc.IDRaw == nil:
				reverseIncomplete++
			case loc.Type != "council_position" || *loc.IDRaw != id:
				reverseMismatch++
			default:
				reciprocal++
			}
			if pos.Country != nil && ldr.CountryID != nil && *pos.Country != *ldr.CountryID {
				leaderCountryMismatch++
			}
		}
		for id, ldr := range gs.Leaders {
			loc := ldr.CouncilLocation
			if loc.Type != "council_position" || loc.IDRaw == nil {
				continue
			}
			pos, ok := gs.CouncilPositions.Items[*loc.IDRaw]
			if !ok || pos.Leader == nil || *pos.Leader != id {
				reverseWithoutPosition++
			}
		}
	}
	t.Logf("local council coverage: saves=%d decoded=%d positions=%d gov_position_refs=%d country_matches=%d country_differences=%d missing_positions=%d same_index_type_matches=%d same_index_type_differences=%d list_length_differences=%d agenda_fields=%d progress_fields=%d slot_fields=%d cooldown_records=%d leader_refs=%d reciprocal=%d missing_leaders=%d incomplete_leader_reverse=%d mismatched_leader_reverse=%d leader_country_differences=%d leader_reverse_without_matching_position=%d unsupported_shapes=%d", len(files), decoded, positions, govRefs, ownerMatches, ownerDifferences, missingPositions, alignedTypes, differingTypes, listLengthDifferences, agendas, progresses, slots, cooldowns, leaderRefs, reciprocal, missingLeader, reverseIncomplete, reverseMismatch, leaderCountryMismatch, reverseWithoutPosition, unsupported)
	if unsupported > 0 {
		t.Fatal("local council shapes need additional support")
	}
}
