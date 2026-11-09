package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func galaxyRegionText(t *testing.T, args map[string]any) string {
	t.Helper()
	result, err := handleGalaxyRegions(context.Background(), rawRequest(args))
	if err != nil || result.IsError {
		t.Fatalf("galaxy region read failed: %v %v", result, err)
	}
	return resultText(result)
}

func requireGalaxyRegionOrder(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		at := strings.Index(got, want)
		if at < 0 {
			t.Fatalf("missing or out-of-order output %q", want)
		}
		got = got[at+len(want):]
	}
}

func TestGalaxyRegionsSavedShapesAndCandidates(t *testing.T) {
	const fixture = `
galactic_object={0={} 7={} 7={} 8=none unknown={}}
planets={planet={0={} 7={} 8={} 10={} 11=bad}}
clusters={{id=synthetic id=repeated position={x=1 y=2 origin=7} radius=5 objects={0 7 0 bad {id=0} 8 10 11 44} objects={0}} {id=second objects={0}} {id=missing} {objects={}} invalid}
nebula={coordinate={x=1 y=2} galactic_object=0 galactic_object=7 galactic_object={0 8} galactic_object=0}
nebula={galactic_object=0}
clusters={}
clusters={keyed={} loose}
clusters={named={objects={0}}}
nebula=invalid`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})

	parents := galaxyRegionText(t, nil)
	requireGalaxyRegionOrder(t, parents,
		"kind=clusters root_occurrences=4 saved_entries=8 offset=0",
		`entry[0] root[2] key="clusters"/item[0]: objects_fields=2 child_kind=cluster_objects child_offset=0 child_entries=10`,
		`field[0] key="id": "synthetic"`,
		`field[1] key="id" [duplicate saved key]: "repeated"`,
		`Field 2 key="origin": "7"`,
		`field[3] key="radius": "5"`,
		`entry[1] root[2] key="clusters"/item[1]: objects_fields=1 child_kind=cluster_objects child_offset=10 child_entries=1`,
		`entry[2] root[2] key="clusters"/item[2]: objects_fields=0`,
		`entry[5] root[5] key="clusters"`,
		"empty-root: empty object",
		`entry[6] root[6] key="clusters"`,
		"unsupported-root: mixed saved block",
		`entry[7] root[7] key="clusters"`,
		"unsupported-root: object")
	if strings.Contains(parents, `"bad" (saved scalar)`) || strings.Contains(parents, "occurrences_in_parent") {
		t.Fatal("parent view expanded object entries")
	}

	requireGalaxyRegionOrder(t, galaxyRegionText(t, map[string]any{"kind": "cluster_objects"}),
		"kind=cluster_objects root_occurrences=4 saved_entries=17 offset=0",
		"Candidate tables: system=incomplete planet=recorded country=not-recorded",
		`entry[0] root[2] key="clusters"/item[0]/field[4] key="objects"/item[0]: "0"`,
		"occurrences_in_parent=3 parents_with_id=2 system=candidate planet=candidate country=table-not-recorded",
		`/field[4] key="objects"/item[1]: "7"`,
		"system=duplicate-key planet=candidate",
		`/field[4] key="objects"/item[3] (unsupported-id): "bad"`,
		`/field[4] key="objects"/item[4] (unsupported-id): object`,
		`/field[4] key="objects"/item[5]: "8"`,
		"system=unsupported-record planet=candidate",
		`/field[4] key="objects"/item[6]: "10"`,
		"system=incomplete-table planet=candidate",
		`/field[4] key="objects"/item[7]: "11"`,
		"system=incomplete-table planet=unsupported-record",
		`/field[4] key="objects"/item[8]: "44"`,
		"system=incomplete-table planet=no-candidate",
		`/field[5] key="objects"/item[0]: "0"`,
		`entry[10] root[2] key="clusters"/item[1]/field[1] key="objects"/item[0]: "0"`,
		"occurrences_in_parent=1 parents_with_id=2",
		`entry[11] root[2] key="clusters"/item[2]: object field not recorded`,
		`entry[12] root[2] key="clusters"/item[3]/field[0] key="objects" (empty-list): empty object`,
		`entry[13] root[2] key="clusters"/item[4] (unsupported-region): "invalid"`)

	requireGalaxyRegionOrder(t, galaxyRegionText(t, map[string]any{"kind": "nebulae"}),
		"kind=nebulae root_occurrences=3 saved_entries=3 offset=0",
		`entry[0] root[3] key="nebula": galactic_object_fields=4 child_kind=nebula_objects child_offset=0 child_entries=4`,
		`Field 0 key="x": "1"`,
		`entry[1] root[4] key="nebula": galactic_object_fields=1 child_kind=nebula_objects child_offset=4 child_entries=1`,
		`entry[2] root[8] key="nebula"`,
		`unsupported-region: "invalid"`)
	requireGalaxyRegionOrder(t, galaxyRegionText(t, map[string]any{"kind": "nebula_objects"}),
		"kind=nebula_objects root_occurrences=3 saved_entries=6 offset=0",
		`entry[0] root[3] key="nebula"/field[1] key="galactic_object": "0"`,
		"occurrences_in_parent=2 parents_with_id=2 system=candidate planet=candidate",
		`entry[1] root[3] key="nebula"/field[2] key="galactic_object": "7"`,
		`entry[2] root[3] key="nebula"/field[3] key="galactic_object" (unsupported-id): list with 2 saved entries`,
		`entry[3] root[3] key="nebula"/field[4] key="galactic_object": "0"`,
		`entry[4] root[4] key="nebula"/field[0] key="galactic_object": "0"`,
		`entry[5] root[8] key="nebula" (unsupported-region): "invalid"`)
}

