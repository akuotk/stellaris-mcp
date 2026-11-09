package tools

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func technologyTreeHandler(source gamestate.TechnologyDefinitionSource) rawHandler {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleTechnologyDefinitionTree(ctx, req, source)
	}
}

func TestTechnologyDefinitionTreeOrderedNavigation(t *testing.T) {
	const script = `tech_sample={dup=first dup=second deep={a={b={c={anonymous named={value=deep}}}}}} tech_sample={tier=2}`
	_, locale := technologyGameFixture(t, script)
	oldDir, oldLoc := saveDir, loc
	t.Cleanup(func() { saveDir, loc = oldDir, oldLoc })
	s := server.NewMCPServer("test", "test")
	RegisterWithInstalledScripts(s, "", loc, locale)
	entry, ok := s.ListTools()["read_technology_definition"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only registration")
	}
	if _, exists := entry.Tool.InputSchema.Properties["save_id"]; exists {
		t.Fatal("static reader advertises save selection")
	}
	call := rawHandler(entry.Handler)
	ambiguous := rawCall(t, call, map[string]any{"technology_id": "tech_sample"})
	if ambiguous["status"] != "ambiguous" || ambiguous["match_status"] != "multiple" ||
		ambiguous["definition_matches"] != float64(2) {
		t.Fatalf("unexpected ambiguous response %v", ambiguous)
	}
	first := rawCall(t, call, map[string]any{"technology_id": "tech_sample", "definition_index": 0, "limit": 1})
	children := first["children"].([]any)
	child := children[0].(map[string]any)
	if len(children) != 1 || child["path"] != "/0" || child["key"] != "dup" || child["scalar_preview"] != "first" ||
		first["next_offset"] != float64(1) || first["exact_source_bytes_available"] != false {
		t.Fatalf("unexpected first page %v", first)
	}
	if _, exists := child["value_source_offset"]; exists {
		t.Fatal("reader advertised a source byte range")
	}
	second := rawCall(t, call, map[string]any{"technology_id": "tech_sample", "definition_index": 0,
		"offset": 1, "limit": 1, "page_context": first["page_context"]})
	child = second["children"].([]any)[0].(map[string]any)
	if child["path"] != "/1" || child["key"] != "dup" || child["scalar_preview"] != "second" {
		t.Fatalf("duplicate key lost %v", child)
	}
	mixed := rawCall(t, call, map[string]any{"technology_id": "tech_sample", "definition_index": 0, "path": "/2/0/0/0"})
	nodes := mixed["children"].([]any)
	if mixed["node"].(map[string]any)["kind"] != "mixed" || len(nodes) != 2 ||
		nodes[0].(map[string]any)["scalar_preview"] != "anonymous" || nodes[1].(map[string]any)["key"] != "named" {
		t.Fatalf("mixed child order lost %v", mixed)
	}
	deep := rawCall(t, call, map[string]any{"technology_id": "tech_sample", "definition_index": 0, "path": "/2/0/0/0/1/0"})
	if deep["node"].(map[string]any)["scalar_preview"] != "deep" || deep["total_children"] != float64(0) {
		t.Fatalf("deep scalar unavailable %v", deep)
	}
	other := rawCall(t, call, map[string]any{"technology_id": "tech_sample", "definition_index": 1})
	if other["children"].([]any)[0].(map[string]any)["scalar_preview"] != "2" {
		t.Fatalf("failed to select second definition %v", other)
	}
}

