package gamestate

import "testing"

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
