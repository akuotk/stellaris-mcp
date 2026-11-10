package tools

import (
	"context"
	"encoding/base64"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestSaveRecordRefsNormalizationAndRawRecovery(t *testing.T) {
	first := `0={nested={0={marker=1}} large="` + strings.Repeat("値", 130) + `"}`
	source := `table={` + first + ` 00=none "0"="none" +0={a=1 free} -000={} 9=weird 09={b=1} 0.0={} -1={} named={} 184467440737095516160={big=yes} anonymous {x=1}}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	args := map[string]any{"revision": rawRevision(t), "table_path": "/0", "record_id": 0, "limit": 2}
	result := rawCall(t, handleSaveRecordRefs, args)
	table := result["table"].(map[string]any)
	if table["kind"] != "mixed" || table["status"] != "partial" || table["children_count"] != float64(13) ||
		result["lookup_status"] != "duplicate" || result["match_count"] != float64(5) {
		t.Fatal("wrong table completeness or numeric match count")
	}
	counts := table["counts"].(map[string]any)
	for k, want := range map[string]int{
		"numeric_key_count": 8, "unaddressable_numeric_key_count": 1, "nonnumeric_key_count": 3, "anonymous_count": 2, "object_count": 3,
		"none_count": 1, "empty_block_count": 1, "unsupported_value_count": 3,
		"duplicate_numeric_key_count": 2, "duplicate_numeric_extra_count": 5,
	} {
		if counts[k] != float64(want) {
			t.Fatalf("wrong count for %s", k)
		}
	}
	page := result
	for i, want := range []struct{ key, status string }{
		{"0", "object"}, {"00", "none"}, {"0", "unsupported-record"}, {"+0", "unsupported-record"}, {"-000", "empty-block"},
	} {
		if i > 0 && i%2 == 0 {
			args["offset"], args["page_context"] = page["next_offset"], page["page_context"]
			page = rawCall(t, handleSaveRecordRefs, args)
		}
		row := page["candidates"].([]any)[i%2].(map[string]any)
		if row["key"] != want.key || row["record_status"] != want.status || row["child_index"] != float64(i) {
			t.Fatalf("candidate %d lost its order, spelling or value status", i)
		}
		if i == 2 && (row["key_quoted"] != true || row["scalar_quoted"] != true) {
			t.Fatal("quoted numeric key or quoted none lost its quote flag")
		}
		if i == 0 {
			if row["children_count"] != float64(2) || row["children"] != nil {
				t.Fatal("candidate record was expanded inline")
			}
			raw := map[string]any{"save_id": result["save_id"], "revision": result["revision"], "entry_index": result["entry_index"]}
			raw["path"] = row["path"]
			tree := rawCall(t, handleReadSaveTree, raw)
			if tree["node"].(map[string]any)["path"] != "/0/0" {
				t.Fatal("candidate path did not resolve")
			}
			delete(raw, "path")
			raw["offset"], raw["limit"] = row["entry_source_offset"], row["entry_source_bytes"]
			bytes := rawCall(t, handleReadSaveBytes, raw)
			if bytes["data"] != base64.StdEncoding.EncodeToString([]byte(first)) {
				t.Fatal("candidate byte range did not recover the original record")
			}
		}
	}
	if page["next_offset"] != nil {
		t.Fatal("last page has a continuation")
	}
}

func TestSaveRecordRefsCoverage(t *testing.T) {
	source := `complete={0={nested={17={name=not_direct}}} 1=none 2={} 3={a=1 anon} 4={1 2}} empty={} mixed={9={} anon} array={1 2} scalar=other unindexed={some=value} boundary={9007199254740991={ok=yes}}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	revision := rawRevision(t)
	for _, tc := range []struct {
		path, table, status string
		id, matches         int
	}{
		{"/0", "complete", "unique", 0, 1},
		{"/0", "complete", "none", 1, 1},
		{"/0", "complete", "empty-block", 2, 1},
		{"/0", "complete", "unsupported-record", 3, 1},
		{"/0", "complete", "unsupported-record", 4, 1},
		{"/0", "complete", "missing", 17, 0},
		{"/1", "complete", "missing", 0, 0},
		{"/2", "partial", "unresolved", 0, 0},
		{"/2", "partial", "observed", 9, 1},
		{"/3", "unsupported", "table-unsupported", 0, 0},
		{"/4", "unsupported", "table-unsupported", 0, 0},
		{"/5", "partial", "unresolved", 0, 0},
		{"", "partial", "unresolved", 0, 0},
	} {
		result := rawCall(t, handleSaveRecordRefs, map[string]any{"revision": revision, "table_path": tc.path, "record_id": tc.id})
		if result["table"].(map[string]any)["status"] != tc.table || result["lookup_status"] != tc.status ||
			result["match_count"] != float64(tc.matches) || len(result["candidates"].([]any)) != tc.matches {
			t.Fatalf("wrong classification for synthetic table %q and ID %d", tc.path, tc.id)
		}
	}
	if maxSaveRecordID == 1<<53-1 {
		result := rawCall(t, handleSaveRecordRefs, map[string]any{"revision": revision, "table_path": "/6", "record_id": float64(maxSaveRecordID)})
		if result["lookup_status"] != "unique" {
			t.Fatal("largest exact JSON integer did not resolve")
		}
	}
	expectSaveRecordRefsError(t, map[string]any{"revision": revision, "table_path": "/90", "record_id": 0}, "does not exist")
}

func TestSaveRecordRefsPartialMatches(t *testing.T) {
	rows := "0={value=1} 1=none 2={} 3={field=1 anonymous}"
	source := "objects={" + rows + " metadata=present} mixed={" + rows + " anonymous}"
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	revision := rawRevision(t)
	for _, path := range []string{"/0", "/1"} {
		for id, status := range []string{"object", "none", "empty-block", "unsupported-record"} {
			args := map[string]any{"revision": revision, "table_path": path, "record_id": id}
			result := rawCall(t, handleSaveRecordRefs, args)
			candidate := result["candidates"].([]any)[0].(map[string]any)
			if result["table"].(map[string]any)["status"] != "partial" || result["lookup_status"] != "observed" ||
				result["observed_record_status"] != status || candidate["record_status"] != status ||
				result["match_count"] != float64(1) {
				t.Fatalf("partial lookup for %s returned the wrong observed status for %s", path, status)
			}
		}
	}
}

func TestSaveRecordRefsUnaddressableNumericKeys(t *testing.T) {
	maxKey := strconv.FormatInt(maxSaveRecordID, 10)
	aboveMax := strconv.FormatInt(maxSaveRecordID+1, 10)
	huge := strings.Repeat("9", 1024)
	source := `table={"+000` + maxKey + `"={at_bound=yes} ` + aboveMax + "=none 00" + aboveMax + "={} " + huge + "=none 0={}}"
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	args := map[string]any{"revision": rawRevision(t), "table_path": "/0", "record_id": int(maxSaveRecordID)}
	result := rawCall(t, handleSaveRecordRefs, args)
	table := result["table"].(map[string]any)
	counts := table["counts"].(map[string]any)
	if table["status"] != "complete" || result["lookup_status"] != "unique" ||
		counts["numeric_key_count"] != float64(5) || counts["unaddressable_numeric_key_count"] != float64(3) ||
		counts["duplicate_numeric_key_count"] != float64(1) {
		t.Fatal("numeric key counts did not respect the normalized input bound")
	}
	args["record_id"] = float64(maxSaveRecordID + 1)
	expectSaveRecordRefsError(t, args, "record_id must be an integer")
}

func TestSaveRecordRefsValidationAndContinuation(t *testing.T) {
	source := []byte("table={0={value=one} 0={value=two} 1={} 1={}} other={0={} 0={}}")
	entries := []syntheticEntry{{"meta", source}, {"gamestate", source}}
	dir := syntheticSaveDir(t, entries)
	s := server.NewMCPServer("test", "test")
	oldLoc := loc
	t.Cleanup(func() { loc = oldLoc })
	Register(s, dir, nil)
	tool := s.ListTools()["get_save_record_refs"].Tool
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint ||
		tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint ||
		tool.InputSchema.Properties["record_id"].(map[string]any)["type"] != "integer" {
		t.Fatal("missing read-only annotations or integer schema")
	}
	revision := rawRevision(t)
	first := rawCall(t, handleSaveRecordRefs, map[string]any{"revision": revision, "table_path": "/0", "record_id": 0, "limit": 1})
	saveID := first["save_id"]
	args := map[string]any{"save_id": saveID, "revision": revision, "entry_index": 1, "table_path": "/0", "record_id": 0,
		"offset": first["next_offset"], "page_context": first["page_context"]}
	second := rawCall(t, handleSaveRecordRefs, args)
	if rows := second["candidates"].([]any); len(rows) != 1 || rows[0].(map[string]any)["path"] != "/0/1" || second["next_offset"] != nil {
		t.Fatal("continuation with explicit gamestate index did not resolve")
	}
	tree := rawCall(t, handleReadSaveTree, map[string]any{"revision": revision, "path": "/0"})
	for _, change := range []struct {
		key  string
		val  any
		want string
	}{
		{"entry_index", 0, "page_context does not match"},
		{"entry_index", -2, "ZIP container is binary"},
		{"table_path", "/1", "page_context does not match"},
		{"table_path", "/00", "invalid positional path"},
		{"table_path", nil, "table_path is required"},
		{"record_id", 1, "page_context does not match"},
		{"record_id", -1, "record_id must be an integer"},
		{"record_id", 0.5, "record_id must be an integer"},
		{"record_id", math.NaN(), "record_id must be an integer"},
		{"record_id", float64(maxSaveRecordID) + 1, "record_id must be an integer"},
		{"record_id", "0", "record_id must be a JSON number"},
		{"offset", -1, "offset must be an integer"},
		{"offset", 3, "offset exceeds"},
		{"limit", 0, "limit must be an integer"},
		{"limit", 201, "limit must be an integer"},
		{"page_context", "", "page_context required"},
		{"page_context", tree["page_context"], "page_context does not match"},
		{"revision", "", "revision is required"},
		{"save_id", 0, "save_id must be a string"},
	} {
		old, present := args[change.key]
		args[change.key] = change.val
		expectSaveRecordRefsError(t, args, change.want)
		if present {
			args[change.key] = old
		} else {
			delete(args, change.key)
		}
	}
	delete(args, "record_id")
	expectSaveRecordRefsError(t, args, "record_id is required")
	args["record_id"] = 0
	writeSyntheticSave(t, filepath.Join(dir, "another.sav"), entries)
	catalog := rawCall(t, handleSaveCatalog, nil)
	if rows := catalog["saves"].([]any); len(rows) == 2 {
		for _, value := range rows {
			if id := value.(map[string]any)["save_id"]; id != saveID {
				args["save_id"] = id
				expectSaveRecordRefsError(t, args, "page_context does not match")
			}
		}
	} else {
		t.Fatal("expected two synthetic saves")
	}
	args["save_id"] = saveID
	entries[1].data = append(source, ' ')
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), entries)
	expectSaveRecordRefsError(t, args, "selected save changed")
	args["revision"] = rawCall(t, handleInspectSaveArchive, map[string]any{"save_id": saveID})["revision"]
	expectSaveRecordRefsError(t, args, "page_context does not match")
}

func expectSaveRecordRefsError(t *testing.T, args map[string]any, want string) {
	t.Helper()
	result, err := handleSaveRecordRefs(context.Background(), rawRequest(args))
	if err != nil || !result.IsError || !strings.Contains(resultText(result), want) {
		t.Fatalf("expected error containing %q", want)
	}
}