func TestTechnologyDefinitionTreeNormalizedBytesAndContexts(t *testing.T) {
	longKey := strings.Repeat("鍵", 101)
	longValue := strings.Repeat("日本語😀", 70) + "\n\"\\"
	script := "tech_sample={" + strconv.Quote(longKey) + "=" + strconv.Quote(longValue) + " other=short} tech_sample={a=1 b=2} tech_other={a=3 b=4}"
	root, locale := technologyGameFixture(t, script)
	source := gamestate.NewTechnologyDefinitionSource(locale)
	call := technologyTreeHandler(source)
	node := rawCall(t, call, map[string]any{"technology_id": "tech_sample", "definition_index": 0, "path": "/0"})["node"].(map[string]any)
	if node["key_preview_truncated"] != true || node["scalar_preview_truncated"] != true ||
		node["key_bytes"] != float64(len(longKey)) || node["scalar_bytes"] != float64(len(longValue)) ||
		node["scalar_quoted"] != true || node["scalar_escaped"] != true {
		t.Fatalf("unexpected preview metadata %v", node)
	}
	for _, fixture := range []struct{ part, want string }{{"key", longKey}, {"value", longValue}} {
		args := map[string]any{"technology_id": "tech_sample", "definition_index": 0, "path": "/0", "operation": "scalar", "part": fixture.part, "limit": 31}
		var data []byte
		for {
			page := rawCall(t, call, args)
			part, err := base64.StdEncoding.DecodeString(page["data"].(string))
			if err != nil {
				t.Fatal(err)
			}
			if page["encoding"] != "base64" || page["total_bytes"] != float64(len(fixture.want)) || len(part) > 31 {
				t.Fatal("invalid scalar page")
			}
			data = append(data, part...)
			if page["next_offset"] == nil {
				break
			}
			args["offset"], args["page_context"] = page["next_offset"], page["page_context"]
		}
		if string(data) != fixture.want {
			t.Fatalf("%s bytes did not round-trip", fixture.part)
		}
	}
	first := rawCall(t, call, map[string]any{"technology_id": "tech_sample", "definition_index": 0,
		"path": "/0", "operation": "scalar", "limit": 1})
	token := first["page_context"]
	for _, args := range []map[string]any{
		{"technology_id": "tech_sample", "definition_index": 0, "path": "/0", "operation": "scalar", "offset": 1},
		{"technology_id": "tech_sample", "definition_index": 0, "path": "/0", "operation": "scalar", "part": "key", "offset": 1, "page_context": token},
		{"technology_id": "tech_sample", "definition_index": 0, "path": "/0", "operation": "tree", "page_context": token},
		{"technology_id": "tech_sample", "definition_index": 0, "path": "/1", "operation": "scalar", "page_context": token},
		{"technology_id": "tech_sample", "definition_index": 1, "path": "/0", "operation": "scalar", "page_context": token},
		{"technology_id": "tech_other", "definition_index": 0, "path": "/0", "operation": "scalar", "page_context": token},
		{"technology_id": "tech_sample", "definition_index": 0, "path": "/0", "operation": "scalar", "offset": len(longValue) + 1, "page_context": token},
		{"technology_id": "tech_sample", "definition_index": 0, "path": "/00"},
		{"technology_id": "tech_sample", "definition_index": "0"},
		{"technology_id": "tech_sample", "definition_index": 0.5},
		{"technology_id": "tech_sample", "definition_index": -1},
		{"technology_id": "tech_sample", "definition_index": 0, "path": 0},
		{"technology_id": "tech_sample", "definition_index": 0, "page_context": false},
		{"technology_id": "tech_sample", "definition_index": 0, "offset": 0.5},
		{"technology_id": "tech_sample", "definition_index": 0, "limit": 201},
		{"technology_id": "tech_sample", "definition_index": 0, "operation": "scalar", "part": "wrong"},
	} {
		result, err := call(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Fatalf("accepted invalid synthetic query %v", args)
		}
	}
	writeTechnologyFixture(t, root, "common/technology/synthetic.txt", script+" extra={}")
	result, err := technologyTreeHandler(gamestate.NewTechnologyDefinitionSource(locale))(context.Background(), rawRequest(map[string]any{
		"technology_id": "tech_sample", "definition_index": 0, "path": "/0", "operation": "scalar", "offset": 1, "page_context": token,
	}))
	if err != nil || !result.IsError || !strings.Contains(resultText(result), "page_context") {
		t.Fatal("changed source accepted an old token")
	}
}

func TestTechnologyDefinitionTreeSourceStates(t *testing.T) {
	root, locale := technologyGameFixture(t, "tech_sample={tier=1}")
	query := map[string]any{"technology_id": "tech_missing"}
	missing := rawCall(t, technologyTreeHandler(gamestate.NewTechnologyDefinitionSource(locale)), query)
	if missing["status"] != "unmatched" || missing["source_status"] != "complete" {
		t.Fatalf("unexpected unmatched response %v", missing)
	}
	unavailable := rawCall(t, technologyTreeHandler(nil), query)
	if unavailable["status"] != "unavailable" {
		t.Fatalf("unexpected unavailable response %v", unavailable)
	}
	writeTechnologyFixture(t, root, "common/technology/partial.txt", "tech_broken={")
	writeTechnologyFixture(t, root, "common/technology/large.txt", "tech_large={array={"+strings.Repeat(" 0", 150001)+"}}")
	call := technologyTreeHandler(gamestate.NewTechnologyDefinitionSource(locale))
	partial := rawCall(t, call, query)
	if partial["status"] != "unresolved" || partial["source_status"] != "partial" {
		t.Fatalf("unexpected partial response %v", partial)
	}
	observed := rawCall(t, call, map[string]any{"technology_id": "tech_sample"})
	if observed["status"] != "ok" || observed["match_status"] != "unique-observed" {
		t.Fatalf("lost an observed definition %v", observed)
	}
	large := rawCall(t, call, map[string]any{"technology_id": "tech_large"})
	if large["status"] != "definition-not-retained" || large["definition_matches"] != float64(1) {
		t.Fatalf("unexpected retention response %v", large)
	}
}
