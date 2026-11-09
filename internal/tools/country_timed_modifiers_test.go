package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestCountryTimedModifiersRawFallback(t *testing.T) {
	field := "nested={# synthetic comment\n detail={payload=\"" + strings.Repeat("x", 512) + "\" anonymous}}"
	fixture := "player={country=0} country={0={timed_modifier={items={{" + field + "}}}}}"
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	result := callCountryTimedModifiers(t, nil)
	if result.IsError || !strings.Contains(resultText(result), "raw_preview_truncated=true") {
		t.Fatal("large value did not expose the bounded preview marker")
	}
	const path = "/1/0/0/0/0/0"
	offset := strings.Index(fixture, field)
	if !strings.Contains(resultText(result), fmt.Sprintf("path=%q kind=object source_offset=%d source_bytes=%d", path, offset, len(field))) {
		t.Fatal("large field did not retain its path and exact entry byte range")
	}
	revision := rawRevision(t)
	tree := rawCall(t, handleReadSaveTree, map[string]any{"path": path, "revision": revision})
	if tree["node"].(map[string]any)["path"] != path {
		t.Fatal("field path does not resolve in the raw tree")
	}
	bytes := rawCall(t, handleReadSaveBytes, map[string]any{"offset": offset, "limit": len(field), "revision": revision})
	if bytes["data"] != base64.StdEncoding.EncodeToString([]byte(field)) {
		t.Fatal("field range does not recover the original bytes")
	}
}

func callCountryTimedModifiers(t *testing.T, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := handleCountryTimedModifiers(context.Background(), rawRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCountryTimedModifiersSavedShapes(t *testing.T) {
	fixture := "player={{country=0} {country=0}} country={0={" +
		"timed_modifier={items={{modifier=\"synthetic\" days=-001 days=0 anonymous} {} strange slot={modifier=synthetic}} other={} items={}} " +
		"timed_modifier={unpaired items=bad} timed_modifier={} timed_modifier=weird}}"
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	result := callCountryTimedModifiers(t, nil)
	if result.IsError {
		t.Fatal(resultText(result))
	}
	text := resultText(result)
	for _, want := range []string{
		"timed_modifier_occurrences=4 items_occurrences=3 duplicate_items_wrappers=1 saved_items=4 saved_entries=10",
		"entry[0] role=item container_kind=mixed path=\"/1/0/0/0/0\" kind=mixed",
		"field[1] duplicate_field_key=true path=\"/1/0/0/0/0/1\"",
		"raw_preview=\"days=-001\"",
		"repeated_saved_key=true saved_key_occurrences=2",
		"entry[1] role=item container_kind=mixed path=\"/1/0/0/0/1\" kind=empty_block",
		"entry[2] role=item container_kind=mixed path=\"/1/0/0/0/2\" kind=scalar",
		"entry[3] role=item container_kind=mixed path=\"/1/0/0/0/3\" kind=object",
		"entry[4] role=wrapper_field container_kind=object",
		"entry[5] role=items container_kind=object",
		"entry[6] role=wrapper_field container_kind=mixed",
		"entry[7] role=items container_kind=mixed",
		"entry[8] role=timed_modifier container_kind=object path=\"/1/0/2\" kind=empty_block",
		"entry[9] role=timed_modifier container_kind=object path=\"/1/0/3\" kind=scalar",
		fmt.Sprintf("source_offset=%d source_bytes=%d raw_preview=\"days=-001\"", strings.Index(fixture, "days=-001"), len("days=-001")),
		"next_offset=none",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in synthetic result:\n%s", want, text)
		}
	}
}

func TestCountryTimedModifiersCountrySelection(t *testing.T) {
	for _, tt := range []struct {
		name, fixture string
		args          map[string]any
		wantError     bool
	}{
		{"missing modifier", "player={country=0} country={0={}}", nil, false},
		{"foreign explicit", "player={{country=1}} country={0={}}", map[string]any{"country_id": 0}, false},
		{"ambiguous players", "player={{country=0} {country=1}} country={0={} 1={}}", nil, true},
		{"mixed players", "player={country=0 anonymous} country={0={}}", nil, true},
		{"duplicate player country", "player={{country=0 country=0}} country={0={}}", nil, true},
		{"empty player with nonempty player", "player={} player={{country=0}} country={0={}}", nil, true},
		{"duplicate country table", "player={{country=0}} country={0={}} country={0={}}", nil, true},
		{"numeric duplicate country", "player={{country=0}} country={0={} 00={}}", nil, true},
		{"mixed country table", "player={{country=0}} country={0={} anonymous}", nil, true},
		{"mixed country row", "player={{country=0}} country={0={timed_modifier={} anonymous}}", nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(tt.fixture)}})
			result := callCountryTimedModifiers(t, tt.args)
			if result.IsError != tt.wantError {
				t.Fatal(resultText(result))
			}
			if !tt.wantError && !strings.Contains(resultText(result), "timed_modifier_occurrences=0") {
				t.Fatal("missing timed_modifier did not report zero occurrences")
			}
		})
	}
}

func TestCountryTimedModifiersPagination(t *testing.T) {
	var fixture strings.Builder
	fixture.WriteString("player={country=0} country={0={timed_modifier={items={")
	for i := 0; i < 21; i++ {
		fmt.Fprintf(&fixture, "{modifier=synthetic_%d days=%d}", i%20, i)
	}
	fixture.WriteString("}}} 1={}}")
	source := []byte(fixture.String())
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", source}})
	s := server.NewMCPServer("test", "test")
	registerCountryTimedModifiers(s)
	tool := s.ListTools()["get_country_timed_modifiers"].Tool
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint ||
		tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only annotations")
	}
	result := callCountryTimedModifiers(t, nil)
	first := resultText(result)
	if result.IsError || !strings.Contains(first, "entry[19]") || strings.Contains(first, "entry[20]") ||
		!strings.Contains(first, "next_offset=20") || !strings.Contains(first, "saved_key_occurrences=2") {
		t.Fatal("first page did not respect the 20-entry boundary or count repeated keys across pages")
	}
	var token, saveID string
	for _, line := range strings.Split(first, "\n") {
		if value, ok := strings.CutPrefix(line, "page_context="); ok {
			token = value
		}
		if value, ok := strings.CutPrefix(line, "save_id="); ok {
			saveID = value
		}
	}
	args := map[string]any{"save_id": saveID, "offset": 20, "page_context": token, "country_id": 0}
	result = callCountryTimedModifiers(t, args)
	second := resultText(result)
	if result.IsError || !strings.Contains(second, "entry[20]") || strings.Contains(second, "entry[19]") ||
		!strings.Contains(second, "next_offset=none") || !strings.Contains(second, "saved_key_occurrences=2") {
		t.Fatal("invalid second page")
	}
	for _, bad := range []map[string]any{
		{"country_id": 1, "offset": 20, "page_context": token},
		{"offset": 20},
		{"offset": 22, "page_context": token},
		{"country_id": "0"},
		{"offset": 0.5},
	} {
		if !callCountryTimedModifiers(t, bad).IsError {
			t.Fatal("invalid arguments accepted")
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", append(source, ' ')}})
	if !callCountryTimedModifiers(t, args).IsError {
		t.Fatal("old page token survived a save revision change")
	}
}
