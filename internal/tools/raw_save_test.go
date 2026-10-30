package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
)

type syntheticEntry struct {
	name string
	data []byte
}

// This optional survey reports aggregate counts only; it never logs save content.
func TestRawParserLocalCatalogCoverage(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for a read-only survey")
	}
	files, err := gamestate.ListSaveFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	parsed, failed, missing := 0, 0, 0
	for _, save := range files {
		z, err := zip.OpenReader(save.Path)
		if err != nil {
			failed++
			continue
		}
		found := false
		for _, entry := range z.File {
			if entry.Name != "gamestate" && entry.Name != "meta" {
				continue
			}
			if entry.Name == "gamestate" {
				found = true
			}
			stream, err := entry.Open()
			if err != nil {
				failed++
				continue
			}
			data, err := io.ReadAll(stream)
			stream.Close()
			if err != nil {
				failed++
				continue
			}
			if _, err := clausewitz.ParseRaw(context.Background(), data); err != nil {
				failed++
			} else {
				parsed++
			}
		}
		z.Close()
		if !found {
			missing++
		}
	}
	t.Logf("local raw parser: %d saves, %d gamestate/meta entries parsed, %d entry errors, %d saves without gamestate", len(files), parsed, failed, missing)
	if parsed == 0 {
		t.Fatal("no local save entries parsed")
	}
}

func writeSyntheticSave(t *testing.T, path string, entries []syntheticEntry) {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	if err := z.SetComment("synthetic container comment"); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		w, err := z.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func syntheticSaveDir(t *testing.T, entries []syntheticEntry) string {
	t.Helper()
	dir := t.TempDir()
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), entries)
	old := saveDir
	saveDir = dir
	t.Cleanup(func() { saveDir = old })
	return dir
}

func rawRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

type rawHandler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

