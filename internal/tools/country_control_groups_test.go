package tools

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestCountryControlGroupsSavedOrderAndCandidates(t *testing.T) {
	const source = `player={{country=0} {country=0}} country={0={control_groups={
hotkey={{id=0 type=4} {id=1 id=01 id=+1 id=1 id=1 type=9} {id={bad=x} type=no}
{id=2 type=0 type=9} {id=3 type=-001 anon} {} bad keyed={id=0 type=4}}
hotkey={} {{type=1}} scalar=unsupported} control_groups={} control_groups=bad}}
fleet={0={marker=yes} 2={} 3=none 4={type_only=yes}}
ships={0={marker=yes} 00={duplicate=yes} 2=none 3={}}
ship_design={0=none 2={value=1} 3="none" invalid={}}`
	syntheticSaveDir(t, []syntheticEntry{{"meta", []byte("synthetic=yes")}, {"gamestate", []byte(source)}})
	result := rawCall(t, handleCountryControlGroups, nil)
	rows := result["rows"].([]any)
	if result["country_id"] != float64(0) || result["entry_index"] != float64(1) ||
		result["field_occurrences"] != float64(3) || len(rows) != 15 {
		t.Fatal("saved slots or wrappers were lost")
	}
	for i, want := range []struct{ suffix, role string }{
		{"0/0", "slot"}, {"0/0/0", "item"}, {"0/0/1", "item"}, {"0/0/2", "item"}, {"0/0/3", "item"},
		{"0/0/4", "mixed-item"}, {"0/0/5", "empty-item"}, {"0/0/6", "unsupported-item"}, {"0/0/7", "unsupported-item"},
		{"0/1", "slot"}, {"0/2", "anonymous-slot"}, {"0/2/0", "item"}, {"0/3", "slot"},
		{"1", "empty-control-groups"}, {"2", "unsupported-control-groups"},
	} {
		row := rows[i].(map[string]any)
		if row["path"] != "/1/0/"+want.suffix || row["role"] != want.role {
			t.Fatalf("saved order or shape lost at row %d", i)
		}
	}
	first := rows[1].(map[string]any)
	if rows[0].(map[string]any)["key"] != "hotkey" || rows[9].(map[string]any)["key"] != "hotkey" ||
		rows[9].(map[string]any)["kind"] != "empty_block" || rows[10].(map[string]any)["key"] != nil ||
		first["slot_path"] != "/1/0/0/0" || first["container_kind"] != "mixed" {
		t.Fatal("duplicate slot key, empty slot or anonymous slot lost")
	}
	for _, rowIndex := range []int{1, 4, 5} {
		want := map[int][]string{
			1: {"numeric-candidate", "duplicate", "observed"},
			4: {"empty-block", "none", "observed"},
			5: {"none", "empty-block", "observed"},
		}[rowIndex]
		for i, item := range rows[rowIndex].(map[string]any)["numeric_candidates"].([]any) {
			candidate := item.(map[string]any)
			if candidate["table"] != []string{"fleet", "ships", "ship_design"}[i] || candidate["status"] != want[i] {
				t.Fatalf("wrong numeric candidate at row %d, table %d", rowIndex, i)
			}
			if i == 0 && rowIndex == 1 && candidate["path"] != "/2/0" {
				t.Fatal("type value was used as a fleet key")
			}
			if i == 1 && rowIndex == 1 && candidate["duplicate_paths_preview"].([]any)[1] != "/3/1" {
				t.Fatal("numeric key alias did not preserve both paths")
			}
		}
	}
	for _, tc := range []struct {
		row      int
		id, kind string
	}{
		{1, "integer", "integer"}, {2, "duplicate", "integer"}, {3, "unsupported", "unsupported"},
		{4, "integer", "duplicate"}, {5, "integer", "integer"}, {11, "not-recorded", "integer"},
	} {
		row := rows[tc.row].(map[string]any)
		if row["id_field"].(map[string]any)["status"] != tc.id || row["type_field"].(map[string]any)["status"] != tc.kind ||
			(tc.id != "integer" && row["numeric_candidates"] != nil) {
			t.Fatalf("wrong id or type field status at row %d", tc.row)
		}
	}
	id := rows[2].(map[string]any)["id_field"].(map[string]any)
	if id["occurrences"] != float64(5) || id["preview_truncated"] != true || len(id["entries_preview"].([]any)) != 4 {
		t.Fatal("duplicate id fields did not retain a bounded preview")
	}
	field := id["entries_preview"].([]any)[1].(map[string]any)
	identity := map[string]any{"save_id": result["save_id"], "revision": result["revision"], "entry_index": result["entry_index"], "path": field["path"]}
	tree := rawCall(t, handleReadSaveTree, identity)
	if tree["node"].(map[string]any)["scalar_preview"] != "01" {
		t.Fatal("duplicate id field path did not resolve")
	}
	delete(identity, "path")
	identity["offset"], identity["limit"] = field["entry_source_offset"], field["entry_source_bytes"]
	bytes := rawCall(t, handleReadSaveBytes, identity)
	if bytes["data"] != base64.StdEncoding.EncodeToString([]byte("id=01")) {
		t.Fatal("raw source range lost the duplicate id spelling")
	}
}

