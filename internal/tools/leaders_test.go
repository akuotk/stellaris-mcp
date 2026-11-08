package tools

import (
	"context"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
)

func TestLeaderRolesSpeciesAndTypedAssignments(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={
	 0={adjective={key="First country"} ruler=0 ruler_title=example_title owned_leaders={0 1 2 2} government={heir=1 council_positions={0} picked_council_types={wrong_type}}}
	 1={adjective={key="Other country"} ruler=0 owned_leaders={1}}
	 2={ruler=0 ruler=1 owned_leaders={0} owned_leaders={1} government={heir=0 heir=1}}
	 3={ruler=99 government={heir=4}}
	 } leaders={
	 0={country=0 species=0 planet=0 age=0 experience=00.123456789 death_date=0 leader_terms=0 available_trait=first available_trait=second council_location={type=council_position id=0 position=0}}
	 1={country=1 location={type=army id=0}}
	 2={country=0 country=1 species={bad=yes} location=unexpected council_location={type=council_position id=0 id=1}}
	 3={location={type=army id=3}}
	 4=none 04={country=1}
	 5={country=0 council_location={type=council_position id=1}}
	 6={known=field anonymous}
	 } species_db={0={name={key="First species"}}} planets={planet={0={name={key="First planet"}}}}
	 army={0={name={key="First army"} leader=1} 3={leader=3 leader=0}}
	 council_positions={council_positions={0={country=0 leader=0 type=correct_type} 1={country=0 leader=0 type=other_type}}}`)}})
	summary, err := handleLeaders(context.Background(), rawRequest(nil))
	if err != nil || summary.IsError {
		t.Fatal("normal leader summary failed")
	}
	for _, want := range []string{`country.ruler_title="example_title"`, "country.ruler=0 (saved reference)", "government.heir=1 (saved reference)", "leader.country agrees with the saved country ID", "leader.country differs from the saved country ID", "Leader 2 has duplicate saved owned_leaders references", "Leader table decoding incomplete"} {
		if !strings.Contains(resultText(summary), want) {
			t.Fatalf("missing %q in %s", want, resultText(summary))
		}
	}
	for _, tc := range []struct {
		id    int
		want  []string
		avoid string
	}{
		{0, []string{"Saved references: country=0 species=0 planet=0", "Species ID candidate 0 (First species)", "Saved leader.planet ID candidate 0 (First planet)", `Experience: "00.123456789"`, `key="death_date": "0"`, `key="leader_terms": "0"`, `key="available_trait" [duplicate saved key]: "second"`, "Country 0 saved references: country.ruler, owned_leaders=validated", "Country 1 saved references: country.ruler", "Referencing country ID differs", "Correct Type (Council #0, reciprocal saved IDs agree)"}, "Wrong Type"},
		{1, []string{"Army 0 (First army, reciprocal saved leader IDs agree)", "Country 0 saved references: government.heir"}, "Country 2 saved references"},
		{2, []string{"country=unsupported or duplicate species=unsupported or duplicate", "Location: unsupported or duplicate", "Council: unsupported or duplicate"}, "Country ID candidate 0"},
		{3, []string{"Army 3 (duplicate army.leader field)", "country=not recorded"}, "reciprocal saved leader IDs agree"},
		{5, []string{"Council #1 (saved position.leader=0 differs)"}, "Other Type (Council"},
	} {
		result, err := handleLeaders(context.Background(), rawRequest(map[string]any{"leader_id": tc.id}))
		if err != nil || result.IsError {
			t.Fatal("normal leader detail failed")
		}
		text := resultText(result)
		for _, want := range tc.want {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in %s", want, text)
			}
		}
		if strings.Contains(text, tc.avoid) {
			t.Fatalf("unexpected %q in %s", tc.avoid, text)
		}
	}
	for id, want := range map[float64]string{4: "duplicate saved record ID", 6: "unsupported saved shape", 0.5: "must be an integer"} {
		result, err := handleLeaders(context.Background(), rawRequest(map[string]any{"leader_id": id}))
		if err != nil || !result.IsError || !strings.Contains(resultText(result), want) {
			t.Fatalf("unsupported leader selection %g not isolated", id)
		}
	}
	for id, want := range map[int]string{0: "government.heir=1 (saved reference)", 2: "government.heir=unsupported or duplicate", 3: "Leader ID candidate 4 (duplicate saved record ID"} {
		result, err := handleCouncil(context.Background(), rawRequest(map[string]any{"country_id": id}))
		if err != nil || result.IsError || !strings.Contains(resultText(result), want) {
			t.Fatalf("saved country roles missing from council for synthetic country %d", id)
		}
	}
}

func TestLeaderShipLocationUsesReciprocalShipFleetReference(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`leaders={1={location={type=ship id=0}} 2={location={type=ship}} 3={location={type=ship id=2}} 4={location={type=ship id=3}} 5={location={type=ship id=4}}} ships={0={fleet=7} 2={} 3={fleet=8} 4={fleet=9}} fleet={0={name={key="Wrong fleet"} ships={}} 7={name={key="Verified fleet"} ships={0}} 8={ships={}}}`))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		id   int
		want string
	}{{1, "Ship 0, fleet Verified fleet"}, {2, "Ship reference not recorded"}, {3, "Ship 2 (fleet reference not recorded)"}, {4, "Ship 3, fleet 8 (membership unverified)"}, {5, "Ship 4, fleet 9 (record unavailable)"}}
	for _, tt := range tests {
		got := resolveAssignment(gs, tt.id, gs.Leaders[tt.id])
		if !strings.Contains(got, tt.want) || strings.Contains(got, "Wrong fleet") {
			t.Errorf("leader %d: %q", tt.id, got)
		}
	}
	missing := gamestate.Leader{Location: gamestate.LeaderLocation{Type: "ship", IDRaw: intPointer(99)}}
	if got := resolveAssignment(gs, 0, missing); !strings.Contains(got, "record unavailable") {
		t.Errorf("missing ship: %s", got)
	}
}
func intPointer(n int) *int { return &n }

func TestLeaderSummaryReportsMissingReferences(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs := &gamestate.GameState{Leaders: map[int]gamestate.Leader{}}
	result, err := leaderSummary(gs, 0, &gamestate.Country{OwnedLeaders: []int{99}})
	if err != nil {
		t.Fatal(err)
	}
	got := resultText(result)
	if !strings.Contains(got, "Leader 99: referenced record unavailable") || strings.Contains(got, "No owned leader references recorded") {
		t.Fatal(got)
	}
}
