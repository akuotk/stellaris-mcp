package tools

import (
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
	if _, _, err := getCountry(gs, rawRequest(nil)); err == nil {
		t.Fatal("ambiguous multiplayer country silently selected")
	}
	gs.Player = nil
	if _, _, err := getCountry(gs, rawRequest(nil)); err == nil {
		t.Fatal("missing player country silently selected")
	}
}
