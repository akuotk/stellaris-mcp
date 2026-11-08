package gamestate

import (
	"os"
	"testing"
)

func TestRelationRowsPreserveOrderAndIsolateUnknownFields(t *testing.T) {
	gs, err := Load([]byte(`date=2201.01.01 country={
	0={relations_manager={relation={{owner=0 country=0 contact=yes trust=-0.00 relation_current=00.125 modifier={modifier=first value=0} modifier={modifier=second value=1}} bad}
	relation={owner=0 owner=1 country=0 country=2 contact=yes contact=no trust=0 trust=1 relation_current=0 relation_current=1}
	relation={owner={unknown=yes} country=3 contact={unknown=yes}}}}
	1={relations_manager={} relations_manager={relation={country=0}}}
	2={relations_manager=bad}
	3={}}
	`))
	if err != nil {
		t.Fatal(err)
	}
	relations := gs.Country[0].RelationsManager
	if gs.Date != "2201.01.01" || !relations.Recorded || len(relations.Relation) != 4 {
		t.Fatal("ordered relation rows or neighboring data lost")
	}
	first, invalid, duplicate, optional := relations.Relation[0], relations.Relation[1], relations.Relation[2], relations.Relation[3]
	if first.OwnerRef.Value == nil || *first.OwnerRef.Value != 0 || first.CountryID == nil || *first.CountryID != 0 ||
		first.ContactRaw == nil || *first.ContactRaw != "yes" || first.Trust.Value != "-0.00" ||
		first.RelationCurrent.Value != "00.125" || len(first.SavedDetails) != 2 {
		t.Fatal("saved zero, lexical values or repeated modifiers lost")
	}
	if !invalid.Unsupported || !duplicate.OwnerDuplicate || !duplicate.CountryDuplicate || duplicate.CountryID != nil ||
		duplicate.ContactRaw != nil || !duplicate.TrustDuplicate || !duplicate.CurrentDuplicate ||
		!optional.OwnerRef.Unsupported || !optional.Contact.Unsupported || optional.CountryID == nil || *optional.CountryID != 3 {
		t.Fatal("ambiguous or unsupported relation fields became authoritative")
	}
	if !gs.Country[1].RelationsManager.Duplicate || len(gs.Country[1].RelationsManager.Relation) != 0 ||
		!gs.Country[2].RelationsManager.Unsupported || gs.Country[3].RelationsManager.Recorded {
		t.Fatal("duplicate, unsupported and missing managers conflated")
	}
}

func TestRelationsLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only bilateral relation coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	type direction struct{ from, to int }
	var managers, rows, ownerMatches, targetMatches, reverse, truce, trust, opinion, modifiers, actionDateValues, unsupported int
	for _, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatal("whole-save decoding failed")
		}
		directions := map[direction]int{}
		for owner, c := range gs.Country {
			m := c.RelationsManager
			if m.Recorded {
				managers++
			}
			if m.Duplicate || m.Unsupported {
				unsupported++
			}
			for _, r := range m.Relation {
				rows++
				if r.Unsupported || r.OwnerDuplicate || r.CountryDuplicate || r.ContactDuplicate || r.TrustDuplicate || r.CurrentDuplicate ||
					r.OwnerRef.Unsupported || r.CountryRef.Unsupported || r.Contact.Unsupported || r.Trust.Unsupported || r.RelationCurrent.Unsupported {
					unsupported++
				}
				ownerMatchesRecord := r.OwnerRef.Value != nil && *r.OwnerRef.Value == owner
				if ownerMatchesRecord {
					ownerMatches++
				}
				if r.CountryID != nil {
					if _, ok := gs.Country[*r.CountryID]; ok {
						targetMatches++
					}
					if ownerMatchesRecord {
						directions[direction{owner, *r.CountryID}]++
					}
				}
				if r.Trust.Recorded {
					trust++
				}
				if r.RelationCurrent.Recorded {
					opinion++
				}
				for _, pair := range r.SavedDetails {
					switch pair.Key {
					case "truce":
						truce++
					case "modifier":
						modifiers++
					case "diplo_action_dates":
						actionDateValues += len(pair.Value.List)
					}
				}
			}
		}
		for d, count := range directions {
			if count == 1 && directions[direction{d.to, d.from}] == 1 {
				reverse++
			}
		}
	}
	t.Logf("local relation coverage: saves=%d managers=%d rows=%d owner_matches=%d target_matches=%d directions_with_unique_reverse=%d truce_fields=%d trust_fields=%d opinion_fields=%d opinion_modifier_rows=%d diplo_action_date_values=%d unsupported=%d", len(files), managers, rows, ownerMatches, targetMatches, reverse, truce, trust, opinion, modifiers, actionDateValues, unsupported)
	if unsupported != 0 {
		t.Fatal("local relation shapes need additional support")
	}
}
