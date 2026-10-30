package gamestate

import "testing"

func TestLoadWarDatabase(t *testing.T) {
	gs, err := Load([]byte(`war={ 7=none 9={ name={ key="Test War" } start_date="2263.07.09" } }`))
	if err != nil {
		t.Fatal(err)
	}
	if len(gs.War) != 1 || gs.War[9].StartDate != "2263.07.09" || gs.War[9].Name.Key != "Test War" {
		t.Fatalf("unexpected wars: %#v", gs.War)
	}
}
