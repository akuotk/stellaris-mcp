package gamestate

import (
	"os"
	"testing"
)

func TestResourceMarketKeepsRawValuesAndMissingReferences(t *testing.T) {
	gs, err := Load([]byte(`country={0={modules={standard_economy_module={resources={energy=0 minerals=-0.0000001 energy=999999999999999.123456 unknown=none}}}} 1={modules={standard_economy_module={resources={}}}} 2={modules={standard_economy_module={}}} 3={modules={}} 4={}}
	 market={country=9 monthly_trades={{id=0 amount=0.0000001 price=-1 trade_data={country=0 resource=0 trade_type=unverified}} {trade_data={}} {}}}`))
	if err != nil {
		t.Fatal(err)
	}
	resources := gs.Country[0].EconomyModules.Economy.Resources
	if !gs.Country[0].EconomyModules.Recorded || !resources.Recorded || resources.Unsupported || len(resources.Entries) != 4 || resources.Entries[0].Amount != "0" || resources.Entries[1].Amount != "-0.0000001" || resources.Entries[2].Resource != "energy" || resources.Entries[2].Amount != "999999999999999.123456" || resources.Entries[3].Amount != "none" {
		t.Fatal("raw resources, duplicate keys, precision or saved order lost")
	}
	if !gs.Country[1].EconomyModules.Economy.Resources.Recorded || len(gs.Country[1].EconomyModules.Economy.Resources.Entries) != 0 || gs.Country[2].EconomyModules.Economy.Resources.Recorded || gs.Country[3].EconomyModules.Economy != nil || gs.Country[4].EconomyModules.Recorded {
		t.Fatal("missing and empty resources or modules conflated")
	}
	trades := gs.MarketOrders.MonthlyTrades
	if !gs.MarketOrders.Recorded || gs.MarketOrders.Unsupported || !trades.Recorded || trades.Unsupported || len(trades.Items) != 3 {
		t.Fatal("supported market order shape rejected")
	}
	trade := trades.Items[0]
	if trade.ID == nil || *trade.ID != "0" || trade.Amount == nil || *trade.Amount != "0.0000001" || *trade.Price != "-1" || trade.TradeData.Country == nil || *trade.TradeData.Country != 0 || *trade.TradeData.Resource != "0" || *trade.TradeData.TradeType != "unverified" || trades.Items[1].TradeData.Country != nil || trades.Items[2].ID != nil {
		t.Fatal("raw order fields or missing references lost")
	}
}

func TestResourceMarketUnsupportedShapesStayIsolated(t *testing.T) {
	for _, data := range []string{
		`country={0={modules=none}} market=none`,
		`country={0={modules={standard_economy_module=none}}} market={monthly_trades={unexpected={}}}`,
		`country={0={modules={standard_economy_module={resources={energy=3 minerals={nested=yes}}}}}} market={monthly_trades={{trade_data={country=0}} {trade_data={country=invalid}}}}`,
		`country={0={modules={standard_economy_module={resources={energy=3 anonymous}}}}} market={monthly_trades=none}`,
	} {
		gs, err := Load([]byte(`date=2201.01.01 ` + data))
		if err != nil || gs.Date != "2201.01.01" {
			t.Fatal("optional resource/market shape prevented unrelated save loading")
		}
		modules := gs.Country[0].EconomyModules
		if !modules.Unsupported && (modules.Economy == nil || !modules.Economy.Resources.Unsupported) {
			t.Fatal("unsupported economy shape unreported")
		}
		if !gs.MarketOrders.Unsupported && !gs.MarketOrders.MonthlyTrades.Unsupported {
			t.Fatal("unsupported market shape unreported")
		}
		if len(gs.MarketOrders.MonthlyTrades.Items) != 0 {
			t.Fatal("partially decoded order country could create a false country-zero match")
		}
	}
	gs, err := Load([]byte(`country={0={modules={standard_economy_module={resources={energy=0 invalid={}}}}}} market={monthly_trades={}}`))
	if err != nil || !gs.Country[0].EconomyModules.Economy.Resources.Unsupported || len(gs.Country[0].EconomyModules.Economy.Resources.Entries) != 1 || !gs.MarketOrders.MonthlyTrades.Recorded || gs.MarketOrders.MonthlyTrades.Unsupported || len(gs.MarketOrders.MonthlyTrades.Items) != 0 {
		t.Fatal("supported resource neighbors or empty market orders lost")
	}
}

