package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestCountrySelectionUsesPlayerOrExplicitID(t *testing.T) {
	gs, err := gamestate.Load([]byte(`player={name="synthetic" country=7} country={0={adjective={key="First"}} 7={adjective={key="Player"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := getCountry(gs, rawRequest(nil))
	if err != nil || id != 7 {
		t.Fatalf("single-player default: %d %v", id, err)
	}
	id, _, err = getCountry(gs, rawRequest(map[string]any{"country_id": 0}))
	if err != nil || id != 0 {
		t.Fatalf("explicit zero: %d %v", id, err)
	}
	for _, value := range []any{-1, 1.5, "7"} {
		if _, _, err := getCountry(gs, rawRequest(map[string]any{"country_id": value})); err == nil {
			t.Fatalf("accepted invalid ID: %v", value)
		}
	}
	gs.Player = append(gs.Player, gs.Player[0])
	if id, _, err := getCountry(gs, rawRequest(nil)); err != nil || id != 7 {
		t.Fatal("matching repeated player records did not select the country")
	}
	other := 0
	gs.Player = append(gs.Player, gamestate.Player{CountryID: &other})
	if _, _, err := getCountry(gs, rawRequest(nil)); err == nil {
		t.Fatal("different saved player countries silently selected")
	}
	gs.Player = nil
	if _, _, err := getCountry(gs, rawRequest(nil)); err == nil {
		t.Fatal("missing player country silently selected")
	}
}

func TestSavedCountrySelectionAgreesAcrossReaders(t *testing.T) {
	const countries = ` country={0={tech_status={technology=tech_fixture}} 1={}}`
	for _, fixture := range []struct {
		name, source   string
		auto, explicit bool
	}{
		{"matching list", `player={{country=0} {country="0"}}` + countries, true, true},
		{"matching repeated roots", `player={country=0} player={country=0}` + countries, true, true},
		{"matching root and list", `player={country=0} player={{country=0} {country=0}}` + countries, true, true},
		{"different IDs", `player={{country=0} {country=1}}` + countries, false, true},
		{"missing ID", `player={{country=0} {name=fixture}}` + countries, false, true},
		{"missing player", countries, false, true},
		{"nonnumeric ID", `player={{country=0} {country=invalid}}` + countries, false, true},
		{"negative ID", `player={{country=0} {country=-1}}` + countries, false, true},
		{"duplicate ID field", `player={{country=0} {country=0 country=0}}` + countries, false, true},
		{"unsupported ID shape", `player={{country=0} {country={0}}}` + countries, false, true},
		{"mixed player block", `player={country=0 unexpected}` + countries, false, true},
		{"mixed player item", `player={{country=0} {country=0 unexpected}}` + countries, false, true},
		{"scalar player item", `player={{country=0} unexpected}` + countries, false, true},
		{"empty and valid roots", `player={} player={country=0}` + countries, false, true},
		{"duplicate country ID", `player={country=0} country={0={} 0={}}`, false, false},
		{"duplicate numeric country key", `player={country=0} country={0={} 00={}}`, false, false},
		{"duplicate country table", `player={country=0} country={0={}} country={0={}}`, false, false},
		{"missing country", `player={country=0} country={1={}}`, false, false},
		{"country tombstone", `player={country=0} country={0=none}`, false, false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture.source)}})
			gs, err := gamestate.Load([]byte(fixture.source))
			if err != nil {
				t.Fatal(err)
			}
			for _, explicit := range []bool{false, true} {
				args := map[string]any{}
				want := fixture.auto
				if explicit {
					args["country_id"] = 0
					want = fixture.explicit
				}
				id, _, err := getCountry(gs, rawRequest(args))
				if (err == nil) != want || (want && id != 0) {
					t.Fatalf("unexpected typed selection for explicit=%t, id=%d err=%v", explicit, id, err)
				}
				completed := completedTechnologyCall(t, args)
				if completed.IsError == want || (want && !strings.Contains(resultText(completed), "country_id=0")) {
					t.Fatalf("unexpected completed selection for explicit=%t", explicit)
				}
				args["include_save_status"], args["technology_id"] = true, "tech_fixture"
				definition := technologyResult(t, nil, args)
				if definition.IsError == want || (want && !strings.Contains(resultText(definition), "Saved research key matches")) {
					t.Fatalf("unexpected definition selection for explicit=%t", explicit)
				}
			}
		})
	}
}
