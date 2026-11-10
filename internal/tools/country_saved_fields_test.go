package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestCountrySavedFieldsRawRecovery(t *testing.T) {
	longKey := strings.Repeat("境", 130)
	longValue := strings.Repeat("値", 130)
	field := fmt.Sprintf("%q = %q", longKey, longValue)
	source := "loose player={country=0} player={{country=0}} country={0={novel=01 # example\n marker novel=02 nested={item=1 free} " + field + "}}"
	syntheticSaveDir(t, []syntheticEntry{{"meta", []byte("format=sample")}, {"gamestate", []byte(source)}})
	result := rawCall(t, handleCountrySavedFields, nil)
	if result["country_id"] != float64(0) || result["country_path"] != "/3/0" || result["country_kind"] != "mixed" ||
		result["entry_index"] != float64(1) || result["field_count"] != float64(5) || result["match_count"] != float64(5) {
		t.Fatal("incorrect selected country metadata")
	}
	children := result["fields"].([]any)
	for i, value := range children {
		child := value.(map[string]any)
		if child["path"] != fmt.Sprintf("/3/0/%d", i) || child["child_index"] != float64(i) {
			t.Fatalf("wrong path or position for child %d", i)
		}
		switch i {
		case 0, 2:
			if child["key"] != "novel" || child["key_occurrences"] != float64(2) || child["anonymous"] != false {
				t.Fatal("duplicate field occurrence lost")
			}
		case 1:
			if child["anonymous"] != true || child["key"] != nil || child["scalar_preview"] != "marker" {
				t.Fatal("anonymous scalar lost")
			}
		case 3:
			if child["kind"] != "mixed" || child["children_count"] != float64(2) {
				t.Fatal("nested mixed block misclassified")
			}
		case 4:
			if child["scalar_preview_truncated"] != true || child["key_preview_truncated"] != true ||
				child["entry_source_offset"] != float64(strings.Index(source, field)) || child["entry_source_bytes"] != float64(len(field)) {
				t.Fatal("large field lost preview or exact source range")
			}
			args := map[string]any{"save_id": result["save_id"], "revision": result["revision"], "entry_index": result["entry_index"], "path": child["path"]}
			tree := rawCall(t, handleReadSaveTree, args)
			if tree["node"].(map[string]any)["key"] != child["key"] {
				t.Fatal("path does not resolve in read_save_tree")
			}
			for part, value := range map[string]string{"key": longKey, "value": longValue} {
				args["part"] = part
				raw := rawCall(t, handleReadSaveScalar, args)
				if raw["data"] != base64.StdEncoding.EncodeToString([]byte(value)) {
					t.Fatalf("read_save_scalar did not recover the %s", part)
				}
			}
			delete(args, "path")
			delete(args, "part")
			args["offset"], args["limit"] = child["entry_source_offset"], child["entry_source_bytes"]
			raw := rawCall(t, handleReadSaveBytes, args)
			if raw["data"] != base64.StdEncoding.EncodeToString([]byte(field)) {
				t.Fatal("byte range did not recover the original field")
			}
		}
	}
}