func TestResourceMarketDuplicateWrapperFieldsAreUnsupported(t *testing.T) {
	for _, fields := range []string{
		`modules={standard_economy_module={resources={energy=1}} standard_economy_module={resources={energy=2}}}`,
		`modules={standard_economy_module={resources={energy=1} resources={energy=2}}}`,
	} {
		gs, err := Load([]byte(`date=2201.01.01 country={0={` + fields + `}}`))
		if err != nil || gs.Date != "2201.01.01" || !gs.Country[0].EconomyModules.Unsupported || gs.Country[0].EconomyModules.Economy != nil {
			t.Fatal("duplicate economy wrapper was silently accepted or prevented save loading")
		}
	}
	for _, market := range []string{
		`monthly_trades={} monthly_trades={}`,
		`monthly_trades={{id=0 id=1 trade_data={country=0}}}`,
		`monthly_trades={{amount=1 amount=2 trade_data={country=0}}}`,
		`monthly_trades={{price=1 price=2 trade_data={country=0}}}`,
		`monthly_trades={{trade_data={country=0} trade_data={country=1}}}`,
		`monthly_trades={{trade_data={country=0 country=1}}}`,
		`monthly_trades={{trade_data={country=0 resource=0 resource=1}}}`,
		`monthly_trades={{trade_data={country=0 trade_type=0 trade_type=1}}}`,
	} {
		gs, err := Load([]byte(`date=2201.01.01 market={` + market + `}`))
		if err != nil || gs.Date != "2201.01.01" || (!gs.MarketOrders.Unsupported && !gs.MarketOrders.MonthlyTrades.Unsupported) || len(gs.MarketOrders.MonthlyTrades.Items) != 0 {
			t.Fatal("duplicate order fields were silently accepted or left a usable country path")
		}
	}
}

// This survey reads every local save and logs aggregate counts only.
func TestResourceMarketLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local resource/market coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("save catalog unavailable or empty")
	}
	decoded, countries, economies, resourceTables, resourceEntries, unsupported, markets, tradeLists, trades, missingCountry, unresolvedCountry, missingFields := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("catalog save index %d whole-save decoding failed", index)
		}
		decoded++
		for _, c := range gs.Country {
			countries++
			if c.EconomyModules.Unsupported {
				unsupported++
			}
			if c.EconomyModules.Economy == nil {
				continue
			}
			economies++
			resources := c.EconomyModules.Economy.Resources
			if resources.Recorded {
				resourceTables++
			}
			if resources.Unsupported {
				unsupported++
			}
			resourceEntries += len(resources.Entries)
		}
		if gs.MarketOrders.Recorded {
			markets++
		}
		if gs.MarketOrders.Unsupported || gs.MarketOrders.MonthlyTrades.Unsupported {
			unsupported++
		}
		if gs.MarketOrders.MonthlyTrades.Recorded {
			tradeLists++
		}
		trades += len(gs.MarketOrders.MonthlyTrades.Items)
		for _, trade := range gs.MarketOrders.MonthlyTrades.Items {
			if trade.TradeData.Country == nil {
				missingCountry++
			} else if _, ok := gs.Country[*trade.TradeData.Country]; !ok {
				unresolvedCountry++
			}
			if trade.ID == nil || trade.Amount == nil || trade.Price == nil || trade.TradeData.Resource == nil || trade.TradeData.TradeType == nil {
				missingFields++
			}
		}
	}
	t.Logf("local resource/market coverage: saves=%d decoded=%d countries=%d economy_modules=%d resource_tables=%d raw_resource_entries=%d recorded_markets=%d recorded_trade_lists=%d monthly_trade_entries=%d missing_trade_country=%d unresolved_trade_country=%d trade_entries_missing_fields=%d unsupported_shapes=%d", len(files), decoded, countries, economies, resourceTables, resourceEntries, markets, tradeLists, trades, missingCountry, unresolvedCountry, missingFields, unsupported)
	if unsupported > 0 {
		t.Fatal("local resource/market shapes need additional support")
	}
}
