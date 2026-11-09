package tools

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func technologyReferenceFixture(t *testing.T) (root, locale string) {
	t.Helper()
	root, locale = technologyGameFixture(t, "")
	for _, category := range []string{
		"armies", "buildings", "component_sets", "component_templates", "decisions", "districts", "edicts",
		"megastructures", "section_templates", "ship_sizes", "starbase_buildings", "starbase_levels", "starbase_modules",
	} {
		if err := os.Mkdir(filepath.Join(root, "common", category), 0700); err != nil {
			t.Fatal(err)
		}
	}
	return root, locale
}

func technologyReferenceResult(t *testing.T, source gamestate.TechnologyReferenceSource, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := handleTechnologyReferences(context.Background(), rawRequest(args), source)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTechnologyReferencesLiteralScope(t *testing.T) {
	root, locale := technologyReferenceFixture(t)
	writeTechnologyFixture(t, root, "common/buildings/fixture.txt", `
building_sample={prerequisites={tech_sample tech_other "tech_sample"} prerequisites={tech_sample}}
building_sample={prerequisites={tech_other}}
building_mixed={prerequisites={tech_sample condition={has_technology=tech_nested}}}
building_conditional={prerequisites={condition={has_technology=tech_sample}}}
building_check={potential={has_technology=tech_sample}}
building_inline={possible={inline_script=some_template}}
building_text={prerequisites={tech_sample_suffix}}
# building_comment={prerequisites={tech_sample}}
building_string={name="prerequisites={tech_sample}"}
`)
	writeTechnologyFixture(t, root, "common/component_templates/fixture.txt", `
weapon_component_template={key="fixture_component" prerequisites={tech_sample}}
weapon_component_template={prerequisites={tech_sample}}
weapon_component_template={key=one key=two prerequisites={tech_sample}}
weapon_component_template={key=dynamic_component prerequisites={$tech_sample$}}
`)
	archive, err := os.Create(filepath.Join(root, "dlc", "fixture.zip"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(archive)
	for _, member := range []string{"bundle/common/ship_sizes/fixture.txt", "events/ignored.txt"} {
		w, err := z.Create(member)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("fixture_ship={prerequisites={tech_sample}}")); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	source := gamestate.NewTechnologyReferenceSource(locale)
	result := technologyReferenceResult(t, source, map[string]any{"technology_id": "tech_sample"})
	if result.IsError {
		t.Fatal("lookup failed")
	}
	got := resultText(result)
	technologyContains(t, got, "source_status=partial match_status=observed observed_fields=7",
		"inline_script_observed=true", "mod_coverage=not-inspected",
		`category=buildings source=common definition_key="building_sample" definition_key_occurrences=2 field_index=0 prerequisites_fields=2 prerequisites_shape=list literal_matches=2 direct_literal_count=3 definition_key_status=duplicate prerequisites_field_status=duplicate`,
		`entry[0]: "tech_sample" (raw scalar; unquoted)`,
		`entry[1]: "tech_other" (raw scalar; unquoted)`,
		`entry[2]: "tech_sample" (raw scalar; quoted)`,
		`definition_key="building_sample" definition_key_occurrences=2 field_index=1 prerequisites_fields=2`,
		`definition_key="building_mixed" definition_key_occurrences=1 field_index=0 prerequisites_fields=1 prerequisites_shape=mixed literal_matches=1`,
		`entry[1] key="condition": object static block`,
		`category=component_templates source=common wrapper_key="weapon_component_template" definition_key="fixture_component"`,
		`wrapper_key="weapon_component_template" definition_key_status=unresolved`,
		`wrapper_key="weapon_component_template" definition_key_status=unresolved`,
		`category=ship_sizes source=dlc-zip definition_key="fixture_ship"`)
	for _, skip := range []string{"building_conditional", "building_check", "building_text", "building_comment", "building_string", "dynamic_component", "fixture.txt", root} {
		if strings.Contains(got, skip) {
			t.Fatal("lookup included an excluded record or source path")
		}
	}
	for _, id := range []string{"tech_nested", "Tech_Sample", "missing"} {
		got := resultText(technologyReferenceResult(t, source, map[string]any{"technology_id": id}))
		technologyContains(t, got, "source_status=partial match_status=unresolved observed_fields=0")
	}
}

func TestTechnologyReferencesPaginationAndSourceStates(t *testing.T) {
	root, locale := technologyReferenceFixture(t)
	var script strings.Builder
	for i := 0; i < 23; i++ {
		fmt.Fprintf(&script, "fixture_%d={prerequisites={tech_page tech_other}}\n", i)
	}
	writeTechnologyFixture(t, root, "common/buildings/fixture.txt", script.String())
	oldDir, oldLoc := saveDir, loc
	t.Cleanup(func() { saveDir, loc = oldDir, oldLoc })
	s := server.NewMCPServer("test", "test")
	RegisterWithInstalledScripts(s, "", loc, locale)
	entry, ok := s.ListTools()["get_technology_references"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only registration")
	}
	if _, ok := entry.Tool.InputSchema.Properties["save_id"]; ok {
		t.Fatal("static reference lookup advertises a save selector")
	}
	result, err := entry.Handler(context.Background(), rawRequest(map[string]any{"technology_id": "tech_page"}))
	if err != nil || result.IsError {
		t.Fatal("lookup with no save directory failed")
	}
	first := resultText(result)
	technologyContains(t, first, "source_status=complete match_status=observed observed_fields=23 offset=0",
		`entry[19] category=buildings source=common definition_key="fixture_19"`, "next_offset=20")
	if strings.Contains(first, "entry[20] category=") {
		t.Fatal("page exceeds 20 fields")
	}
	var token string
	for _, line := range strings.Split(first, "\n") {
		if value, ok := strings.CutPrefix(line, "page_context="); ok {
			token = value
		}
	}
	if token == "" {
		t.Fatal("missing continuation context")
	}
	source := gamestate.NewTechnologyReferenceSource(locale)
	args := map[string]any{"technology_id": "tech_page", "offset": 20, "page_context": token}
	technologyContains(t, resultText(technologyReferenceResult(t, source, args)), "offset=20", "entry[20] category=",
		"entry[21] category=", "entry[22] category=", "next_offset=none")
	for _, invalid := range []map[string]any{
		{"technology_id": "tech_page", "offset": 20},
		{"technology_id": "tech_other", "offset": 20, "page_context": token},
		{"technology_id": "tech_page", "offset": 24, "page_context": token},
		{"technology_id": "tech_page", "offset": 0.5},
		{"technology_id": "tech_page", "page_context": 0},
		{"technology_id": 0},
		{"technology_id": "$dynamic$"},
	} {
		if !technologyReferenceResult(t, source, invalid).IsError {
			t.Fatalf("accepted invalid synthetic query %v", invalid)
		}
	}
	missing := map[string]any{"technology_id": "tech_missing"}
	technologyContains(t, resultText(technologyReferenceResult(t, source, missing)), "source_status=complete match_status=unmatched")
	technologyContains(t, resultText(technologyReferenceResult(t, nil, missing)), "source_status=unavailable match_status=unavailable")
	writeTechnologyFixture(t, root, "common/buildings/fixture.txt", script.String()+" changed={}")
	stale := technologyReferenceResult(t, gamestate.NewTechnologyReferenceSource(locale), args)
	if !stale.IsError || !strings.Contains(resultText(stale), "page_context") {
		t.Fatal("changed source accepted a stale token")
	}
	writeTechnologyFixture(t, root, "common/buildings/fixture.txt", script.String()+" changed={possible={inline_script=unexpanded}}")
	technologyContains(t, resultText(technologyReferenceResult(t, gamestate.NewTechnologyReferenceSource(locale), missing)),
		"source_status=partial match_status=unresolved", "inline_script_observed=true")
	writeTechnologyFixture(t, root, "common/buildings/fixture.txt", script.String())
	if err := os.Remove(filepath.Join(root, "common", "ship_sizes")); err != nil {
		t.Fatal(err)
	}
	technologyContains(t, resultText(technologyReferenceResult(t, gamestate.NewTechnologyReferenceSource(locale), missing)),
		"source_status=partial match_status=unresolved")
}
