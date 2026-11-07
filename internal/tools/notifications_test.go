package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
)

func TestNotificationWarRecordsDistinguishMissingAndEnded(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`player={{name="Player"} {name="Other" country=0}} country={0={adjective={key="Synthetic"} relations_manager={relation={country=0 contact=yes} relation={country=0 contact=maybe}}}} war={0={name={key="Unfinished"} start_date="" attackers={{country=0} {country=99} {}}} 1={name={key="Dated"} start_date="2200.01.01" end_date="2201.01.01" defenders={}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderNotifications(gs, &c))
	for _, want := range []string{"Player (country reference: not recorded)", "Other (country reference: 0)", "ID 0: Unfinished (saved start=\"\", saved end=not recorded)", "ID 1: Dated (saved start=\"2200.01.01\", saved end=\"2201.01.01\")", "Country 99 (record unavailable)", "country reference not recorded", "Defenders: empty", "Saved contact=yes relations: 1", "Contact status not established: 1", "Victory rank: not recorded"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "Active Wars:") || strings.Contains(text, "No active wars.") {
		t.Fatal("inferred war activity")
	}
}
