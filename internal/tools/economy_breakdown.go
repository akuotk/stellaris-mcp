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
			mcp.WithDescription("Get saved current or previous month budget by category and raw balance; computed net is separately labeled"),
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
	if !month.Recorded {
		fmt.Fprintln(&b, "Budget month: not recorded")
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
	return mcp.NewToolResultText(b.String())
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