func TestCountrySavedFieldsQuery(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={country=0} country={0={"ÄrG"=1 "ärG"=2 "年齢"=3 "Straße"=4 "É"=5 word=Straße nested={ärG=6} ÄrG}}`)}})
	for _, tc := range []struct {
		query string
		paths []string
	}{
		{"ÄRg", []string{"/1/0/0", "/1/0/1"}},
		{"齢", []string{"/1/0/2"}},
		{"STRASSE", nil},
		{"e\u0301", nil},
		{"straße", []string{"/1/0/3"}},
		{"missing", nil},
	} {
		result := rawCall(t, handleCountrySavedFields, map[string]any{"query": tc.query})
		fields := result["fields"].([]any)
		if len(fields) != len(tc.paths) || result["match_count"] != float64(len(tc.paths)) {
			t.Fatalf("wrong match count for %q", tc.query)
		}
		for i, raw := range fields {
			field := raw.(map[string]any)
			if field["path"] != tc.paths[i] || field["key_occurrences"] != float64(1) {
				t.Fatal("search did not retain the original index and case-sensitive duplicate count")
			}
		}
	}
}

func TestCountrySavedFieldsStrictSelection(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		args         map[string]any
		wantError    bool
	}{
		{"empty record", "player={country=0} country={0={}}", nil, false},
		{"explicit with malformed players", "player={country=1 anonymous} country={0={}}", map[string]any{"country_id": 0}, false},
		{"disagreeing players", "player={{country=0} {country=1}} country={0={} 1={}}", nil, true},
		{"mixed player", "player={country=0 anonymous} country={0={}}", nil, true},
		{"duplicate player country", "player={country=0 country=0} country={0={}}", nil, true},
		{"missing player country", "player={{country=0} {sample=0}} country={0={}}", nil, true},
		{"duplicate country roots", "player={country=0} country={0={}} country={0={}}", nil, true},
		{"numeric duplicate country keys", "player={country=0} country={0={test=1 anonymous} 00={}}", nil, true},
		{"mixed country table", "player={country=0} country={0={} anonymous}", nil, true},
		{"list country record", "player={country=0} country={0={anonymous}}", nil, true},
		{"scalar country record", "player={country=0} country={0=invalid}", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(tc.source)}})
			result, err := handleCountrySavedFields(context.Background(), rawRequest(tc.args))
			if err != nil || result.IsError != tc.wantError {
				t.Fatal("unexpected country selection outcome")
			}
			if !tc.wantError && !strings.Contains(resultText(result), `"fields":[]`) {
				t.Fatal("empty country record did not produce an empty list")
			}
		})
	}
}

func TestCountrySavedFieldsPagination(t *testing.T) {
	fields := strings.Repeat("hit=1 unfiltered=2 ", 21)
	source := "player={country=0} country={0={" + fields + "} 1={" + fields + "}}"
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	s := server.NewMCPServer("test", "test")
	oldLoc := loc
	t.Cleanup(func() { loc = oldLoc })
	Register(s, dir, nil)
	tool := s.ListTools()["get_country_saved_fields"].Tool
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint ||
		tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only tool registration")
	}
	first := rawCall(t, handleCountrySavedFields, map[string]any{"query": "HIT"})
	if first["field_count"] != float64(42) || first["match_count"] != float64(21) ||
		len(first["fields"].([]any)) != 20 || first["next_offset"] != float64(20) ||
		first["fields"].([]any)[0].(map[string]any)["key_occurrences"] != float64(21) {
		t.Fatal("wrong first page")
	}
	saveID, token := first["save_id"], first["page_context"]
	args := map[string]any{"save_id": saveID, "page_context": token, "query": "HIT", "offset": 20, "country_id": 0}
	second := rawCall(t, handleCountrySavedFields, args)
	if rows := second["fields"].([]any); len(rows) != 1 || second["next_offset"] != nil || rows[0].(map[string]any)["path"] != "/1/0/40" {
		t.Fatal("wrong continuation page or lost original child index")
	}
	badCalls := []map[string]any{
		{"save_id": saveID, "query": "hit", "offset": 20, "page_context": token},
		{"save_id": saveID, "query": "HIT", "offset": 20, "page_context": token, "country_id": 1},
		{"save_id": saveID, "query": "HIT", "offset": 22, "page_context": token},
		{"offset": 20},
		{"offset": -1},
		{"offset": 0.5},
		{"country_id": -1},
		{"country_id": "0"},
		{"save_id": false},
		{"page_context": 0},
		{"query": 123},
	}
	writeSyntheticSave(t, filepath.Join(dir, "another.sav"), []syntheticEntry{{"gamestate", []byte(source)}})
	catalog := rawCall(t, handleSaveCatalog, nil)
	saves := catalog["saves"].([]any)
	if len(saves) != 2 {
		t.Fatal("expected two synthetic saves")
	}
	for _, value := range saves {
		if id := value.(map[string]any)["save_id"]; id != saveID {
			badCalls = append(badCalls, map[string]any{"save_id": id, "query": "HIT", "offset": 20, "page_context": token})
		}
	}
	for i, bad := range badCalls {
		result, err := handleCountrySavedFields(context.Background(), rawRequest(bad))
		if err != nil || !result.IsError {
			t.Fatalf("invalid continuation or argument %d accepted", i)
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(source + " ")}})
	if result, err := handleCountrySavedFields(context.Background(), rawRequest(args)); err != nil || !result.IsError {
		t.Fatal("old continuation survived changed archive revision")
	}
}
