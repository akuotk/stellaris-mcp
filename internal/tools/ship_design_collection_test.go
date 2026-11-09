package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func shipDesignCollectionCall(t *testing.T, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := handleShipDesignCollection(context.Background(), rawRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestShipDesignCollectionOrderAndMatches(t *testing.T) {
	const fixture = `
player={country=0} player={country=0}
country={1={} 0={
	ship_design_collection={ metadata=unknown ship_design={00 1 2 3 0 bad {key=x anonymous}} ship_design=4 ship_design={} }
	ship_design_collection={key=x anonymous}
	ship_design_collection={}
}}
ship_design={0={name={key=fixture}} 1=none 2={} 02={} 4={}}
ships={
	0={ship_design_implementation={design=0}}
	1={ship_design=0 ship_design_implementation={design=0}}
	2={ship_design=1}
	3={ship_design_implementation={design=3}}
	4={ship_design=0 ship_design_implementation={design=1}}
	5={ship_design_implementation={design=0 design=0}}
	6={}
	7=none
	8={ship_design=0}
	08={ship_design=0}
}
construction={0={design=4}}
fleet_template={0={design=4}}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	result := shipDesignCollectionCall(t, nil)
	if result.IsError {
		t.Fatal("read failed")
	}
	got := resultText(result)
	technologyContains(t, got,
		"country_id=0",
		"collection_status=duplicate collection_occurrences=3 unsupported_collections=1 ship_design_fields=3 design_references=6 saved_entries=12",
		"design_table_status=recorded design_unindexed_keys=0 ship_table_status=partial ship_unresolved_records=5",
		"Construction, template and event references are outside this comparison.",
		`entry[0] path="/2/1/0/0" kind=collection-field key="metadata": "unknown"`,
		`entry[1] path="/2/1/0/1/0" kind=design-reference key="ship_design" repeated_ship_design_field=true design_id=0 saved_reference="00" design_status=unique repeated_saved_reference=true design_path="/3/0" observed_ship_reference_matches=2`,
		`entry[2] path="/2/1/0/1/1" kind=design-reference key="ship_design" repeated_ship_design_field=true design_id=1 saved_reference="1" design_status=nonobject design_path="/3/1" observed_ship_reference_matches=1`,
		`design_id=2 saved_reference="2" design_status=duplicate observed_ship_reference_matches=0`,
		`design_id=3 saved_reference="3" design_status=missing observed_ship_reference_matches=1`,
		`design_id=0 saved_reference="0" design_status=unique repeated_saved_reference=true design_path="/3/0" observed_ship_reference_matches=2`,
		`entry[6] path="/2/1/0/1/5" kind=unsupported-reference key="ship_design" repeated_ship_design_field=true: "bad"`,
		`entry[7] path="/2/1/0/1/6" kind=unsupported-reference key="ship_design" repeated_ship_design_field=true: mixed saved block`,
		`design_id=4 saved_reference="4" design_status=unique design_path="/3/4" observed_ship_reference_matches=0`,
		`entry[9] path="/2/1/0/3" kind=empty-design-list key="ship_design"`,
		`entry[10] path="/2/1/1" kind=unsupported-collection key="ship_design_collection": mixed saved block`,
		`entry[11] path="/2/1/2" kind=empty-collection key="ship_design_collection"`,
		"next_offset=none")
}

func TestShipDesignCollectionTableStates(t *testing.T) {
	for _, fixture := range []struct{ name, save, want, match string }{
		{"missing collection", `country={0={}}`, "collection_status=not-recorded", ""},
		{"empty collection", `country={0={ship_design_collection={}}}`, "collection_status=empty", ""},
		{"unsupported collection", `country={0={ship_design_collection=unexpected}}`, "collection_status=unsupported", ""},
		{"missing tables", `country={0={ship_design_collection={ship_design={0}}}}`, "design_table_status=not-recorded", "design_status=table-not-recorded ship_reference_matches=unavailable"},
		{"duplicate tables", `country={0={ship_design_collection={ship_design={0}}}} ship_design={0={}} ship_design={} ships={} ships={}`, "design_table_status=duplicate", "design_status=table-duplicate ship_reference_matches=unavailable"},
		{"unsupported tables", `country={0={ship_design_collection={ship_design={0}}}} ship_design={0 1} ships=unknown`, "design_table_status=unsupported", "design_status=table-unsupported ship_reference_matches=unavailable"},
		{"partial design table", `country={0={ship_design_collection={ship_design={0}}}} ship_design={unknown={}} ships={}`, "design_table_status=partial", "design_status=unresolved ship_reference_matches=0"},
		{"no ship matches", `country={0={ship_design_collection={ship_design={0}}}} ship_design={0={}} ships={1={ship_design=1}}`, "ship_table_status=recorded", "design_status=unique design_path=\"/1/0\" ship_reference_matches=0"},
		{"ship missing design references", `country={0={ship_design_collection={ship_design={0}}}} ship_design={0={}} ships={1={ship_design_implementation={growth_stage=1}}}`, "ship_table_status=partial ship_unresolved_records=1", "design_status=unique design_path=\"/1/0\" observed_ship_reference_matches=0"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture.save)}})
			result := shipDesignCollectionCall(t, map[string]any{"country_id": 0})
			if result.IsError {
				t.Fatal("lookup failed")
			}
			got := resultText(result)
			if !strings.Contains(got, fixture.want) || (fixture.match != "" && !strings.Contains(got, fixture.match)) {
				t.Fatalf("expected %q and %q", fixture.want, fixture.match)
			}
		})
	}
}

func TestShipDesignCollectionPagination(t *testing.T) {
	var b strings.Builder
	b.WriteString("player={{country=0} {country=0}} country={0={ship_design_collection={ship_design={")
	for i := 0; i < 23; i++ {
		fmt.Fprintf(&b, " %d", i)
	}
	b.WriteString("}}} 1={ship_design_collection={ship_design={0}}}} ship_design={0={}} ships={}")
	fixture := b.String()
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	s := server.NewMCPServer("test", "test")
	Register(s, dir, loc)
	entry, ok := s.ListTools()["get_ship_design_collection"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only registration")
	}
	first := shipDesignCollectionCall(t, nil)
	if first.IsError {
		t.Fatal("first page failed")
	}
	got := resultText(first)
	technologyContains(t, got, "saved_entries=23 offset=0", "entry[19]", "next_offset=20")
	if strings.Contains(got, "entry[20]") {
		t.Fatal("first page exceeds 20 entries")
	}
	var token, saveID string
	for _, line := range strings.Split(got, "\n") {
		if value, ok := strings.CutPrefix(line, "page_context="); ok {
			token = value
		}
		if value, ok := strings.CutPrefix(line, "save_id="); ok {
			saveID = value
		}
	}
	if token == "" || saveID == "" {
		t.Fatal("missing continuation context")
	}
	args := map[string]any{"country_id": 0, "save_id": saveID, "offset": 20, "page_context": token}
	second := shipDesignCollectionCall(t, args)
	if second.IsError {
		t.Fatal("second page failed")
	}
	technologyContains(t, resultText(second), "saved_entries=23 offset=20", "entry[20]", "entry[21]", "entry[22]", "next_offset=none")
	for _, invalid := range []map[string]any{
		{"country_id": 1, "offset": 20, "page_context": token},
		{"country_id": "0"}, {"country_id": -1}, {"country_id": 0.5},
		{"save_id": 0}, {"page_context": 0}, {"offset": 20}, {"offset": 24, "page_context": token},
	} {
		if !shipDesignCollectionCall(t, invalid).IsError {
			t.Fatalf("accepted invalid synthetic request %v", invalid)
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(fixture + " changed=yes")}})
	if stale := shipDesignCollectionCall(t, args); !stale.IsError || !strings.Contains(resultText(stale), "page_context") {
		t.Fatal("changed save revision accepted")
	}
}
