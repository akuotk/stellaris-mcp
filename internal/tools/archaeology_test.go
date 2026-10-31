package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestArchaeologyScopeCandidatesAndRawHistory(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Synthetic nation"}}} planets={planet={0={name={key="Candidate planet"}}}} fleet={0={name={key="Candidate fleet"}}}
 archaeological_sites={sites={
  0={type="synthetic_site" visible_to={0} location={type=0 id=0} excavator_fleet=0 index=0 days_left=0 clues=0 last_roll=0 difficulty=0 locked=no
    completed={{country=0 date="2201.01.01"} {country=0 date="2201.01.02"}}
    events={{event_id="synthetic_event" expired=0 owner=0 fleet=0 index=0 player_event=0 scope={opaque_field=0}}}
    log={{title="first" roll=0 bonus=0 total=0 clues=0 difficulty=0} {title="second"}}}
  1={last_excavator_country=0 excavator_fleet=999}
  2={completed={{country=0}}}
  3={visible_to={1}}
 }}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	summary := resultText(renderArchaeology(gs, 0, &c, nil))
	for _, want := range []string{"Site 0:", "Site 1:", "Site 2:", "Possible planet ID match: Candidate planet", "Possible fleet ID match: Candidate fleet", "no reverse site link validated", "Excavator fleet reference unresolved"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in %s", want, summary)
		}
	}
	if strings.Contains(summary, "Site 3:") {
		t.Fatal("unreferenced country site included")
	}
	id := 0
	detail := resultText(renderArchaeology(gs, 0, &c, &id))
	for _, want := range []string{"index=0 days_left=0", "clues=0 last_roll=0 difficulty=0", "locked: false", "global site completion not inferred", "completed entry 0", "completed entry 1", "event entry 0: event_id=synthetic_event expired=0 owner=0 fleet=0 index=0 player_event=0", "scope: preserved opaque object", "title=first", "title=second"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("missing %q in %s", want, detail)
		}
	}
	if strings.Index(detail, "title=first") > strings.Index(detail, "title=second") || strings.Contains(detail, "%") || strings.Contains(detail, "Site completed: true") {
		t.Fatal("raw history or unverified meaning changed")
	}
	id = 3
	if !renderArchaeology(gs, 0, &c, &id).IsError {
		t.Fatal("unreferenced country site accepted")
	}
}

func TestArchaeologyMissingValuesAndUnsupportedWarning(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`archaeological_sites={sites={0={last_excavator_country=0} 1={days_left={unexpected=yes}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	id := 0
	text := resultText(renderArchaeology(gs, 0, &gamestate.Country{}, &id))
	for _, want := range []string{"Archaeology decoding incomplete", "raw tree/byte tools", "index=not recorded days_left=not recorded", "type=not recorded id=not recorded", "locked: not recorded", "completed entries: not recorded", "events: not recorded", "log: not recorded"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}
