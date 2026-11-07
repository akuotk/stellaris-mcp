package tools

import (
	"context"
	"testing"
)

func TestRawPaginationContextAndBackwardCompatibility(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`a=needle b=needle block={child=needle}`)}, {"meta", []byte(`x=value`)}})
	rev := rawRevision(t)
	cases := []struct {
		name                   string
		fn                     rawHandler
		first, second, changed map[string]any
	}{
		{"bytes", handleReadSaveBytes, map[string]any{"revision": rev, "limit": 1}, map[string]any{"revision": rev, "offset": 1, "limit": 2}, map[string]any{"revision": rev, "entry_index": 1, "offset": 1}},
		{"tree", handleReadSaveTree, map[string]any{"revision": rev, "limit": 1}, map[string]any{"revision": rev, "offset": 1, "limit": 2}, map[string]any{"revision": rev, "path": "/2", "offset": 0}},
		{"scalar", handleReadSaveScalar, map[string]any{"revision": rev, "path": "/0", "limit": 1}, map[string]any{"revision": rev, "path": "/0", "offset": 1, "limit": 2}, map[string]any{"revision": rev, "path": "/1", "offset": 1}},
		{"search", handleSearchSaveTree, map[string]any{"revision": rev, "query": "NEEDLE", "limit": 1}, map[string]any{"revision": rev, "query": "needle", "offset": 1, "limit": 2}, map[string]any{"revision": rev, "query": "other", "offset": 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			first := rawCall(t, c.fn, c.first)
			token, ok := first["page_context"].(string)
			if !ok || token == "" || first["pagination_context_verified"] != true {
				t.Fatal("first page missing context")
			}
			legacy := rawCall(t, c.fn, c.second)
			if legacy["pagination_context_verified"] != false {
				t.Fatal("legacy nonzero offset presented as verified")
			}
			c.second["page_context"] = token
			next := rawCall(t, c.fn, c.second)
			if next["pagination_context_verified"] != true {
				t.Fatal("matching context not verified")
			}
			c.changed["page_context"] = token
			r, err := c.fn(context.Background(), rawRequest(c.changed))
			if err != nil || !r.IsError {
				t.Fatal("changed context accepted")
			}
			r, err = c.fn(context.Background(), rawRequest(map[string]any{"revision": rev, "page_context": true}))
			if err != nil || !r.IsError {
				t.Fatal("non-string context accepted")
			}
		})
	}
}
