package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerEconomyBreakdown(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_economy_breakdown", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Read saved current or previous month budget, trade and extra categories, and income_high_water_mark samples. Saved values and computed net are separated. History timing remains unverified."),
			mcp.WithString("month", mcp.Description("current (default) or last"), mcp.DefaultString("current")),
			mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save")),
		),
		handleEconomyBreakdown,
	)
}

func handleEconomyBreakdown(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	_, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}

	if value, ok := req.GetArguments()["month"]; ok {
		if _, ok := value.(string); !ok {
			return toolError(fmt.Errorf("month must be a string")), nil
		}
	}
	monthName := req.GetString("month", "current")
	var month gamestate.BudgetMonth
	switch monthName {
	case "current":
		month = c.Budget.CurrentMonth
	case "last":
		month = c.Budget.LastMonth
	default:
		return toolError(fmt.Errorf("month must be current or last")), nil
	}
	return renderEconomyBreakdown(c, monthName, month), nil
}

func renderEconomyBreakdown(c *gamestate.Country, monthName string, month gamestate.BudgetMonth) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Economy Breakdown (%s; saved %s month) ===\n", name(c.Adjective), monthName)
	if c.Budget.Duplicate || month.Duplicate {
		fmt.Fprintln(&b, "Budget month: duplicate saved fields. Use raw tools.")
		writeIncomeHighWaterMark(&b, c.Budget.IncomeHighWaterMark)
		return mcp.NewToolResultText(b.String())
	}
	if !month.Recorded {
		fmt.Fprintln(&b, "Budget month: not recorded")
		writeIncomeHighWaterMark(&b, c.Budget.IncomeHighWaterMark)
		return mcp.NewToolResultText(b.String())
	}

	// Income
	fmt.Fprintf(&b, "\n--- INCOME ---\n")
	totalIncome := make(map[string]float64)
	writeBudgetSection(&b, month.Income, totalIncome)
	if month.Income == nil {
		fmt.Fprintln(&b, "  Income categories: not recorded")
	} else {
		fmt.Fprintf(&b, "  TOTAL: %s\n", formatBudgetResources(totalIncome))
	}

	// Expenses
	fmt.Fprintf(&b, "\n--- EXPENSES ---\n")
	totalExpenses := make(map[string]float64)
	writeBudgetSection(&b, month.Expenses, totalExpenses)
	if month.Expenses == nil {
		fmt.Fprintln(&b, "  Expense categories: not recorded")
	} else {
		fmt.Fprintf(&b, "  TOTAL: %s\n", formatBudgetResources(totalExpenses))
	}

	// Net
	fmt.Fprintf(&b, "\n--- COMPUTED NET (income minus expenses) ---\n")
	if month.Income == nil || month.Expenses == nil {
		fmt.Fprintln(&b, "  Incomplete: one or more category tables not recorded")
	}
	net := make(map[string]float64)
	for k, v := range totalIncome {
		net[k] += v
	}
	for k, v := range totalExpenses {
		net[k] -= v
	}
	keys := make([]string, 0, len(net))
	for k := range net {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sign := "+"
		if net[k] < 0 {
			sign = ""
		}
		fmt.Fprintf(&b, "  %s [%s]: %s%g\n", l(k), k, sign, net[k])
	}

	fmt.Fprintln(&b, "\n--- SAVED BALANCE CATEGORIES ---")
	if month.Balance == nil {
		fmt.Fprintln(&b, "  Balance categories: not recorded")
	} else {
		writeBudgetSection(&b, month.Balance, nil)
	}
	fmt.Fprintln(&b, "\n--- SAVED TRADE AND EXTRA CATEGORIES ---")
	commercial := c.Budget.CurrentCommercialPacts
	if monthName == "last" {
		commercial = c.Budget.LastCommercialPacts
	}
	fmt.Fprintf(&b, "  %s_month_commercial_pacts=%s (saved value)\n", monthName, councilSavedString(commercial))
	for _, section := range []struct {
		label string
		value gamestate.SavedBudgetCategories
	}{
		{"trade_income", month.TradeIncome}, {"trade_expenses", month.TradeExpenses}, {"trade_balance", month.TradeBalance},
		{"extra_income", month.ExtraIncome}, {"extra_expenses", month.ExtraExpenses}, {"extra_balance", month.ExtraBalance},
	} {
		writeSavedBudgetCategories(&b, section.label, section.value)
	}
	writeIncomeHighWaterMark(&b, c.Budget.IncomeHighWaterMark)
	return mcp.NewToolResultText(b.String())
}

