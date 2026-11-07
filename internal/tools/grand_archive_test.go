package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestGrandArchiveShowsSavedShapeWithoutOwnershipInference(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`country={0={modules={standard_grand_archive_module={
	 exhibits={historical_item={specimen=0 specimen=1 details={secret=hidden_nested}} historical_item={} aesthetic_wonder={}}
	 auto_culling_settings={} culling_value={{} {} {}}
	 specimen_management_settings={auto_buy=unknown mixed={key=yes anonymous}}
	 }}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderGrandArchive(0, &c))
	for _, want := range []string{
		"Specimen identity, ownership, display status and setting effects remain unverified",
		"exhibits: object with 3 saved fields",
		`Field 0 key="historical_item": object with 3 saved fields`,
		`Field 1 key="historical_item" [duplicate saved key]: empty object`,
		`Field 0 key="specimen": "0" (saved scalar)`,
		`Field 1 key="specimen" [duplicate saved key]: "1" (saved scalar)`,
		"nested saved block; use raw tree/byte tools",
		"auto_culling_settings: empty object",
		"culling_value: list of 3 empty objects",
		`key="auto_buy": "unknown" (saved scalar)`,
		"mixed saved block; use raw tree/byte tools",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "hidden_nested") {
		t.Fatal("nested raw fallback was bypassed")
	}
}

func TestGrandArchiveMissingAndUnsupportedStates(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	for _, item := range []struct{ data, want string }{
		{`country={0={}}`, "country.modules not recorded"},
		{`country={0={modules={}}}`, "standard_grand_archive_module not recorded"},
		{`country={0={modules=none}}`, "Unsupported modules shape"},
		{`country={0={modules={standard_grand_archive_module=none}}}`, "Unsupported Grand Archive module shape"},
		{`country={0={modules={standard_grand_archive_module={exhibits={} exhibits={}}}}}`, "duplicate known fields"},
		{`country={0={modules={standard_grand_archive_module={culling_value={0 {}}}}}}`, "Item 0: \"0\" (saved scalar)\n  Item 1: empty object"},
		{`country={0={modules={standard_grand_archive_module={exhibits={}}}}}`, "exhibits: empty object\nauto_culling_settings: not recorded"},
	} {
		gs, err := gamestate.Load([]byte(item.data))
		if err != nil {
			t.Fatal(err)
		}
		c := gs.Country[0]
		if text := resultText(renderGrandArchive(0, &c)); !strings.Contains(text, item.want) {
			t.Fatalf("missing %q in %s", item.want, text)
		}
	}
}

func TestGrandArchiveRegisteredHandler(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={modules={standard_grand_archive_module={exhibits={}}}}}`)}})
	s := server.NewMCPServer("test", "test")
	registerGrandArchive(s)
	if _, ok := s.ListTools()["get_grand_archive"]; !ok {
		t.Fatal("Grand Archive tool missing")
	}
	result, err := handleGrandArchive(context.Background(), rawRequest(nil))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "exhibits: empty object") {
		t.Fatal("normal Grand Archive handler failed")
	}
}
