package gamestate

import (
	"os"
	"testing"
)

func TestDuplicateEconomyFieldsKeepOtherSavedSections(t *testing.T) {
	gs, err := Load([]byte(`country={0={budget={current_month={} current_month={}
	last_month={trade_income={}} income_high_water_mark={length=0}}}}
	market={monthly_trades={} monthly_trades={} resources_bought={country=0 amount={0}}}`))
	if err != nil {
		t.Fatal(err)
	}
	budget, market := gs.Country[0].Budget, gs.MarketOrders
	if !budget.CurrentMonth.Duplicate || !budget.LastMonth.TradeIncome.Recorded ||
		budget.IncomeHighWaterMark.Length.Value != "0" || !market.MonthlyTrades.Duplicate ||
		len(market.ResourcesBought.Items) != 1 || market.ResourcesBought.Items[0].Country.Value == nil ||
		*market.ResourcesBought.Items[0].Country.Value != 0 {
		t.Fatal("duplicate sections invalidated neighboring data or lost zero")
	}
	gs, err = Load([]byte(`date=2201.01.01 market={} market={resources_bought={country=0 amount={0}}}`))
	if err != nil || gs.Date != "2201.01.01" || !gs.MarketOrders.Duplicate || len(gs.MarketOrders.ResourcesBought.Items) != 0 {
		t.Fatal("duplicate market root was accepted or affected unrelated data")
	}
}

func TestEconomicLedgerLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only economy ledger coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	var rows, paired, countryMatches, scalars [3]int
	var markets, globalDetails, countries, watermarks, currentAmounts, historyRows, historyAmounts, tradeCategories, tradeAmounts, extraCategories, unsupported int
	for _, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatal("whole-save decoding failed")
		}
		m := gs.MarketOrders
		if m.Recorded {
			markets++
		}
		globalDetails += len(m.GlobalDetails)
		if m.Duplicate || m.Unsupported {
			unsupported++
		}
		for i, table := range []MarketCountryRows{m.ResourcesBought.MarketCountryRows, m.ResourcesSold.MarketCountryRows, m.InternalFluctuations.MarketCountryRows} {
			rows[i] += len(table.Items)
			unsupported += table.OrphanPayloads
			if table.Duplicate || table.Unsupported {
				unsupported++
			}
			for _, row := range table.Items {
				if row.Country.Unsupported || row.UnsupportedPayload {
					unsupported++
				}
				if row.Country.Value != nil {
					if _, ok := gs.Country[*row.Country.Value]; ok {
						countryMatches[i]++
					}
				}
				if row.Payload == nil {
					continue
				}
				paired[i]++
				if row.Payload.IsList() {
					scalars[i] += len(row.Payload.List)
				} else if row.Payload.IsObject() {
					scalars[i] += len(row.Payload.Object.Pairs)
				}
			}
		}
		count := func(list SavedBudgetCategories) int {
			if list.Duplicate || list.Unsupported {
				unsupported++
			}
			n := 0
			for _, row := range list.Items {
				n += len(row.Resources.Entries)
				if row.Resources.Unsupported {
					unsupported++
				}
			}
			return n
		}
		for _, c := range gs.Country {
			countries++
			b := c.Budget
			h := b.IncomeHighWaterMark
			if h.Recorded {
				watermarks++
			}
			if b.Duplicate || h.Duplicate || h.Unsupported || h.Current.Unsupported {
				unsupported++
			}
			currentAmounts += len(h.Current.Entries)
			historyRows += len(h.History.Items)
			historyAmounts += count(h.History)
			for _, month := range []BudgetMonth{b.CurrentMonth, b.LastMonth} {
				for _, list := range []SavedBudgetCategories{month.TradeIncome, month.TradeExpenses, month.TradeBalance} {
					tradeCategories += len(list.Items)
					tradeAmounts += count(list)
				}
				for _, list := range []SavedBudgetCategories{month.ExtraIncome, month.ExtraExpenses, month.ExtraBalance} {
					extraCategories += len(list.Items)
					count(list)
				}
			}
		}
	}
	t.Logf("local economy coverage: saves=%d markets=%d global_fields=%d market_bought_sold_internal_rows=%v paired=%v country_matches=%v saved_payload_entries=%v countries=%d high_water_marks=%d current_amounts=%d history_rows=%d history_amounts=%d trade_categories=%d trade_amounts=%d extra_categories=%d unsupported=%d", len(files), markets, globalDetails, rows, paired, countryMatches, scalars, countries, watermarks, currentAmounts, historyRows, historyAmounts, tradeCategories, tradeAmounts, extraCategories, unsupported)
	if unsupported != 0 {
		t.Fatal("local budget or market shapes need additional support")
	}
}