func writeIncomeHighWaterMark(b *strings.Builder, h gamestate.IncomeHighWaterMark) {
	fmt.Fprintln(b, "\n--- SAVED INCOME_HIGH_WATER_MARK ---")
	switch {
	case h.Duplicate:
		fmt.Fprintln(b, "  Duplicate saved fields. Use raw tools.")
	case h.Unsupported:
		fmt.Fprintln(b, "  Unsupported saved shape. Use raw tools.")
	case !h.Recorded:
		fmt.Fprintln(b, "  Not recorded.")
	default:
		fmt.Fprintf(b, "  length=%s (saved value). History interval and outer key meanings remain unverified.\n", councilSavedString(h.Length))
		writeRawBudgetAmounts(b, "  current", h.Current)
		writeSavedBudgetCategories(b, "history", h.History)
	}
}

func writeSavedBudgetCategories(b *strings.Builder, label string, categories gamestate.SavedBudgetCategories) {
	switch {
	case categories.Duplicate:
		fmt.Fprintf(b, "  %s: duplicate saved fields. Use raw tools.\n", label)
	case categories.Unsupported:
		fmt.Fprintf(b, "  %s: unsupported saved shape. Use raw tools.\n", label)
	case !categories.Recorded:
		fmt.Fprintf(b, "  %s: not recorded\n", label)
	case len(categories.Items) == 0:
		fmt.Fprintf(b, "  %s: empty\n", label)
	default:
		fmt.Fprintf(b, "  %s (saved order):\n", label)
		seen := map[string]bool{}
		for i, entry := range categories.Items {
			row := fmt.Sprintf("    Entry %d key=%q", i, entry.Key)
			if seen[entry.Key] {
				row += " [duplicate saved key]"
			}
			seen[entry.Key] = true
			writeRawBudgetAmounts(b, row, entry.Resources)
		}
	}
}

func writeRawBudgetAmounts(b *strings.Builder, label string, resources gamestate.SavedResourceAmounts) {
	fmt.Fprintf(b, "%s:", label)
	switch {
	case resources.Unsupported:
		fmt.Fprint(b, " unsupported saved shape or entries (use raw tools)")
	case !resources.Recorded:
		fmt.Fprint(b, " not recorded")
	case len(resources.Entries) == 0:
		fmt.Fprint(b, " empty")
	}
	seen := map[string]bool{}
	for _, entry := range resources.Entries {
		fmt.Fprintf(b, " %q=%q", entry.Resource, entry.Amount)
		if seen[entry.Resource] {
			fmt.Fprint(b, " [duplicate saved resource key]")
		}
		seen[entry.Resource] = true
	}
	fmt.Fprintln(b)
}

func writeBudgetSection(b *strings.Builder, categories map[string]map[string]float64, totals map[string]float64) {
	keys := make([]string, 0, len(categories))
	for k := range categories {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, cat := range keys {
		resources := categories[cat]
		fmt.Fprintf(b, "  %s [%s]: %s\n", l(cat), cat, formatBudgetResources(resources))
		if totals != nil {
			for k, v := range resources {
				totals[k] += v
			}
		}
	}
}

func formatBudgetResources(resources map[string]float64) string {
	if resources == nil {
		return "not recorded"
	}
	if len(resources) == 0 {
		return "empty"
	}
	keys := make([]string, 0, len(resources))
	for key := range resources {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s [%s]=%g", l(key), key, resources[key]))
	}
	return strings.Join(parts, ", ")
}