func rawCall(t *testing.T, handler rawHandler, args map[string]any) map[string]any {
	t.Helper()
	r, err := handler(context.Background(), rawRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	if r.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(r))
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(resultText(r)), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func rawRevision(t *testing.T) string {
	return rawCall(t, handleInspectSaveArchive, nil)["revision"].(string)
}

func TestRawSaveExactBytesAndArchivePagination(t *testing.T) {
	original := []byte("# synthetic comment\r\nvalue=\"日本語\\n\\\"quoted\\\"\"\r\nrepeat=one repeat=two")
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", original}, {"meta", []byte("name=synthetic")}, {"../never_extracted", []byte{0, 255, 1}}})
	inspect := rawCall(t, handleInspectSaveArchive, map[string]any{"limit": 1})
	revision := inspect["revision"].(string)
	if inspect["entry_count"] != float64(3) || inspect["next_offset"] != float64(1) || len(inspect["entries"].([]any)) != 1 {
		t.Fatal(inspect)
	}
	page := rawCall(t, handleInspectSaveArchive, map[string]any{"revision": revision, "offset": 2, "limit": 1})
	if page["next_offset"] != nil || page["entries"].([]any)[0].(map[string]any)["entry_index"] != float64(2) {
		t.Fatal(page)
	}
	for _, index := range []int{0, 1, 2, -2} {
		var got []byte
		offset := 0
		for {
			page := rawCall(t, handleReadSaveBytes, map[string]any{"revision": revision, "entry_index": index, "offset": offset, "limit": 7})
			chunk, err := base64.StdEncoding.DecodeString(page["data"].(string))
			if err != nil {
				t.Fatal(err)
			}
			if len(chunk) > 7 || page["returned_bytes"] != float64(len(chunk)) {
				t.Fatal("unbounded or incorrect chunk")
			}
			got = append(got, chunk...)
			if page["next_offset"] == nil {
				if index != -2 && page["entry_crc_verified"] != true {
					t.Fatal("final entry page did not verify CRC")
				}
				break
			}
			if index != -2 && page["entry_crc_verified"] != false {
				t.Fatal("partial entry incorrectly claims CRC validation")
			}
			offset = int(page["next_offset"].(float64))
		}
		var want []byte
		switch index {
		case 0:
			want = original
		case 1:
			want = []byte("name=synthetic")
		case 2:
			want = []byte{0, 255, 1}
		case -2:
			var err error
			want, err = os.ReadFile(filepath.Join(dir, "synthetic.sav"))
			if err != nil {
				t.Fatal(err)
			}
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("byte reconstruction differs for entry %d", index)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "never_extracted")); !os.IsNotExist(err) {
		t.Fatal("archive entry extracted to filesystem")
	}
}

func TestRawSaveTreeDuplicateKeysNestedListsAndSearchPagination(t *testing.T) {
	source := []byte(`same=first same=second deep={ { values={ "alpha" { leaf="needle" } } } { marker=needle } } long="` + strings.Repeat("x", 600) + `" empty={}`)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", source}})
	revision := rawRevision(t)
	root := rawCall(t, handleReadSaveTree, map[string]any{"revision": revision, "limit": 1})
	if root["node"].(map[string]any)["children_count"] != float64(5) || root["next_offset"] != float64(1) {
		t.Fatal(root)
	}
	first := root["children"].([]any)[0].(map[string]any)
	secondPage := rawCall(t, handleReadSaveTree, map[string]any{"revision": revision, "offset": 1, "limit": 1})
	second := secondPage["children"].([]any)[0].(map[string]any)
	if first["key"] != "same" || second["key"] != "same" || first["path"] != "/0" || second["path"] != "/1" {
		t.Fatal("duplicate keys lost their positions")
	}
	leaf := rawCall(t, handleReadSaveTree, map[string]any{"revision": revision, "path": "/2/0/0/1/0"})["node"].(map[string]any)
	if leaf["kind"] != "scalar" || leaf["scalar_preview"] != "needle" {
		t.Fatal(leaf)
	}
	long := rawCall(t, handleReadSaveTree, map[string]any{"revision": revision, "path": "/3"})["node"].(map[string]any)
	if long["scalar_bytes"] != float64(600) || long["scalar_preview_truncated"] != true || len(long["scalar_preview"].(string)) != rawScalarPreviewBytes {
		t.Fatal("scalar preview lost its truncation metadata")
	}
	search := rawCall(t, handleSearchSaveTree, map[string]any{"revision": revision, "query": "NEEDLE", "limit": 1})
	if search["total_matches"] != float64(2) || search["next_offset"] != float64(1) {
		t.Fatal(search)
	}
	search2 := rawCall(t, handleSearchSaveTree, map[string]any{"revision": revision, "query": "needle", "offset": 1, "limit": 1})
	if search2["next_offset"] != nil || search["matches"].([]any)[0].(map[string]any)["path"] == search2["matches"].([]any)[0].(map[string]any)["path"] {
		t.Fatal("search pages repeat or omit results")
	}
	seen := map[string]bool{}
	offset := 0
	total := 0
	for {
		page := rawCall(t, handleSearchSaveTree, map[string]any{"revision": revision, "offset": offset, "limit": 2})
		total = int(page["total_matches"].(float64))
		for _, match := range page["matches"].([]any) {
			path := match.(map[string]any)["path"].(string)
			if seen[path] {
				t.Fatalf("duplicate path %s", path)
			}
			seen[path] = true
		}
		if page["next_offset"] == nil {
			break
		}
		offset = int(page["next_offset"].(float64))
	}
	if len(seen) != total || !seen["/2/0/0/1/0"] {
		t.Fatal("all-node pagination missed descendants")
	}
}

func TestRawSaveSearchHasNoFixedDepthCutoff(t *testing.T) {
	depth := 300
	source := strings.Repeat("nested={", depth) + "leaf=target" + strings.Repeat("}", depth)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	page := rawCall(t, handleSearchSaveTree, map[string]any{"revision": rawRevision(t), "query": "target"})
	matches := page["matches"].([]any)
	if len(matches) != 1 || strings.Count(matches[0].(map[string]any)["path"].(string), "/") != depth+1 {
		t.Fatal("deep descendant omitted")
	}
}

func TestRawSaveErrorsAreExplicit(t *testing.T) {
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte("a=one")}, {"directory/", nil}})
	revision := rawRevision(t)
	cases := []struct {
		handler rawHandler
		args    map[string]any
	}{
		{handleReadSaveBytes, map[string]any{}},
		{handleReadSaveBytes, map[string]any{"revision": "stale"}},
		{handleReadSaveBytes, map[string]any{"revision": revision, "offset": -1}},
		{handleReadSaveBytes, map[string]any{"revision": revision, "limit": 65537}},
		{handleReadSaveBytes, map[string]any{"revision": revision, "offset": "oops"}},
		{handleReadSaveBytes, map[string]any{"revision": revision, "offset": 0.5}},
		{handleReadSaveBytes, map[string]any{"revision": revision, "entry_index": 999}},
		{handleReadSaveBytes, map[string]any{"revision": revision, "entry_index": 1}},
		{handleReadSaveTree, map[string]any{"revision": revision, "path": "../escape"}},
		{handleReadSaveTree, map[string]any{"revision": revision, "path": "/01"}},
		{handleReadSaveTree, map[string]any{"revision": revision, "path": "/0/0"}},
		{handleReadSaveTree, map[string]any{"revision": revision, "path": true}},
		{handleReadSaveTree, map[string]any{"revision": revision, "entry_index": -2}},
		{handleSearchSaveTree, map[string]any{"revision": revision, "offset": 99}},
	}
	for i, tc := range cases {
		r, err := tc.handler(context.Background(), rawRequest(tc.args))
		if err != nil || !r.IsError {
			t.Fatalf("case %d: expected tool error", i)
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte("changed=yes")}})
	r, err := handleReadSaveBytes(context.Background(), rawRequest(map[string]any{"revision": revision}))
	if err != nil || !r.IsError || !strings.Contains(resultText(r), "selected save changed") {
		t.Fatal("stale revision accepted")
	}
}

