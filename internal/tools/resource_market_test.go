package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestResourceMarketRawAmountsAndCountryPaths(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`country={0={modules={standard_economy_module={resources={energy=0 energy=999999999999.123456 minerals=-0.0000001 invalid={nested=yes}}}}} 1={}}
	 market={country=0 monthly_trades={{id=0 amount=0 price=-1 trade_data={country=0 resource=0 trade_type=unknown}} {id=7 trade_data={country=1}} {id=8 trade_data={}} {id=9 trade_data={country=0}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderResourceMarket(gs, 0, &c))
	for _, want := range []string{"stockpile candidates", "country.modules.standard_economy_module.resources", `Entry 0 resource="energy" amount="0"`, `amount="999999999999.123456" [duplicate saved resource key]`, `amount="-0.0000001"`, "Unsupported resource entries", `Entry 0 id="0" trade_data.country=0 trade_data.resource="0" trade_data.trade_type="unknown" amount="0" price="-1"`, `Entry 3 id="9"`, "trade_data.resource=not recorded", "Order entries without country reference: 1", "current order activity and history remain unverified"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, `id="7"`) || strings.Contains(text, `id="8"`) {
		t.Fatal("foreign or missing-country order was scoped using global market.country")
	}
	if strings.Index(text, `id="0"`) > strings.Index(text, `id="9"`) {
		t.Fatal("saved order changed")
	}
}

func TestResourceMarketMissingEmptyUnsupportedOutput(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	for _, item := range []struct{ data, economy, market string }{
		{`country={0={}}`, "country.modules not recorded", "market not recorded"},
		{`country={0={modules={}}} market={}`, "standard_economy_module not recorded", "monthly_trades not recorded"},
		{`country={0={modules={standard_economy_module={}}}} market={monthly_trades={}}`, "resources not recorded", "  Empty"},
		{`country={0={modules={standard_economy_module={resources={}}}}} market={monthly_trades={{trade_data={country=1}}}}`, "  Empty", "No matching decoded entries"},
		{`country={0={modules=none}} market=none`, "Unsupported modules shape", "Unsupported market/order shape"},
		{`country={0={modules={standard_economy_module={resources={}} standard_economy_module={resources={}}}}} market={monthly_trades={} monthly_trades={}}`, "Unsupported modules shape", "Unsupported market/order shape"},
		{`country={0={modules={standard_economy_module={resources={} resources={}}}}} market={monthly_trades={{trade_data={country=0} trade_data={country=1}}}}`, "Unsupported modules shape", "Unsupported market/order shape"},
		{`country={0={modules={standard_economy_module={resources=none}}}} market={monthly_trades={{trade_data={country=invalid}}}}`, "Unsupported resource entries", "Unsupported market/order shape"},
	} {
		gs, err := gamestate.Load([]byte(item.data))
		if err != nil {
			t.Fatal(err)
		}
		c := gs.Country[0]
		text := resultText(renderResourceMarket(gs, 0, &c))
		if !strings.Contains(text, item.economy) || !strings.Contains(text, item.market) {
			t.Fatalf("missing collection state in %s", text)
		}
	}
}

func TestResourceMarketRegisteredHandler(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={modules={standard_economy_module={resources={energy=5}}}}} market={monthly_trades={{trade_data={country=0}}}}`)}})
	s := server.NewMCPServer("test", "test")
	registerResourceMarket(s)
	entry, ok := s.ListTools()["get_resource_market"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint || entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("resource/market tool missing or annotations incorrect")
	}
	for _, key := range []string{"save_id", "country_id"} {
		if _, ok := entry.Tool.InputSchema.Properties[key]; !ok {
			t.Fatalf("missing input %q", key)
		}
	}
	result, err := handleResourceMarket(context.Background(), rawRequest(nil))
	if err != nil || result.IsError || !strings.Contains(resultText(result), `resource="energy" amount="5"`) || !strings.Contains(resultText(result), "trade_data.country=0") {
		t.Fatal("normal resource/market handler failed")
	}
	result, err = handleResourceMarket(context.Background(), rawRequest(map[string]any{"country_id": 99}))
	if err != nil || !result.IsError {
		t.Fatal("unknown country accepted")
	}
}
