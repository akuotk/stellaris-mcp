package tools

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestCountryExplorationRecordsCandidates(t *testing.T) {
	source := `player={country=0} player={{country=0}}
country={0={visited_objects={0 00 1 2 9 bad {custom=1}} visited_objects={2 keyed=0}
visited_objects={} visited_objects=unsupported hyperlane_systems={0}}}
planets={planet={0={flag=yes} 1=none 2={}}}
galactic_object={0={} 00={other=yes} 1={x=1} 2={x=yes} bad={}}
bypasses={0=none 1="none" 2={value=1 anon}}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	result := rawCall(t, handleCountryExplorationRecords, nil)
	rows := result["rows"].([]any)
	if result["country_id"] != float64(0) || result["kind"] != "visited_objects" ||
		result["field_occurrences"] != float64(4) || len(rows) != 11 {
		t.Fatal("saved list occurrences were lost")
	}
	for i, want := range [][]string{
		{"numeric-candidate", "duplicate", "none"},
		{"numeric-candidate", "duplicate", "none"},
		{"none", "observed", "unsupported-record"},
		{"empty-block", "observed", "unsupported-record"},
		{"missing", "unresolved", "missing"},
	} {
		row := rows[i].(map[string]any)
		candidates := row["numeric_candidates"].([]any)
		for j, status := range want {
			candidate := candidates[j].(map[string]any)
			if candidate["status"] != status || candidate["table"] != []string{"planet", "system", "bypass"}[j] {
				t.Fatalf("wrong table candidate status for row %d, table %d", i, j)
			}
			if status == "observed" && candidate["record_status"] != "object" {
				t.Fatal("partial table hid the observed record shape")
			}
			if status == "duplicate" && (candidate["occurrences"] != float64(2) ||
				candidate["duplicate_paths_preview"].([]any)[1] != "/4/1") {
				t.Fatal("numeric key collision lost a raw path")
			}
		}
	}
	if rows[1].(map[string]any)["scalar_preview"] != "00" {
		t.Fatal("repeated value's spelling lost")
	}
	for i, role := range map[int]string{5: "unsupported-list-entry", 6: "unsupported-list-entry", 7: "mixed-entry", 8: "mixed-entry", 9: "empty-field", 10: "unsupported-field"} {
		row := rows[i].(map[string]any)
		if row["role"] != role || row["path"] == nil || (i != 7 && row["numeric_candidates"] != nil) {
			t.Fatalf("malformed, mixed or empty row %d was lost or interpreted as an ID", i)
		}
	}
	hyperlanes := rawCall(t, handleCountryExplorationRecords, map[string]any{"kind": "hyperlane_systems"})
	if hyperlanes["rows"].([]any)[0].(map[string]any)["path"] != "/2/0/4/0" || hyperlanes["field_occurrences"] != float64(1) {
		t.Fatal("hyperlane view did not select its own field")
	}
}

func TestCountryExplorationRecordsTerraOrderAndSource(t *testing.T) {
	const source = `player={country=0} country={0={
terra_incognita={size=0 "opaque"={0 1} "opaque"={} metadata={values={2}} 9 keyed={note=one anonymous}}
terra_incognita={1 2} terra_incognita=unknown terra_incognita={}}}`
	syntheticSaveDir(t, []syntheticEntry{{"meta", []byte("sample=1")}, {"gamestate", []byte(source)}})
	result := rawCall(t, handleCountryExplorationRecords, map[string]any{"kind": "terra_incognita"})
	rows := result["rows"].([]any)
	want := []string{"0/0", "0/1", "0/1/0", "0/1/1", "0/2", "0/3", "0/3/0", "0/3/0/0", "0/4", "0/5", "0/5/0", "0/5/1", "1", "2", "3"}
	if result["entry_index"] != float64(1) || result["field_occurrences"] != float64(4) || len(rows) != len(want) {
		t.Fatal("terra record fields or source entry lost")
	}
	for i, suffix := range want {
		if rows[i].(map[string]any)["path"] != "/1/0/"+suffix {
			t.Fatalf("wrong terra field order at row %d", i)
		}
	}
	field := rows[1].(map[string]any)
	if rows[0].(map[string]any)["numeric_candidates"] != nil || rows[0].(map[string]any)["scalar_preview"] != "0" ||
		field["key"] != "opaque" || field["children_count"] != float64(2) || rows[4].(map[string]any)["key"] != "opaque" ||
		rows[4].(map[string]any)["kind"] != "empty_block" || rows[12].(map[string]any)["role"] != "unsupported-field" {
		t.Fatal("named scalar, unknown field, duplicate or unusual terra shape lost")
	}
	identity := map[string]any{"save_id": result["save_id"], "revision": result["revision"], "entry_index": result["entry_index"], "path": field["path"]}
	tree := rawCall(t, handleReadSaveTree, identity)
	if tree["node"].(map[string]any)["path"] != field["path"] {
		t.Fatal("terra field path did not resolve")
	}
	delete(identity, "path")
	identity["offset"], identity["limit"] = field["entry_source_offset"], field["entry_source_bytes"]
	original := rawCall(t, handleReadSaveBytes, identity)
	if original["data"] != base64.StdEncoding.EncodeToString([]byte(`"opaque"={0 1}`)) {
		t.Fatal("terra field did not recover the original bytes")
	}
}

func TestCountryExplorationRecordsTableCoverage(t *testing.T) {
	source := `player={country=0} country={0={visited_objects={0}}}
planets={planet={0={}} planet={0={}}} galactic_object={0=none anonymous} bypasses={0 1}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	result := rawCall(t, handleCountryExplorationRecords, nil)
	tables := result["candidate_tables"].([]any)
	candidates := result["rows"].([]any)[0].(map[string]any)["numeric_candidates"].([]any)
	for i, want := range []struct{ table, candidate string }{{"duplicate", "table-duplicate"}, {"partial", "observed"}, {"unsupported", "table-unsupported"}} {
		if tables[i].(map[string]any)["status"] != want.table || candidates[i].(map[string]any)["status"] != want.candidate {
			t.Fatalf("ambiguous table %d was not preserved", i)
		}
	}
	if tables[0].(map[string]any)["duplicate_paths_preview"].([]any)[1] != "/2/1" ||
		candidates[1].(map[string]any)["record_status"] != "none" {
		t.Fatal("duplicate table path or observed none lost")
	}
	missing := rawCall(t, handleCountryExplorationRecords, map[string]any{"kind": "hyperlane_systems"})
	if missing["field_occurrences"] != float64(0) || len(missing["rows"].([]any)) != 0 {
		t.Fatal("absent field did not return an empty result")
	}
}

