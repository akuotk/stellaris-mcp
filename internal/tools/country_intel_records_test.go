package tools

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestCountryIntelRecordsManager(t *testing.T) {
	source := `player={country=0} player={{country=0}}
country={0={intel_manager={other=1 intel={
 {0 {intel=01 stale_intel={key=yes anon} stale_intel=-001}}
 {0 {intel=02}}
 {2 {intel=03}}
 {7 {intel=04}}
 {8 {intel=05}}
 named={0 {intel=6}}
 {3 unknown}
 malformed
} anonymous intel={} intel=bad} intel_manager={} intel_manager=bad}
 2=none 7={} 07={}}
spy_networks={0={} 2={} 02={} 7=none broken={}}`
	syntheticSaveDir(t, []syntheticEntry{{"meta", []byte("sample=1")}, {"gamestate", []byte(source)}})
	result := rawCall(t, handleCountryIntelRecords, nil)
	rows := result["rows"].([]any)
	if result["country_id"] != float64(0) || result["country_path"] != "/2/0" || result["entry_index"] != float64(1) ||
		result["field_occurrences"] != float64(3) || result["intel_occurrences"] != float64(3) || len(rows) != 14 {
		t.Fatal("manager wrappers, intel fields or row count lost")
	}
	for i, want := range []struct{ path, role string }{
		{"/2/0/0/0", "manager-field"},
		{"/2/0/0/1/0", "pair"}, {"/2/0/0/1/1", "pair"}, {"/2/0/0/1/2", "pair"}, {"/2/0/0/1/3", "pair"}, {"/2/0/0/1/4", "pair"},
		{"/2/0/0/1/5", "unsupported-pair"}, {"/2/0/0/1/6", "unsupported-pair"}, {"/2/0/0/1/7", "unsupported-pair"},
		{"/2/0/0/2", "manager-field"},
		{"/2/0/0/3", "empty-list"},
		{"/2/0/0/4", "unsupported-list"},
		{"/2/0/1", "empty-manager"},
		{"/2/0/2", "unsupported-manager"},
	} {
		row := rows[i].(map[string]any)
		if row["path"] != want.path || row["role"] != want.role {
			t.Fatalf("wrong saved shape or path at row %d", i)
		}
	}
	for i, want := range []struct{ country, spy string }{
		{"numeric-candidate", "numeric-candidate"}, {"numeric-candidate", "numeric-candidate"},
		{"unsupported", "duplicate"}, {"duplicate", "unsupported"}, {"missing", "unresolved"},
	} {
		row := rows[i+1].(map[string]any)
		candidates := row["numeric_candidates"].([]any)
		country, spy := candidates[0].(map[string]any), candidates[1].(map[string]any)
		if country["table"] != "country" || country["status"] != want.country ||
			spy["table"] != "spy_networks" || spy["status"] != want.spy {
			t.Fatalf("wrong candidate statuses at row %d", i+1)
		}
		if want.spy == "duplicate" && (spy["occurrences"] != float64(2) ||
			spy["duplicate_paths_preview"].([]any)[1] != "/3/2") {
			t.Fatal("duplicate spy numeric keys lost their paths")
		}
	}
	pair := rows[1].(map[string]any)
	counts := pair["payload_field_occurrences"].(map[string]any)
	if pair["head_occurrences"] != float64(2) || pair["container_kind"] != "mixed" ||
		pair["head"].(map[string]any)["scalar_preview"] != "0" ||
		pair["payload"].(map[string]any)["path"] != "/2/0/0/1/0/1" ||
		counts["intel"] != float64(1) || counts["stale_intel"] != float64(2) {
		t.Fatal("pair members or duplicates lost")
	}
	previews := pair["payload_fields_preview"].([]any)
	stale := previews[1].(map[string]any)
	if stale["key"] != "stale_intel" || stale["kind"] != "mixed" ||
		previews[2].(map[string]any)["scalar_preview"] != "-001" {
		t.Fatal("stale_intel shape or spelling lost")
	}
	const original = "stale_intel={key=yes anon}"
	identity := map[string]any{"save_id": result["save_id"], "revision": result["revision"], "entry_index": result["entry_index"]}
	identity["path"] = stale["path"]
	raw := rawCall(t, handleReadSaveTree, identity)
	if raw["node"].(map[string]any)["path"] != "/2/0/0/1/0/1/1" {
		t.Fatal("payload path did not resolve")
	}
	delete(identity, "path")
	identity["offset"], identity["limit"] = stale["entry_source_offset"], stale["entry_source_bytes"]
	bytes := rawCall(t, handleReadSaveBytes, identity)
	if bytes["data"] != base64.StdEncoding.EncodeToString([]byte(original)) {
		t.Fatal("payload range did not recover original bytes")
	}
	for _, row := range rows[6:9] {
		if row.(map[string]any)["numeric_candidates"] != nil {
			t.Fatal("unsupported pair was interpreted as a reference")
		}
	}
}

