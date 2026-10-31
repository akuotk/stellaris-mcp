package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestFirstContactCandidatesRawValuesAndHistory(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Synthetic nation"}} 1={adjective={key="Other nation"}}}
 galactic_object={0={name={key="Candidate system"}}} leaders={0={name={full_names={key="Candidate leader"}} location={type="first_contact" id=0}}}
 first_contacts={contacts={
  0={owner=0 country=1 name={key="Synthetic contact"} location=0 leader=0 stage="synthetic_stage" status="synthetic_status" last_roll=0 days_left=0 difficulty=0 clues=0
   event={event_id=0 expired=0 owner=0 fleet=0 effect=0 scope={opaque=0} index=0 player_event=0}
   events={{event_id="first_event"} {event_id="second_event"}}
   completed={{date="2201.01.01" stage="first_stage"} {date="2201.01.02" stage="second_stage"}}}
  1={owner=2 country=0 location=999 leader=999}
  2={owner=2 country=3}
 }}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	summary := resultText(renderFirstContacts(gs, 0, &c, nil))
	for _, want := range []string{"First contact 0:", "First contact 1:", "Possible system ID match: Candidate system", "Possible leader ID match: Candidate leader", "leader.location (raw): type=first_contact assignment= id=0", "Location reference unresolved", "Leader reference unresolved", "owner country reference unresolved"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in %s", want, summary)
		}
	}
	if strings.Contains(summary, "First contact 2:") {
		t.Fatal("unrelated contact included")
	}
	id := 0
	detail := resultText(renderFirstContacts(gs, 0, &c, &id))
	for _, want := range []string{"stage=synthetic_stage status=synthetic_status", "last_roll=0 days_left=0 difficulty=0 clues=0", "event: event_id=0 expired=0", "owner=0 fleet=0 effect=0 (as saved)", "events[0] (saved order)", "owner=not recorded fleet=not recorded effect=not recorded", "completed[0] (saved history)", "stage=first_stage", "stage=second_stage"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("missing %q in %s", want, detail)
		}
	}
	if strings.Index(detail, "event_id=first_event") > strings.Index(detail, "event_id=second_event") || strings.Contains(detail, "%") || strings.Contains(detail, "Active: true") {
		t.Fatal("raw history/state interpreted")
	}
	id = 2
	if !renderFirstContacts(gs, 0, &c, &id).IsError {
		t.Fatal("unrelated contact accepted")
	}
}

func TestFirstContactMissingValuesUnsupportedAndLeaderMismatch(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`leaders={0={location={type="first_contact" id=9}}} first_contacts={contacts={0={owner=0 leader=0} 1={difficulty={unknown=shape}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	id := 0
	text := resultText(renderFirstContacts(gs, 0, &gamestate.Country{}, &id))
	for _, want := range []string{"First contact decoding incomplete", "raw tree/byte tools", "stage=not recorded status=not recorded", "Raw leader.location ID differs from contact ID; namespace/timing unverified", "last_roll=not recorded days_left=not recorded", "event: not recorded", "events: not recorded", "completed history: not recorded"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	leader := gs.Leaders[0]
	leader.Location.Type = "unknown_namespace"
	gs.Leaders[0] = leader
	text = resultText(renderFirstContacts(gs, 0, &gamestate.Country{}, &id))
	if !strings.Contains(text, "No reverse first_contact leader assignment validated") {
		t.Fatal("unknown location namespace treated as verified")
	}
}
