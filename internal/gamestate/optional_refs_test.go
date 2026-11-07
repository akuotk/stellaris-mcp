package gamestate

import "testing"

func TestOptionalSystemAndShipReferences(t *testing.T) {
	gs, err := Load([]byte(`galactic_object={0={starbases={0 2 2}} 1={starbases={}} 2={starbases={bad=yes anonymous}} 3={}} ships={0={coordinate={origin=0}} 1={coordinate={origin={invalid=yes}}} 2={}}`))
	if err != nil {
		t.Fatal(err)
	}
	s := gs.Systems[0].StarbaseRefs
	if !s.Recorded || s.Unsupported || len(s.Values) != 3 || s.Values[0] != 0 || s.Values[1] != 2 || s.Values[2] != 2 {
		t.Fatal("reference sequence lost")
	}
	if !gs.Systems[1].StarbaseRefs.Recorded || gs.Systems[1].StarbaseRefs.Unsupported || len(gs.Systems[1].StarbaseRefs.Values) != 0 || !gs.Systems[2].StarbaseRefs.Unsupported || gs.Systems[3].StarbaseRefs.Recorded {
		t.Fatal("empty, unknown, and absent lists conflated")
	}
	c := gs.Ships[0].Coordinate
	if !c.Recorded || c.Unsupported || c.Origin == nil || *c.Origin != 0 {
		t.Fatal("coordinate zero lost")
	}
	if !gs.Ships[1].Coordinate.Unsupported || gs.Ships[2].Coordinate.Recorded {
		t.Fatal("unknown and absent coordinate conflated")
	}
}