func TestCountryIntelRecordsArrays(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={country=0} country={0={
 intel_level={0 1 0} intel_level={foo=8 7 {block=1}} intel_level={} intel_level=bad
 highest_intel_level={9 10}}}`)}})
	result := rawCall(t, handleCountryIntelRecords, map[string]any{"kind": "intel_level"})
	rows := result["rows"].([]any)
	if result["field_occurrences"] != float64(4) || len(rows) != 8 || result["candidate_tables"] != nil {
		t.Fatal("array coverage incorrect")
	}
	for i, role := range []string{"array-element", "array-element", "array-element",
		"unsupported-list-entry", "unsupported-list-entry", "unsupported-list-entry", "empty-list", "unsupported-list"} {
		row := rows[i].(map[string]any)
		if row["role"] != role || row["numeric_candidates"] != nil {
			t.Fatalf("wrong role or inferred country candidate at row %d", i)
		}
	}
	if rows[2].(map[string]any)["path"] != "/1/0/0/2" || rows[2].(map[string]any)["scalar_preview"] != "0" ||
		rows[4].(map[string]any)["path"] != "/1/0/1/1" || rows[4].(map[string]any)["container_kind"] != "mixed" {
		t.Fatal("duplicate array value or mixed entry position lost")
	}
	highest := rawCall(t, handleCountryIntelRecords, map[string]any{"kind": "highest_intel_level"})
	if len(highest["rows"].([]any)) != 2 || highest["rows"].([]any)[0].(map[string]any)["path"] != "/1/0/4/0" {
		t.Fatal("highest array mixed with intel_level")
	}
	missing := rawCall(t, handleCountryIntelRecords, nil)
	if missing["field_occurrences"] != float64(0) || len(missing["rows"].([]any)) != 0 {
		t.Fatal("absent manager did not report zero rows")
	}
}

func TestCountryIntelRecordsSpyRootStatus(t *testing.T) {
	for _, tc := range []struct{ name, extra, status string }{
		{"missing", "", "not-recorded"},
		{"duplicate", "spy_networks={0={}} spy_networks={0={}}", "duplicate"},
		{"mixed", "spy_networks={0={} anonymous}", "unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "player={country=0} country={0={intel_manager={intel={{0 {intel=0}}}}}} " + tc.extra
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
			result := rawCall(t, handleCountryIntelRecords, nil)
			tables := result["candidate_tables"].([]any)
			spy := tables[1].(map[string]any)
			row := result["rows"].([]any)[0].(map[string]any)
			candidate := row["numeric_candidates"].([]any)[1].(map[string]any)
			if spy["status"] != tc.status || candidate["status"] != "table-"+tc.status {
				t.Fatal("spy table ambiguity was treated as a numeric match")
			}
			if tc.status == "duplicate" && len(spy["duplicate_paths_preview"].([]any)) != 2 {
				t.Fatal("duplicate spy table paths lost")
			}
		})
	}
}

func TestCountryIntelRecordsSelectionAndPagination(t *testing.T) {
	block := "{intel_level={" + strings.Repeat("0 ", 21) + "} highest_intel_level={" + strings.Repeat("1 ", 21) + "}}"
	source := "player={{country=0} {country=0}} country={0=" + block + " 1=" + block + "}"
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	s := server.NewMCPServer("test", "test")
	registerCountryIntelRecords(s)
	tool := s.ListTools()["get_country_intel_records"].Tool
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint ||
		tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Fatal("read-only annotations missing")
	}
	first := rawCall(t, handleCountryIntelRecords, map[string]any{"kind": "intel_level"})
	if len(first["rows"].([]any)) != 20 || first["next_offset"] != float64(20) {
		t.Fatal("incorrect first page")
	}
	id, token := first["save_id"], first["page_context"]
	args := map[string]any{"kind": "intel_level", "country_id": 0, "save_id": id, "page_context": token, "offset": 20}
	second := rawCall(t, handleCountryIntelRecords, args)
	if rows := second["rows"].([]any); len(rows) != 1 || second["next_offset"] != nil || rows[0].(map[string]any)["child_index"] != float64(20) {
		t.Fatal("incorrect second page")
	}
	for _, bad := range []map[string]any{
		{"kind": "highest_intel_level", "page_context": token, "offset": 20},
		{"kind": "intel_level", "country_id": 1, "page_context": token, "offset": 20},
		{"kind": "intel_level", "page_context": token, "offset": 22},
		{"kind": "intel_level", "offset": 20},
		{"kind": "unknown"},
		{"kind": 0},
		{"country_id": -1},
		{"country_id": "0"},
		{"save_id": false},
		{"offset": 0.5},
		{"page_context": 0},
	} {
		expectCountryIntelError(t, bad)
	}
	savePath := filepath.Join(dir, "synthetic.sav")
	writeSyntheticSave(t, savePath, []syntheticEntry{{"gamestate", []byte(source + " ")}})
	expectCountryIntelError(t, args)
	writeSyntheticSave(t, savePath, []syntheticEntry{{"gamestate", []byte(strings.Replace(source, "{country=0}}", "{country=1}}", 1))}})
	expectCountryIntelError(t, nil)
	explicit := rawCall(t, handleCountryIntelRecords, map[string]any{"country_id": 1})
	if explicit["country_id"] != float64(1) {
		t.Fatal("explicit selection did not bypass contradictory player records")
	}
}

func expectCountryIntelError(t *testing.T, args map[string]any) {
	t.Helper()
	result, err := handleCountryIntelRecords(context.Background(), rawRequest(args))
	if err != nil || !result.IsError {
		t.Fatal("invalid input accepted")
	}
}
