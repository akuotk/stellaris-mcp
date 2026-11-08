package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestSpaceCombatObjectsPaginationAndAmbiguity(t *testing.T) {
	var raw strings.Builder
	raw.WriteString(`missile={0=none 00={fleet=0 position={origin=0}} bad={target={type=unknown id=0}} `)
	for i := 1; i <= 21; i++ {
		fmt.Fprintf(&raw, `%d={speed=%d} `, i, i)
	}
	raw.WriteString(`} strike_craft={0={action=0}} fleet={0={}} galactic_object={0={}}`)
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(raw.String())}})
	s := server.NewMCPServer("test", "test")
	registerSpaceCombatObjects(s)
	if _, ok := s.ListTools()["get_space_combat_objects"]; !ok {
		t.Fatal("tool not registered")
	}
	result, err := handleSpaceCombatObjects(context.Background(), rawRequest(map[string]any{"kind": "missile"}))
	if err != nil || result.IsError {
		t.Fatal("first page failed")
	}
	got := resultText(result)
	for _, want := range []string{"none_entries=1 non_none_entries=23", "duplicate numeric ID; identity ambiguous", "non-numeric key; identity unresolved", "fleet=0: saved table ID candidate exists", "position.origin=0: saved table ID candidate exists", "next_offset=20"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	var token string
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "page_context=") {
			token = strings.TrimPrefix(line, "page_context=")
		}
	}
	if token == "" {
		t.Fatal("missing page context")
	}
	result, err = handleSpaceCombatObjects(context.Background(), rawRequest(map[string]any{"kind": "missile", "offset": 20, "page_context": token}))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "next_offset=none") {
		t.Fatal("second page failed")
	}
	for _, args := range []map[string]any{{"kind": "missile", "offset": 20}, {"kind": "strike_craft", "page_context": token}, {"kind": "invalid"}, {"kind": "missile", "offset": 1.5}} {
		result, err = handleSpaceCombatObjects(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Fatal("invalid request accepted")
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(raw.String() + " changed=yes")}})
	result, err = handleSpaceCombatObjects(context.Background(), rawRequest(map[string]any{"kind": "missile", "offset": 20, "page_context": token}))
	if err != nil || !result.IsError {
		t.Fatal("changed save accepted old page context")
	}
}

func TestSpaceCombatObjectsMissingAndDuplicateTables(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{`date=2200.01.01`, "Table not recorded"}, {`missile={} missile={}`, "Table unsupported or duplicate"}, {`missile=bad`, "Table unsupported or duplicate"}} {
		syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(tc.input)}})
		result, err := handleSpaceCombatObjects(context.Background(), rawRequest(map[string]any{"kind": "missile"}))
		if err != nil || result.IsError || !strings.Contains(resultText(result), tc.want) {
			t.Fatalf("missing expected table state: %s", tc.want)
		}
	}
}
