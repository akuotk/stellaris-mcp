package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerResourceMarket(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_resource_market", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read saved resource candidates, monthly market orders and country rows under resources_bought, resources_sold and internal_market_fluctuations. Raw values and row order are preserved. Resource-index mapping, ledger period, price conversion and order activity remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player"))), handleResourceMarket)
}

func handleResourceMarket(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderResourceMarket(gs, id, country), nil
}

func renderResourceMarket(gs *gamestate.GameState, countryID int, country *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved resource and market records (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Resource amounts below are stockpile candidates from the saved economy module. Their stockpile identity and units remain unverified.")
	fmt.Fprintln(&b, "\ncountry.modules.standard_economy_module.resources (saved order)")
	modules := country.EconomyModules
	switch {
	case modules.Unsupported:
		fmt.Fprintln(&b, "  Unsupported modules shape or duplicate known fields; use raw tree/byte tools")
	case !modules.Recorded:
		fmt.Fprintln(&b, "  country.modules not recorded")
	case modules.Economy == nil:
		fmt.Fprintln(&b, "  standard_economy_module not recorded")
	default:
		resources := modules.Economy.Resources
		switch {
		case resources.Unsupported:
			fmt.Fprintln(&b, "  Unsupported resource entries; supported entries below may be partial, use raw tree/byte tools")
		case !resources.Recorded:
			fmt.Fprintln(&b, "  resources not recorded")
		case len(resources.Entries) == 0:
			fmt.Fprintln(&b, "  Empty")
		}
		seen := map[string]bool{}
		for index, entry := range resources.Entries {
			fmt.Fprintf(&b, "  Entry %d resource=%q amount=%q", index, entry.Resource, entry.Amount)
			if seen[entry.Resource] {
				fmt.Fprint(&b, " [duplicate saved resource key]")
			}
			seen[entry.Resource] = true
			fmt.Fprintln(&b)
		}
	}
	fmt.Fprintln(&b, "\nmarket.monthly_trades (saved order, filtered by trade_data.country)")
	fmt.Fprintln(&b, "  Resource identity, transaction direction, amount/price units, current order activity and history remain unverified.")
	market, trades := gs.MarketOrders, gs.MarketOrders.MonthlyTrades
	switch {
	case market.Duplicate:
		fmt.Fprintln(&b, "  Duplicate market tables. Use raw tree/byte tools.")
	case trades.Duplicate:
		fmt.Fprintln(&b, "  Duplicate monthly_trades fields. Use raw tree/byte tools.")
	case market.Unsupported || trades.Unsupported:
		fmt.Fprintln(&b, "  Unsupported market/order shape or duplicate known fields; use raw tree/byte tools")
	case !market.Recorded:
		fmt.Fprintln(&b, "  market not recorded")
	case !trades.Recorded:
		fmt.Fprintln(&b, "  monthly_trades not recorded")
	default:
		matched, missingCountry := 0, 0
		for index, trade := range trades.Items {
			if trade.TradeData.Country == nil {
				missingCountry++
				continue
			}
			if *trade.TradeData.Country != countryID {
				continue
			}
			matched++
			fmt.Fprintf(&b, "  Entry %d id=%s trade_data.country=%d trade_data.resource=%s trade_data.trade_type=%s amount=%s price=%s\n", index, governanceValue(trade.ID), *trade.TradeData.Country, governanceValue(trade.TradeData.Resource), governanceValue(trade.TradeData.TradeType), governanceValue(trade.Amount), governanceValue(trade.Price))
		}
		if missingCountry > 0 {
			fmt.Fprintf(&b, "  Order entries without country reference: %d; matching-country coverage incomplete\n", missingCountry)
		}
		if len(trades.Items) == 0 {
			fmt.Fprintln(&b, "  Empty")
		} else if matched == 0 {
			fmt.Fprintln(&b, "  No matching decoded entries")
		}
	}
	writeMarketCountryRows(&b, countryID, "market.resources_bought", market.ResourcesBought.MarketCountryRows)
	writeMarketCountryRows(&b, countryID, "market.resources_sold", market.ResourcesSold.MarketCountryRows)
	writeMarketCountryRows(&b, countryID, "market.internal_market_fluctuations", market.InternalFluctuations.MarketCountryRows)
	if len(market.GlobalDetails) > 0 {
		fmt.Fprintln(&b, "\nGlobal market fields (saved order. Array index mappings and prices remain unverified.)")
		seen := map[string]bool{}
		for _, pair := range market.GlobalDetails {
			label := "market." + pair.Key
			if seen[pair.Key] {
				label += " [duplicate saved key]"
			}
			seen[pair.Key] = true
			writeSavedValue(&b, label, pair.Value, 0)
		}
	}
	return mcp.NewToolResultText(b.String())
}

func writeMarketCountryRows(b *strings.Builder, countryID int, label string, rows gamestate.MarketCountryRows) {
	fmt.Fprintf(b, "\n%s (saved order, matching country)\n", label)
	switch {
	case rows.Duplicate:
		fmt.Fprintln(b, "  Duplicate saved fields. Use raw tools.")
	case !rows.Recorded:
		fmt.Fprintln(b, "  Not recorded.")
	case rows.Unsupported:
		fmt.Fprintln(b, "  Unsupported saved shape or fields. Use raw tools for complete data.")
	case len(rows.Items) == 0 && rows.OrphanPayloads == 0:
		fmt.Fprintln(b, "  Empty.")
	}
	if rows.OrphanPayloads > 0 {
		fmt.Fprintf(b, "  Payloads without an immediately preceding country: %d. Country unresolved.\n", rows.OrphanPayloads)
	}
	matched, unsupportedCountry := 0, 0
	for index, row := range rows.Items {
		if row.Country.Unsupported {
			unsupportedCountry++
		}
		if row.Country.Value == nil || *row.Country.Value != countryID {
			continue
		}
		matched++
		fmt.Fprintf(b, "  Row %d country=%d\n", index, countryID)
		if row.Payload == nil {
			fmt.Fprintln(b, "    Adjacent payload not recorded.")
			continue
		}
		if row.UnsupportedPayload {
			fmt.Fprintln(b, "    Unsupported payload shape. Use raw tools for complete data.")
		}
		if row.Payload.IsList() && !row.UnsupportedPayload {
			values := make([]string, 0, len(row.Payload.List))
			for _, item := range row.Payload.List {
				values = append(values, item.Scalar)
			}
			fmt.Fprintf(b, "    Amounts in saved order: %q (resource indices and accumulation period unverified)\n", values)
		} else {
			writeSavedValue(b, "    Saved payload", *row.Payload, 0)
		}
	}
	if unsupportedCountry > 0 {
		fmt.Fprintf(b, "  Rows with unsupported country fields: %d. Country unresolved.\n", unsupportedCountry)
	}
	if matched > 1 {
		fmt.Fprintf(b, "  Duplicate saved country rows: %d. Rows are shown separately.\n", matched)
	} else if matched == 0 && len(rows.Items) != 0 {
		fmt.Fprintln(b, "  No matching decoded country rows.")
	}
}