func TestRawSaveUnsupportedTreeStillHasExactBytes(t *testing.T) {
	source := []byte(`broken={key=}`)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", source}})
	revision := rawRevision(t)
	r, err := handleReadSaveTree(context.Background(), rawRequest(map[string]any{"revision": revision}))
	if err != nil || !r.IsError || !strings.Contains(resultText(r), "read_save_bytes") {
		t.Fatal("unsupported mixed syntax not explained")
	}
	page := rawCall(t, handleReadSaveBytes, map[string]any{"revision": revision})
	got, err := base64.StdEncoding.DecodeString(page["data"].(string))
	if err != nil || !bytes.Equal(got, source) {
		t.Fatal("unsupported source not accessible as exact bytes")
	}
}

func TestRawSaveDuplicateArchiveNamesNeedIndex(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte("first=yes")}, {"gamestate", []byte("second=yes")}})
	revision := rawRevision(t)
	r, err := handleReadSaveBytes(context.Background(), rawRequest(map[string]any{"revision": revision}))
	if err != nil || !r.IsError {
		t.Fatal("ambiguous entry selected silently")
	}
	page := rawCall(t, handleReadSaveBytes, map[string]any{"revision": revision, "entry_index": 1})
	got, err := base64.StdEncoding.DecodeString(page["data"].(string))
	if err != nil || string(got) != "second=yes" {
		t.Fatal("explicit duplicate entry index failed")
	}
}

func TestRawSaveLocalIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for local read-only integration")
	}
	old := saveDir
	saveDir = dir
	t.Cleanup(func() { saveDir = old })
	// Never log game values or exact bytes from the user's save.
	archive, err := openRawSave(dir)
	if err != nil {
		t.Fatal("opening local integration save failed")
	}
	parsedEntries, totalRootChildren := 0, 0
	for i, f := range archive.zip.File {
		if f.Name != "gamestate" && f.Name != "meta" {
			continue
		}
		args := map[string]any{"revision": archive.revision, "entry_index": i, "limit": 1}
		r, err := handleReadSaveTree(context.Background(), rawRequest(args))
		if err != nil || r.IsError {
			t.Fatal("local entry tree parsing failed")
		}
		var page map[string]any
		if json.Unmarshal([]byte(resultText(r)), &page) != nil {
			t.Fatal("local tree response decoding failed")
		}
		total := int(page["node"].(map[string]any)["children_count"].(float64))
		totalRootChildren += total
		if len(page["children"].([]any)) > 1 {
			t.Fatal("local tree page exceeded limit")
		}
		if total > 1 && page["next_offset"] != float64(1) {
			t.Fatal("local tree continuation missing")
		}
		if f.Name == "gamestate" {
			started := time.Now()
			r, err := handleSearchSaveTree(context.Background(), rawRequest(map[string]any{"revision": archive.revision, "entry_index": i, "query": "", "limit": 1}))
			elapsed := time.Since(started)
			if err != nil || r.IsError {
				t.Fatal("local exhaustive search failed")
			}
			var search map[string]any
			if json.Unmarshal([]byte(resultText(r)), &search) != nil {
				t.Fatal("local search response decoding failed")
			}
			if search["visited_nodes"] != search["total_matches"] || len(search["matches"].([]any)) != 1 {
				t.Fatal("local exhaustive search omitted nodes")
			}
			r, err = handleSearchSaveTree(context.Background(), rawRequest(map[string]any{"revision": archive.revision, "entry_index": i, "query": "", "offset": 1, "limit": 1}))
			if err != nil || r.IsError {
				t.Fatal("local search pagination failed")
			}
			var next map[string]any
			if json.Unmarshal([]byte(resultText(r)), &next) != nil {
				t.Fatal("local search continuation decoding failed")
			}
			if next["offset"] != float64(1) || next["total_matches"] != search["total_matches"] {
				t.Fatal("local search continuation unstable")
			}
			current, err := openRawSave(dir)
			if err != nil || current.revision != archive.revision {
				t.Fatal("local archive revision changed during integration")
			}
			t.Logf("local exhaustive tree search: nodes=%.0f duration=%s; pagination and archive revision stable", search["visited_nodes"].(float64), elapsed)
		}
		parsedEntries++
		// Read only a bounded fragment into the test process; do not print it.
		r, err = handleReadSaveBytes(context.Background(), rawRequest(map[string]any{"revision": archive.revision, "entry_index": i, "limit": 17}))
		if err != nil || r.IsError {
			t.Fatal("bounded local byte read failed")
		}
	}
	if parsedEntries == 0 {
		t.Fatal("no supported text entries found")
	}
	t.Logf("local archive entries=%d parsed_text_entries=%d root_children=%d; bounded byte/tree checks passed", len(archive.zip.File), parsedEntries, totalRootChildren)
}

func TestRawSaveCanceledSearch(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte("a={b=c}")}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := handleSearchSaveTree(ctx, rawRequest(map[string]any{"revision": rawRevision(t)}))
	if err != nil || !r.IsError {
		t.Fatal("canceled search did not return error")
	}
}

func TestRawSaveScalarPaginationPreservesMultibyteBytes(t *testing.T) {
	want := []byte(strings.Repeat("日本語", 80) + "\nend")
	source := []byte(`long="` + strings.Repeat("日本語", 80) + `\nend"`)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", source}})
	revision := rawRevision(t)
	var got []byte
	offset := 0
	for {
		page := rawCall(t, handleReadSaveScalar, map[string]any{"revision": revision, "path": "/0", "offset": offset, "limit": 7})
		chunk, err := base64.StdEncoding.DecodeString(page["data"].(string))
		if err != nil {
			t.Fatal(err)
		}
		if len(chunk) > 7 || page["total_bytes"] != float64(len(want)) {
			t.Fatal("scalar byte bounds incorrect")
		}
		got = append(got, chunk...)
		if page["next_offset"] == nil {
			break
		}
		offset = int(page["next_offset"].(float64))
	}
	if !bytes.Equal(got, want) {
		t.Fatal("normalized scalar bytes changed at multibyte boundaries")
	}
	r, err := handleReadSaveScalar(context.Background(), rawRequest(map[string]any{"revision": revision, "path": ""}))
	if err != nil || !r.IsError {
		t.Fatal("non-scalar node accepted")
	}
}

func TestRawSaveLongKeyPagination(t *testing.T) {
	key := strings.Repeat("鍵", 120)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`"` + key + `"=value`)}})
	revision := rawRevision(t)
	var got []byte
	for offset := 0; ; {
		page := rawCall(t, handleReadSaveScalar, map[string]any{"revision": revision, "path": "/0", "part": "key", "offset": offset, "limit": 7})
		chunk, err := base64.StdEncoding.DecodeString(page["data"].(string))
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, chunk...)
		if page["next_offset"] == nil {
			break
		}
		offset = int(page["next_offset"].(float64))
	}
	if string(got) != key {
		t.Fatal("key bytes lost")
	}
	r, err := handleReadSaveScalar(context.Background(), rawRequest(map[string]any{"revision": revision, "path": "", "part": "key"}))
	if err != nil || !r.IsError {
		t.Fatal("root without a key accepted")
	}
}

