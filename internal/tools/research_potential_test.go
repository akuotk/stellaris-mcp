package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestResearchPotentialPaginationAndRevision(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={country=0} country={0={tech_status={auto_researching_physics=no potential={tech_alpha="7" tech_alpha="9" tech_complex={nested=yes}}}}}`)}})
	first := rawCall(t, handleResearchPotential, map[string]any{"limit": 1})
	if first["total_matches"] != float64(3) || first["next_offset"] != float64(1) || first["save_id"] == "" {
		t.Fatal(first)
	}
	entry := first["entries"].([]any)[0].(map[string]any)
	if entry["key"] != "tech_alpha" || entry["raw_value"] != "7" {
		t.Fatal(entry)
	}
	second := rawCall(t, handleResearchPotential, map[string]any{"save_id": first["save_id"], "revision": first["revision"], "page_context": first["page_context"], "offset": 1, "limit": 2})
	items := second["entries"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["raw_value"] != "9" || items[1].(map[string]any)["kind"] != "object" || items[1].(map[string]any)["raw_value"] != nil {
		t.Fatal(second)
	}
	r, err := handleResearchPotential(context.Background(), rawRequest(map[string]any{"offset": 1}))
	if err != nil || !r.IsError || !strings.Contains(resultText(r), "revision") {
		t.Fatal("missing revision accepted")
	}
	r, err = handleResearchPotential(context.Background(), rawRequest(map[string]any{"offset": 1, "revision": strings.Repeat("0", 64)}))
	if err != nil || !r.IsError {
		t.Fatal("stale revision accepted")
	}
	r, err = handleResearchPotential(context.Background(), rawRequest(map[string]any{"offset": 1, "revision": first["revision"], "page_context": first["page_context"], "query": "different"}))
	if err != nil || !r.IsError {
		t.Fatal("changed query accepted")
	}
}
