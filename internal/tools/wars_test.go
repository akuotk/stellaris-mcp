package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestWarsSavedDetailsPaginationAndAmbiguity(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	data := `country={0={adjective={key="First country"}} 1={adjective={key="Second country"}}} galactic_object={0={name={key="First system"}}} planets={planet={0={name={key="First planet"}}}} colony={0={}}
	war={
	 0={name={key="Synthetic war"} start_date="" attacker_war_goal={type=example actor=0 target=1} defender_war_exhaustion=0.00000001 attacker_force_peace=no
	 attackers={{country=0 caller=1 call_type=0} {country=0} {country=0 country=1} {}} defenders={{country=1}}
	 battles={
	 {attackers={0 0 99} defenders={1} date=2201.01.01 type=0 system=0 planet=0 colony=0 attacker_losses=0 defender_losses=123456789.1234567 attacker_victory=no}
	 {attackers={0} attackers={1} defenders={unexpected} system=0 system=1 planet=4294967295 payload={saved=yes anonymous}}
	 ` + strings.Repeat("{date=2201.01.02} ", 18) + `
	 {date=2201.01.03 type=3}
	 }}
	 1=none 01={name={key="Duplicate war"}}
	 2={start_date=old start_date=new battles={} battles={{type=0}}}
	 3={battles={}}
	 4=unexpected
	}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(data)}})
	s := server.NewMCPServer("test", "test")
	Register(s, saveDir, loc)
	if _, ok := s.ListTools()["get_wars"]; !ok {
		t.Fatal("war tool missing from normal registration")
	}
	var pageContext string
	for _, tc := range []struct {
		args  map[string]any
		want  []string
		avoid string
	}{
		{nil, []string{"ID 0: Synthetic war (saved start=\"\", saved end=not recorded)", "ID 1: war record unsupported or duplicate", "ID 2:", "saved start=unsupported or duplicate", "Battles: unsupported or duplicate", "Battles: empty", "ID 4: war record unsupported or duplicate", "country reference unsupported or duplicate", "country reference not recorded"}, "Duplicate war"},
		{map[string]any{"war_id": 0}, []string{"offset=0 returned=20 total=21 next_battle_offset=20", "Battle[19] (saved order)", `key="defender_war_exhaustion": "0.00000001"`, `key="attacker_force_peace": "no"`, `key="actor": "0"`, `key="caller": "1"`, `key="call_type": "0"`, `key="defender_losses": "123456789.1234567"`, "First country (ID 0) [duplicate saved reference]", "Country 99 (record unavailable)", "system ID candidate 0 (First system)", "planet ID candidate 0 (First planet)", "colony ID candidate 0 (saved colony record exists)", "system ID candidate unresolved", byIntWidth("planet ID candidate 4294967295 (record unavailable)", "planet ID candidate unresolved (unsupported or duplicate)"), "attackers country ID candidates unresolved", `key="payload": mixed saved block`}, "Battle[20]"},
		{map[string]any{"war_id": 0, "battle_offset": 20}, []string{"offset=20 returned=1 total=21", "Battle[20] (saved order)", `key="date": "2201.01.03"`}, "Battle[19]"},
		{map[string]any{"war_id": 2}, []string{"War battles unsupported or duplicate", `key="start_date" [duplicate saved key]: "new"`}, "Battle[0]"},
	} {
		if _, laterPage := tc.args["battle_offset"]; laterPage {
			tc.args["page_context"] = pageContext
		}
		result, err := handleWars(context.Background(), rawRequest(tc.args))
		if err != nil || result.IsError {
			t.Fatal("normal wars handler failed")
		}
		text := resultText(result)
		if tc.args["war_id"] == 0 && tc.args["battle_offset"] == nil {
			_, rest, found := strings.Cut(text, "\npage_context=")
			pageContext, _, _ = strings.Cut(rest, "\n")
			if !found || pageContext == "" {
				t.Fatal("first battle page did not supply a continuation context")
			}
		}
		for _, want := range tc.want {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in %s", want, text)
			}
		}
		if strings.Contains(text, tc.avoid) {
			t.Fatalf("unexpected %q in %s", tc.avoid, text)
		}
	}
	for _, args := range []map[string]any{
		{"battle_offset": 0}, {"war_id": 0, "battle_offset": -1}, {"war_id": 0.5},
		{"war_id": 0, "battle_offset": 22, "page_context": pageContext},
		{"war_id": 0, "battle_offset": 20},
		{"war_id": 2, "battle_offset": 20, "page_context": pageContext},
	} {
		result, err := handleWars(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Fatal("invalid war selection or page offset accepted")
		}
	}
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(data + " date=2202.01.01")}})
	result, err := handleWars(context.Background(), rawRequest(map[string]any{"war_id": 0, "battle_offset": 20, "page_context": pageContext}))
	if err != nil || !result.IsError || !strings.Contains(resultText(result), "page_context does not match") {
		t.Fatal("changed archive accepted a previous battle page context")
	}
}

func TestWarTablesDistinguishMissingAndAmbiguous(t *testing.T) {
	for _, tc := range []struct{ table, want string }{
		{"", "not recorded"},
		{"war={}", "empty"},
		{"war={0={}} war={1={}}", "War table decoding incomplete or duplicate"},
		{"war={anonymous field=zero}", "War table decoding incomplete or duplicate"},
	} {
		gs, err := gamestate.Load([]byte("date=2201.01.01 " + tc.table))
		if err != nil || gs.Date != "2201.01.01" {
			t.Fatal("optional war table prevented unrelated save decoding")
		}
		text := resultText(renderWars(gs, nil, 0, "", ""))
		if !strings.Contains(text, tc.want) || strings.Contains(text, "ID 0:") {
			t.Fatalf("table shape produced a misleading war record: %s", text)
		}
	}
}