func TestCountryControlGroupsTableCoverage(t *testing.T) {
	const source = `player={country=0} country={0={control_groups={slot={{id=0 type=0}}}} 1={}}
fleet={0={}} fleet={0={}} ship_design={0 1}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	result := rawCall(t, handleCountryControlGroups, nil)
	tables := result["candidate_tables"].([]any)
	row := result["rows"].([]any)[1].(map[string]any)
	for i, want := range []string{"duplicate", "not-recorded", "unsupported"} {
		if tables[i].(map[string]any)["status"] != want || row["numeric_candidates"].([]any)[i].(map[string]any)["status"] != "table-"+want {
			t.Fatalf("unavailable candidate table %d was not distinguished", i)
		}
	}
	if len(tables[0].(map[string]any)["duplicate_paths_preview"].([]any)) != 2 {
		t.Fatal("duplicate fleet table paths lost")
	}
	absent := rawCall(t, handleCountryControlGroups, map[string]any{"country_id": 1})
	if absent["field_occurrences"] != float64(0) || len(absent["rows"].([]any)) != 0 {
		t.Fatal("absent control_groups field was not distinguished")
	}
}

func TestCountryControlGroupsPaginationAndSelection(t *testing.T) {
	record := "{control_groups={slot={" + strings.Repeat("{id=0 type=1} ", 23) + "}}}"
	source := "player={{country=0} {country=0}} country={0=" + record + " 1=" + record + "}"
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	s := server.NewMCPServer("test", "test")
	registerCountryControlGroups(s)
	tool := s.ListTools()["get_country_control_groups"].Tool
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint ||
		tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Fatal("read-only annotations missing")
	}
	first := rawCall(t, handleCountryControlGroups, nil)
	if first["row_count"] != float64(24) || len(first["rows"].([]any)) != 20 || first["next_offset"] != float64(20) {
		t.Fatal("large slot list was not paged")
	}
	saveID, token := first["save_id"], first["page_context"]
	args := map[string]any{"save_id": saveID, "country_id": 0, "page_context": token, "offset": 20}
	second := rawCall(t, handleCountryControlGroups, args)
	if rows := second["rows"].([]any); len(rows) != 4 || second["next_offset"] != nil ||
		rows[0].(map[string]any)["path"] != "/1/0/0/0/19" || rows[0].(map[string]any)["slot_path"] != "/1/0/0/0" {
		t.Fatal("continuation lost the slot or first item position")
	}
	for _, bad := range []map[string]any{
		{"country_id": 1, "page_context": token, "offset": 20},
		{"page_context": token, "offset": 25},
		{"offset": 20},
		{"country_id": -1},
		{"country_id": "0"},
		{"offset": 0.5},
		{"save_id": false},
		{"page_context": 0},
	} {
		expectCountryControlGroupsError(t, bad)
	}
	savePath := filepath.Join(dir, "synthetic.sav")
	writeSyntheticSave(t, savePath, []syntheticEntry{{"gamestate", []byte(source + " ")}})
	expectCountryControlGroupsError(t, args)
	writeSyntheticSave(t, savePath, []syntheticEntry{{"gamestate", []byte(strings.Replace(source, "{country=0}}", "{country=1}}", 1))}})
	expectCountryControlGroupsError(t, nil)
}

func expectCountryControlGroupsError(t *testing.T, args map[string]any) {
	t.Helper()
	result, err := handleCountryControlGroups(context.Background(), rawRequest(args))
	if err != nil || !result.IsError {
		t.Fatal("invalid input accepted")
	}
}
