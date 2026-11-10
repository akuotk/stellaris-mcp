package tools

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

func TestRawSearchSubtreeOrderAndContinuation(t *testing.T) {
	source := `table={dup=hit {dup=hit} dup={inner=hit anonymous} "Ä"="NEEDLE" wide={` +
		strings.Repeat("pad=skip ", 11) + "match=hit}} outside=hit"
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	revision := rawRevision(t)
	all := []string{"/0", "/0/0", "/0/1", "/0/1/0", "/0/2", "/0/2/0", "/0/2/1", "/0/3", "/0/4"}
	for i := range 12 {
		all = append(all, "/0/4/"+strconv.Itoa(i))
	}
	for _, tc := range []struct {
		query string
		paths []string
	}{
		{"", all},
		{"HIT", []string{"/0/0", "/0/1/0", "/0/2/0", "/0/4/11"}},
		{"ä", []string{"/0/3"}},
	} {
		args := map[string]any{"revision": revision, "path": "/0", "query": tc.query, "limit": 2}
		page := rawCall(t, handleSearchSaveTree, args)
		token := page["page_context"]
		for i := 0; ; {
			if page["visited_nodes"] != float64(len(all)) || page["total_matches"] != float64(len(tc.paths)) ||
				page["offset"] != float64(i) || page["page_context"] != token || page["pagination_context_verified"] != true {
				t.Fatalf("search metadata changed for query %q", tc.query)
			}
			matches := page["matches"].([]any)
			if len(matches) == 0 {
				t.Fatalf("empty search page for query %q", tc.query)
			}
			for _, value := range matches {
				match := value.(map[string]any)
				if i >= len(tc.paths) || match["path"] != tc.paths[i] {
					t.Fatalf("wrong search order or path at match %d for query %q", i, tc.query)
				}
				node := rawCall(t, handleReadSaveTree, map[string]any{"revision": revision, "path": tc.paths[i]})["node"]
				if !reflect.DeepEqual(match, node) {
					t.Fatalf("search node differs from raw tree at %s", tc.paths[i])
				}
				i++
			}
			if page["next_offset"] == nil {
				if i != len(tc.paths) {
					t.Fatalf("search stopped after %d matches for query %q", i, tc.query)
				}
				break
			}
			if page["next_offset"] != float64(i) || i >= len(tc.paths) {
				t.Fatalf("incorrect continuation for query %q", tc.query)
			}
			args["offset"], args["page_context"], args["limit"], args["query"] = i, token, 3, strings.ToLower(tc.query)
			page = rawCall(t, handleSearchSaveTree, args)
		}
	}
}

func TestRawSearchCancelsDuringTraversal(t *testing.T) {
	root, err := clausewitz.ParseRaw(context.Background(), []byte("table={dup=one {dup=two} duplicate={value=three}}"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checking := &rawReadCancelContext{Context: ctx, cancel: cancel, checks: 4}
	rows, total, visited, err := indexedRawSearchPage(checking, root, nil, "", "", 0, 1)
	if err != context.Canceled || rows != nil || total != 3 || visited != 3 {
		t.Fatal("traversal did not stop on cancellation after visiting three nodes")
	}
}