func TestCountryExplorationRecordsPaginationAndSelection(t *testing.T) {
	list := "{" + strings.Repeat("0 ", 23) + "}"
	record := "{terra_incognita={data=" + list + "} visited_objects=" + list + "}"
	source := "player={{country=0} {country=0}} country={0=" + record + " 1=" + record + "}"
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	s := server.NewMCPServer("test", "test")
	registerCountryExplorationRecords(s)
	tool := s.ListTools()["get_country_exploration_records"].Tool
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint ||
		tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Fatal("read-only annotations missing")
	}
	first := rawCall(t, handleCountryExplorationRecords, map[string]any{"kind": "terra_incognita"})
	if len(first["rows"].([]any)) != 20 || first["row_count"] != float64(24) || first["next_offset"] != float64(20) {
		t.Fatal("inner list was not paged by saved elements")
	}
	saveID, token := first["save_id"], first["page_context"]
	args := map[string]any{"kind": "terra_incognita", "save_id": saveID, "country_id": 0, "page_context": token, "offset": 20}
	second := rawCall(t, handleCountryExplorationRecords, args)
	if rows := second["rows"].([]any); len(rows) != 4 || second["next_offset"] != nil || rows[0].(map[string]any)["path"] != "/1/0/0/0/19" {
		t.Fatal("continuation lost the saved inner-list position")
	}
	for _, bad := range []map[string]any{
		{"kind": "visited_objects", "page_context": token, "offset": 20},
		{"kind": "terra_incognita", "country_id": 1, "page_context": token, "offset": 20},
		{"kind": "terra_incognita", "page_context": token, "offset": 25},
		{"kind": "terra_incognita", "offset": 20},
		{"kind": "unknown"},
		{"kind": 0},
		{"country_id": -1},
		{"country_id": "0"},
		{"offset": 0.5},
		{"save_id": false},
		{"page_context": 0},
	} {
		expectCountryExplorationError(t, bad)
	}
	savePath := filepath.Join(dir, "synthetic.sav")
	writeSyntheticSave(t, savePath, []syntheticEntry{{"gamestate", []byte(source + " ")}})
	expectCountryExplorationError(t, args)
	writeSyntheticSave(t, savePath, []syntheticEntry{{"gamestate", []byte(strings.Replace(source, "{country=0}}", "{country=1}}", 1))}})
	expectCountryExplorationError(t, nil)
}

func expectCountryExplorationError(t *testing.T, args map[string]any) {
	t.Helper()
	result, err := handleCountryExplorationRecords(context.Background(), rawRequest(args))
	if err != nil || !result.IsError {
		t.Fatal("invalid input accepted")
	}
}
