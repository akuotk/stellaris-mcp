package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestSpeciesRightsSavedRecordsAndCandidates(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`country={0={modules={standard_species_rights_module={
	 enabled=no default={citizenship=default_saved former_citizenship=former_default} primary={}
	 species_rights={{species_index=0 citizenship=example former_citizenship=former_specific last_changed_value=0}
	 {species_index=0 species_index=2 citizenship=ambiguous unsupported={nested=yes}}
	 {species_index=99 living_standard=unresolved}}
	 }}}} species_db={0={name={key="Candidate species"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	summary := resultText(renderSpeciesRights(gs, 0, &c, nil))
	for _, want := range []string{
		`enabled="no" (saved value)`, "Effective rights, inheritance", `key="citizenship" value="default_saved"`,
		`key="former_citizenship" value="former_default"`, "primary\n  Empty", "built_species\n  Not recorded",
		"species_rights[0]", "species database ID candidate: Candidate species", "species_rights[1]", "Duplicate saved keys retained",
		"species_index unsupported or duplicate", "Unsupported saved fields", "species_index=99; species database ID candidate unavailable",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in %s", want, summary)
		}
	}
	if strings.Contains(summary, "former_specific") || strings.Count(summary, "species database ID candidate: Candidate species") != 1 {
		t.Fatal("summary expanded history or resolved an ambiguous index")
	}
	selected := 0
	detail := resultText(renderSpeciesRights(gs, 0, &c, &selected))
	for _, want := range []string{`Field 2 key="former_citizenship" value="former_specific"`, `Field 3 key="last_changed_value" value="0"`} {
		if !strings.Contains(detail, want) {
			t.Fatalf("missing detail %q", want)
		}
	}
	if strings.Contains(detail, "species_rights[1]") {
		t.Fatal("detail included an unselected record")
	}
	selected = 1
	detail = resultText(renderSpeciesRights(gs, 0, &c, &selected))
	if !strings.Contains(detail, `Field 0 key="species_index" value="0"`) || !strings.Contains(detail, `Field 1 key="species_index" value="2"`) || strings.Contains(detail, "species database ID candidate: Candidate species") {
		t.Fatal("ambiguous indices lost raw values or produced a species candidate")
	}
}

func TestSpeciesRightsModuleAndListStates(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	for _, item := range []struct{ data, want string }{
		{`country={0={}}`, "country.modules not recorded"},
		{`country={0={modules={}}}`, "standard_species_rights_module not recorded"},
		{`country={0={modules=none}}`, "Unsupported modules shape"},
		{`country={0={modules={standard_species_rights_module={default={} default={}}}}}`, "duplicate known fields"},
		{`country={0={modules={standard_species_rights_module={enabled={bad=yes} species_rights={}}}}}`, "enabled unsupported"},
		{`country={0={modules={standard_species_rights_module={species_rights=none}}}}`, "Unsupported saved list shape"},
		{`country={0={modules={standard_species_rights_module={species_rights={}}}}}`, "species_rights (saved order; record_index shows all saved scalar fields)\n  Empty"},
	} {
		gs, err := gamestate.Load([]byte(item.data))
		if err != nil {
			t.Fatal(err)
		}
		c := gs.Country[0]
		if text := resultText(renderSpeciesRights(gs, 0, &c, nil)); !strings.Contains(text, item.want) {
			t.Fatalf("missing %q in %s", item.want, text)
		}
	}
}

func TestSpeciesRightsRegisteredHandler(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={modules={standard_species_rights_module={species_rights={{species_index=0 citizenship=example}}}}}}`)}})
	s := server.NewMCPServer("test", "test")
	registerSpeciesRights(s)
	entry, ok := s.ListTools()["get_species_rights"]
	if !ok {
		t.Fatal("species rights tool missing")
	}
	if _, ok := entry.Tool.InputSchema.Properties["record_index"]; !ok {
		t.Fatal("record index argument missing")
	}
	for _, args := range []map[string]any{nil, {"record_index": 0}, {"country_id": 0, "record_index": float64(0)}} {
		result, err := handleSpeciesRights(context.Background(), rawRequest(args))
		if err != nil || result.IsError || !strings.Contains(resultText(result), `key="citizenship" value="example"`) {
			t.Fatal("normal species rights handler failed")
		}
	}
	for _, index := range []any{"0", nil, true, -1, 0.5, 1} {
		result, err := handleSpeciesRights(context.Background(), rawRequest(map[string]any{"record_index": index}))
		if err != nil || !result.IsError {
			t.Fatal("invalid record index accepted")
		}
	}
}
