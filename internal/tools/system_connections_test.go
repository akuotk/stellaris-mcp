package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestSystemConnectionsSavedLinksAndMismatches(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`galactic_object={0={hyperlane={{to=1 length=0.000001 bridge=yes} {to=1 length=0} {to=2} {to=99} {to=0} {to=0 to=2} {}}
	 bypasses={0 9} natural_wormholes={0 1 99}}
	 1={name={key="Synthetic target"} hyperlane={{to=0 length=0.000002 bridge=no} {to=0 length=0} bad}}
	 2={hyperlane={}}}
	 bypasses={0={type=raw active=unknown owner={type=0 id=1} linked_to=1 connections={1 2 0} active_connections={1} lock_country=0 lock_remaining_days=-1}
	 1={linked_to=0 connections={0} active_connections={}}}
	 natural_wormholes={0={bypass=0 coordinate={origin=0}} 1={bypass=7 coordinate={origin=2}}}`))
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(renderSystemConnections(gs, 0))
	for _, want := range []string{
		`Entry 0 to=1 length="0.000001" bridge="yes"`, "Target system 1 Synthetic target", "Saved length texts differ", "Saved bridge values differ",
		"Multiple reverse saved entries=2", "Unsupported target hyperlane entries=1", "Duplicate saved target ID", "No decoded reverse hyperlane entry",
		"Target system record unavailable", "Saved target equals the source system ID", "Entry 5 unsupported or duplicate fields", "Entry 6 to=not recorded",
		`owner.type="0" owner.id=1 (namespace unverified)`, "Reciprocal saved linked_to IDs agree", "connections[0] bypass ID candidate 1 reverse connections membership=validated",
		"connections[1] bypass ID candidate 2 unavailable", "connections[2] equals the source bypass ID 0", "active_connections[0] bypass ID candidate 1 reverse active_connections membership=missing reverse reference",
		"Bypass 9 referenced record unavailable", "coordinate.origin=0 agrees", "Bypass candidate owner.id differs from the wormhole ID", "coordinate.origin=2 differs", "Referenced bypass ID candidate unavailable", "Natural wormhole 99 referenced record unavailable",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}

func TestSystemConnectionsMissingAndUnsupportedOutput(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`galactic_object={0={hyperlane={} bypasses={0} natural_wormholes={0}}
	1={hyperlane={} hyperlane={} bypasses={0} bypasses={1} natural_wormholes={bad}} 2={}}`))
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[int][]string{
		0: {"hyperlane (saved order)\n  Empty", "Bypass table not recorded", "Natural wormhole table not recorded"},
		1: {"Unsupported or duplicate saved list", "system.bypasses unsupported or duplicate", "system.natural_wormholes unsupported or duplicate"},
		2: {"hyperlane (saved order)\n  Not recorded", "system.bypasses (saved order)=not recorded"},
	} {
		text := resultText(renderSystemConnections(gs, id))
		for _, expected := range want {
			if !strings.Contains(text, expected) {
				t.Fatalf("missing %q in %s", expected, text)
			}
		}
	}
}

func TestSystemConnectionsRegisteredHandler(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`galactic_object={0={hyperlane={}}}`)}})
	s := server.NewMCPServer("test", "test")
	registerSystemConnections(s)
	if _, ok := s.ListTools()["get_system_connections"]; !ok {
		t.Fatal("system connection tool missing")
	}
	result, err := handleSystemConnections(context.Background(), rawRequest(map[string]any{"system_id": 0}))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "Saved connections for system 0") {
		t.Fatal("normal system connection handler failed")
	}
	for _, args := range []map[string]any{nil, {"system_id": nil}, {"system_id": "0"}, {"system_id": 0.5}, {"system_id": -1}, {"system_id": 1}} {
		result, err := handleSystemConnections(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Fatal("invalid or missing system ID accepted")
		}
	}
}
