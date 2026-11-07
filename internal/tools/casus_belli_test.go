package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestCasusBelliSavedSourcesAndUnresolvedReferences(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`country={0={modules={standard_diplomacy_module={casus_belli={{owner=0 country=1 type=example days_left=0} {owner=1 country=99 type=example days_left=-1} {type=""}}} exclusive_diplomacy_module={casus_belli={{owner=99 country=0 type=exclusive}}}}} 1={adjective={key="Referenced country"} modules={standard_diplomacy_module={casus_belli={{owner=0 country=0 type=foreign_source}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderCasusBelli(gs, 0, &c))
	for _, want := range []string{"country.modules.standard_diplomacy_module.casus_belli", "country.modules.exclusive_diplomacy_module.casus_belli", `Entry 0 owner=0 country=1 type="example" days_left="0"`, `owner=1 country=99 type="example" days_left="-1"`, "owner differs from containing country", "Saved country=1 resolves to country record: Referenced country", "Saved country=99: referenced country record unavailable", "Saved owner=99: referenced country record unavailable", `owner=not recorded country=not recorded type="" days_left=not recorded`, "owner country reference not recorded", "current war-declaration eligibility and days_left semantics remain unverified"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "foreign_source") {
		t.Fatal("another containing country's module was included")
	}
	if strings.Index(text, `days_left="0"`) > strings.Index(text, `days_left="-1"`) {
		t.Fatal("saved order changed")
	}
}

func TestCasusBelliCollectionStates(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	for _, item := range []struct{ data, want string }{
		{`country={0={}}`, "country.modules not recorded"},
		{`country={0={modules=none}}`, "country.modules has an unsupported shape"},
		{`country={0={modules={standard_diplomacy_module={} standard_diplomacy_module={}}}}`, "duplicate diplomacy modules"},
		{`country={0={modules={}}}`, "Diplomacy module not recorded"},
		{`country={0={modules={standard_diplomacy_module={}}}}`, "casus_belli not recorded"},
		{`country={0={modules={standard_diplomacy_module={casus_belli={}}}}}`, "  Empty"},
		{`country={0={modules={standard_diplomacy_module={casus_belli={{owner=0 owner=1}}}}}}`, "Unsupported shape or duplicate known fields"},
	} {
		gs, err := gamestate.Load([]byte(item.data))
		if err != nil {
			t.Fatal(err)
		}
		c := gs.Country[0]
		text := resultText(renderCasusBelli(gs, 0, &c))
		if !strings.Contains(text, item.want) {
			t.Fatalf("missing %q in %s", item.want, text)
		}
	}
}

func TestCasusBelliRegisteredHandler(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={modules={standard_diplomacy_module={casus_belli={{owner=0 country=0 type=example}}}}}}`)}})
	s := server.NewMCPServer("test", "test")
	registerCasusBelli(s)
	entry, ok := s.ListTools()["get_casus_belli"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint || entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("casus_belli tool missing or annotations incorrect")
	}
	for _, key := range []string{"save_id", "country_id"} {
		if _, ok := entry.Tool.InputSchema.Properties[key]; !ok {
			t.Fatalf("missing input %q", key)
		}
	}
	for _, args := range []map[string]any{nil, {"country_id": 0}} {
		r, err := handleCasusBelli(context.Background(), rawRequest(args))
		if err != nil || r.IsError || !strings.Contains(resultText(r), `owner=0 country=0 type="example"`) {
			t.Fatal("normal casus_belli handler failed")
		}
	}
	r, err := handleCasusBelli(context.Background(), rawRequest(map[string]any{"country_id": 99}))
	if err != nil || !r.IsError {
		t.Fatal("unknown country accepted")
	}
}
