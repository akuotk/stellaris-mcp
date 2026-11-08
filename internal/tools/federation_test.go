package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestFederationToolSelectsExactID(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`federation={0={name={key="First"}} 2={name={key="Second"}}}`)}})
	r, err := handleFederations(context.Background(), rawRequest(map[string]any{"federation_id": 0}))
	if err != nil || r.IsError {
		t.Fatalf("selection failed: %v", err)
	}
	text := resultText(r)
	if !strings.Contains(text, "ID 0: First") || strings.Contains(text, "Second") {
		t.Fatal(text)
	}
	r, err = handleFederations(context.Background(), rawRequest(map[string]any{"federation_id": 99}))
	if err != nil || !r.IsError {
		t.Fatal("missing federation not rejected")
	}
}

func TestFederationOutputPreservesIDsAndRawProgress(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Synthetic nation"}}}
 federation={7={name={key="Later federation"}} 0={name={key="First federation"} leader=0 members={0} associates={22}
 federation_progression={federation_type="synthetic_type" cohesion=0.125 experience=0 levels=0 base_cohesion=-0.2 laws={z=last a=first} free_migration=no
 action_settings={synthetic_action={vote_type=all acceptance_type=full}} timed_modifier={items={{modifier="synthetic_modifier" days=8 multiplier=0.5}}}}
 proposed_law_votes_cooldowns={token="vote_one" target=0 start_date="2201.01.01"}
 proposed_law_votes_cooldowns={token="vote_two" target=22 start_date="2201.01.02"}
 }}`))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	writeFederations(&b, gs)
	text := b.String()
	for _, want := range []string{"ID 0: First federation", "Synthetic nation (ID 0)", "ID 22", "cohesion=0.125", "experience=0", "levels (as saved): 0", "free_migration: false", "Law a: first", "Law z: last", "vote_one", "vote_two", "multiplier=0.5", "Leader: not recorded"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "%") || strings.Index(text, "ID 0:") > strings.Index(text, "ID 7:") || strings.Index(text, "Law a:") > strings.Index(text, "Law z:") {
		t.Fatal("output scaled or nondeterministic")
	}
}

func TestFederationOutputEmptyTable(t *testing.T) {
	var b strings.Builder
	writeFederations(&b, &gamestate.GameState{})
	if b.String() != "\nFederations:\n  No federation records.\n" {
		t.Fatal(b.String())
	}
}

func TestFederationReciprocalReferencesAndMissingRecords(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={federation=0} 1={associated_federation=9} 2={}} federation={0={members={0 0 2 99} associates={1} federation_progression={envoy={0 9}}}} leaders={0={location={type=federation id=0}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	writeFederations(&b, gs)
	for _, want := range []string{"member country 0: reciprocal saved IDs agree", "member country 0: duplicate saved reference", "member country 2: reverse reference not recorded", "member country 99: record unavailable", "associate country 1: reverse federation ID 9 differs", "Envoy 0: reciprocal saved federation location agrees", "Envoy 9: leader record unavailable"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("missing %q in %s", want, b.String())
		}
	}
}

func TestFederationInvalidProgressAndDuplicateLeaderRemainVisible(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`federation={0={name={key="Preserved"} leader=0 leader=1 federation_progression=bad}}`))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	writeFederations(&b, gs)
	for _, want := range []string{"ID 0: Preserved", "Leader: unsupported or duplicate", "federation_progression unsupported or duplicate"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("missing %q in %s", want, b.String())
		}
	}
}