func TestRawSaveDetectsEntryChecksumFailure(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, err := z.CreateHeader(&zip.FileHeader{Name: "gamestate", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("synthetic_payload")); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	start, err := zr.File[0].DataOffset()
	if err != nil {
		t.Fatal(err)
	}
	data[int(start)+3] ^= 1
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.sav"), data, 0600); err != nil {
		t.Fatal(err)
	}
	old := saveDir
	saveDir = dir
	t.Cleanup(func() { saveDir = old })
	revision := rawRevision(t)
	r, err := handleReadSaveBytes(context.Background(), rawRequest(map[string]any{"revision": revision}))
	if err != nil || !r.IsError {
		t.Fatal("entry checksum failure ignored")
	}
	// The ZIP container remains accessible exactly, regardless of member CRC validity.
	rawCall(t, handleReadSaveBytes, map[string]any{"revision": revision, "entry_index": -2, "limit": 7})
}

func TestRawSaveMixedTreeSourceRanges(t *testing.T) {
	source := []byte("# leading comment\nroot={ duplicate=one anonymous { inner=yes } duplicate=\"two\\nlines\" \"quoted key\"=\"value\" empty={} } # tail")
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", source}})
	revision := rawRevision(t)
	page := rawCall(t, handleReadSaveTree, map[string]any{"revision": revision, "path": "/0"})
	if page["node"].(map[string]any)["kind"] != "mixed" {
		t.Fatal("mixed block not represented")
	}
	children := page["children"].([]any)
	if len(children) != 6 {
		t.Fatalf("mixed children lost: %d", len(children))
	}
	if _, ok := children[1].(map[string]any)["key"]; ok {
		t.Fatal("anonymous scalar acquired a key")
	}
	if children[2].(map[string]any)["kind"] != "object" {
		t.Fatal("anonymous nested block lost")
	}
	node := children[3].(map[string]any)
	if node["key"] != "duplicate" || node["scalar_quoted"] != true || node["scalar_escaped"] != true {
		t.Fatal("duplicate or token spelling lost")
	}
	start, size := int(node["value_source_offset"].(float64)), int(node["value_source_bytes"].(float64))
	exact := rawCall(t, handleReadSaveBytes, map[string]any{"revision": revision, "offset": start, "limit": size})
	fragment, err := base64.StdEncoding.DecodeString(exact["data"].(string))
	if err != nil || string(fragment) != `"two\nlines"` {
		t.Fatal("source range does not select exact quoted source")
	}
	normalized := rawCall(t, handleReadSaveScalar, map[string]any{"revision": revision, "path": "/0/3"})
	decoded, err := base64.StdEncoding.DecodeString(normalized["data"].(string))
	if err != nil || string(decoded) != "two\nlines" {
		t.Fatal("escaped text normalization incorrect")
	}
	if children[4].(map[string]any)["key_quoted"] != true || children[5].(map[string]any)["kind"] != "empty_block" {
		t.Fatal("quoted key or empty block distinction lost")
	}
}

func TestRawSaveSelectsCatalogSaveID(t *testing.T) {
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte("selected=first")}})
	writeSyntheticSave(t, filepath.Join(dir, "second.sav"), []syntheticEntry{{"gamestate", []byte("selected=second")}})
	files, err := gamestate.ListSaveFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, f := range files {
		if f.RelativePath == "synthetic.sav" {
			id = f.ID
		}
	}
	if id == "" {
		t.Fatal("catalog ID missing")
	}
	archive := rawCall(t, handleInspectSaveArchive, map[string]any{"save_id": id})
	if archive["save_id"] != id {
		t.Fatal("selection ID not returned")
	}
	page := rawCall(t, handleReadSaveScalar, map[string]any{"save_id": id, "revision": archive["revision"], "path": "/0"})
	data, err := base64.StdEncoding.DecodeString(page["data"].(string))
	if err != nil || string(data) != "first" || page["save_id"] != id {
		t.Fatal("selected save not read")
	}
	r, err := handleInspectSaveArchive(context.Background(), rawRequest(map[string]any{"save_id": "../outside.sav"}))
	if err != nil || !r.IsError {
		t.Fatal("untrusted path accepted as save ID")
	}
}
