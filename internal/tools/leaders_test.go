package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
)

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
		got := resolveAssignment(gs, nil, gs.Leaders[tt.id])
		if !strings.Contains(got, tt.want) || strings.Contains(got, "Wrong fleet") {
			t.Errorf("leader %d: %q", tt.id, got)
		}
	}
	missing := gamestate.Leader{Location: gamestate.LeaderLocation{Type: "ship", IDRaw: intPointer(99)}}
	if got := resolveAssignment(gs, nil, missing); !strings.Contains(got, "record unavailable") {
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
