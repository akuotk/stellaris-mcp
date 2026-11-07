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
		mcp.WithDescription("Read a country's saved modules.standard_economy_module.resources and market.monthly_trades entries with matching trade_data.country. Resource amounts are stockpile candidates. Keeps raw resource, amount, price and trade_type values. Units, stockpile identity, order activity, history and transaction direction remain unverified."),
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
	return mcp.NewToolResultText(b.String())
}