func TestGalaxyRegionsAbsentAndBoundedFallback(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte("other=1")}})
		for _, kind := range []string{"clusters", "cluster_objects", "nebulae", "nebula_objects"} {
			requireGalaxyRegionOrder(t, galaxyRegionText(t, map[string]any{"kind": kind}), "root_occurrences=0 saved_entries=0", "next_offset=none")
		}
	})
	t.Run("large and mixed", func(t *testing.T) {
		large := strings.Repeat(" private-marker", 50)
		source := "galactic_object={0={}} galactic_object={0={}} clusters={{objects={0} objects={key=0 stray} objects=wrong objects={" +
			strings.Repeat(" 9283", 50) + "}}} nebula={coordinate={extra={" + large + "}} galactic_object={" + large + "}}"
		syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
		children := galaxyRegionText(t, map[string]any{"kind": "cluster_objects"})
		requireGalaxyRegionOrder(t, children,
			"system=incomplete",
			"system=incomplete-table",
			"(unsupported-list): mixed saved block",
			`(unsupported-list): "wrong"`,
			"entry[19]", "next_offset=20")
		if strings.Contains(children, "entry[20]") || strings.Contains(galaxyRegionText(t, nil), "9283") {
			t.Fatal("large object list expanded beyond its page")
		}
		for _, kind := range []string{"nebulae", "nebula_objects"} {
			got := galaxyRegionText(t, map[string]any{"kind": kind})
			requireGalaxyRegionOrder(t, got, "large saved block; use raw tree/byte tools")
			if strings.Contains(got, "private-marker") {
				t.Fatal("large fallback expanded its payload")
			}
		}
	})
}

func TestGalaxyRegionsPagination(t *testing.T) {
	var clusters, nebulae strings.Builder
	clusters.WriteString("clusters={")
	for i := 0; i < 22; i++ {
		fmt.Fprintf(&clusters, "{id=%d objects={%d}} ", i, i)
		fmt.Fprintf(&nebulae, "nebula={galactic_object=%d} ", i)
	}
	source := clusters.String() + "} " + nebulae.String()
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	s := server.NewMCPServer("test", "test")
	Register(s, dir, loc)
	entry, ok := s.ListTools()["get_galaxy_regions"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only registration")
	}

	var continuation map[string]any
	for _, kind := range []string{"clusters", "cluster_objects", "nebulae", "nebula_objects"} {
		first := galaxyRegionText(t, map[string]any{"kind": kind})
		requireGalaxyRegionOrder(t, first, "saved_entries=22 offset=0", "entry[19]", "next_offset=20")
		if strings.Contains(first, "entry[20]") {
			t.Fatal("page exceeds 20 entries")
		}
		continuation = map[string]any{"kind": kind, "offset": 20}
		for _, line := range strings.Split(first, "\n") {
			key, value, ok := strings.Cut(line, "=")
			if ok && (key == "save_id" || key == "page_context") {
				continuation[key] = value
			}
		}
		if continuation["save_id"] == nil || continuation["save_id"] == "" || continuation["page_context"] == nil || continuation["page_context"] == "" {
			t.Fatal("missing continuation context")
		}
		requireGalaxyRegionOrder(t, galaxyRegionText(t, continuation), "saved_entries=22 offset=20", "entry[20]", "entry[21]", "next_offset=none")
	}
	for _, args := range []map[string]any{
		{"kind": "nebulae", "offset": 20, "page_context": continuation["page_context"]},
		{"kind": "nebula_objects", "offset": 23, "page_context": continuation["page_context"]},
		{"offset": 20},
		{"offset": 0.5},
		{"kind": "invalid"},
		{"kind": 0},
		{"save_id": 0},
		{"page_context": 0},
	} {
		result, err := handleGalaxyRegions(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Errorf("accepted invalid request %v", args)
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(source + " changed=yes")}})
	result, err := handleGalaxyRegions(context.Background(), rawRequest(continuation))
	if err != nil || !result.IsError || !strings.Contains(resultText(result), "page_context") {
		t.Fatal("accepted stale continuation")
	}
}
