package gamestate

import (
	"os"
	"slices"
	"testing"
)

func TestAssetsPreserveCountryZeroAndAbsentOwner(t *testing.T) {
	gs, err := Load([]byte(`agreements={ agreements={ 1={owner=0 target=2 active_status=active term_data={agreement_preset="preset"}} } }
		situations={situations={3={country=0 type="example" progress=12.5 last_month_progress=0.5}}}
		megastructures={4={type="gateway" owner=0 coordinate={origin=7}} 5={type="unknown"}}`))
	if err != nil {
		t.Fatal(err)
	}
	a := gs.Agreements.Items[1]
	s := gs.Situations.Items[3]
	m := gs.Megastructures[4]
	if a.Owner == nil || *a.Owner != 0 || a.Target == nil || *a.Target != 2 || a.Terms.Preset != "preset" || s.Country == nil || *s.Country != 0 || s.Progress != 12.5 || m.Owner == nil || *m.Owner != 0 || m.Coordinate.Origin == nil || *m.Coordinate.Origin != 7 || gs.Megastructures[5].Owner != nil {
		t.Fatalf("asset data incorrect: %+v %+v %+v", a, s, m)
	}
}

func TestAgreementUnsupportedDataStaysIsolated(t *testing.T) {
	for _, input := range []string{
		`agreements=invalid`,
		`agreements={agreements=invalid}`,
		`agreements={agreements={} agreements={0={owner=0}}}`,
		`agreements={agreements={0={owner=0}}} agreements={}`,
	} {
		gs, err := Load([]byte(`date=2201.01.01 ` + input))
		if err != nil || gs.Date != "2201.01.01" || !gs.Agreements.TableIssue || len(gs.Agreements.Items) != 0 {
			t.Fatalf("agreement table issue affected unrelated data or was not reported: %s", input)
		}
	}
	gs, err := Load([]byte(`country={0={overlord=bad subjects={bad} subject_date={unknown=yes} capital=1}}
	agreements={agreements={0={owner=0 target=0}
	1={owner=0 target=0} 1=none
	2={owner=0 term_data={forced_initial_loyalty=0 forced_initial_loyalty=1}}
	3={owner=0 term_data={resource_terms={{key=a key=b value=0}}}}
	4={owner=0 subject_specialization={subject_conversion_process={ignore=no ignore=yes}}}
	5=none}}`))
	if err != nil || !gs.Agreements.Recorded || len(gs.Agreements.Items) != 1 || len(gs.Agreements.DecodeErrors) != 4 {
		t.Fatal("unsupported or duplicate agreement was accepted or blocked unrelated records")
	}
	c := gs.Country[0]
	if !c.OverlordRef.Unsupported || c.OverlordRef.Value != nil || !c.SubjectRefs.Unsupported || len(c.SubjectRefs.Values) != 0 || !c.SubjectDate.Unsupported || c.Capital != 1 {
		t.Fatal("unsupported country fields acquired default values or affected unrelated data")
	}
}

func TestAgreementsLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local agreement coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	var tables, records, bothLinks, loyalty, cooldown, ignore, unsupported int
	for _, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatal("whole-save decoding failed")
		}
		if gs.Agreements.Recorded {
			tables++
		}
		if gs.Agreements.TableIssue {
			unsupported++
		}
		unsupported += len(gs.Agreements.DecodeErrors)
		for _, a := range gs.Agreements.Items {
			records++
			if a.Terms.ForcedInitialLoyalty != nil {
				loyalty++
			}
			if a.Terms.FirstRenegotiationHasCooldown != nil {
				cooldown++
			}
			if a.Specialization.Conversion.Ignore != nil {
				ignore++
			}
			if a.Owner == nil || a.Target == nil {
				continue
			}
			owner := gs.Country[*a.Owner]
			target := gs.Country[*a.Target]
			if slices.Contains(owner.SubjectRefs.Values, *a.Target) && target.OverlordRef.Value != nil && *target.OverlordRef.Value == *a.Owner {
				bothLinks++
			}
		}
	}
	t.Logf("local agreement coverage: saves=%d tables=%d records=%d both_country_links_match=%d forced_initial_loyalty=%d has_cooldown_on_first_renegotiation=%d conversion_ignore=%d unsupported_table_or_records=%d", len(files), tables, records, bothLinks, loyalty, cooldown, ignore, unsupported)
	if unsupported != 0 {
		t.Fatal("local agreement shapes need additional support")
	}
}
