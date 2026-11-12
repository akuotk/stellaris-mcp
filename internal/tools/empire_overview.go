package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerEmpireOverview(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_empire_overview", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Get empire overview: ethics, civics, government, power ratings, pops, saved monthly balance, saved resource amounts (stockpile candidates) and factions"),
			mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save")),
		),
		handleEmpireOverview,
	)
}

func handleEmpireOverview(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	countryID, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}

	// In 4.5 owned_planets contains colony IDs; ownership is explicit on planets.
	planetCount := 0
	for _, p := range gs.Planets.Planet {
		if p.OwnerID != nil && *p.OwnerID == countryID && isColonized(p) {
			planetCount++
		}
	}

	// Net balance from budget. Categories are added in sorted order: float
	// addition is not associative, so map order would change the last digits.
	balance := c.Budget.CurrentMonth.Balance
	categories := make([]string, 0, len(balance))
	for category := range balance {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	netBalance := make(map[string]float64)
	for _, category := range categories {
		for k, v := range balance[category] {
			netBalance[k] += v
		}
	}

	ethics := c.Ethos.Ethics
	if len(ethics) == 0 {
		ethics = c.Ethos.Ethic
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Empire Overview (%s) ===\n", gs.Date)
	fmt.Fprintf(&b, "Game: %s (version %s)\n\n", gs.Name, gs.Version)
	fmt.Fprintf(&b, "Name: %s\n", name(c.Adjective))
	fmt.Fprintf(&b, "Government: %s (%s)\n", l(c.Government.Type), l(c.Government.Authority))
	fmt.Fprintf(&b, "Origin: %s\n", l(c.Government.Origin))
	fmt.Fprintf(&b, "Ethics: %s\n", strings.Join(ll(ethics), ", "))
	fmt.Fprintf(&b, "Civics: %s\n", strings.Join(ll(c.Government.Civics), ", "))
	fmt.Fprintf(&b, "\nPower Ratings:\n")
	fmt.Fprintf(&b, "  Military: %s\n", savedFloat(c.MilitaryPowerRaw))
	fmt.Fprintf(&b, "  Economy:  %s\n", savedFloat(c.EconomyPowerRaw))
	fmt.Fprintf(&b, "  Tech:     %s\n", savedFloat(c.TechPowerRaw))
	fmt.Fprintf(&b, "  Victory Rank: %s\n", savedInt(c.VictoryRankRaw))
	fmt.Fprintf(&b, "\nEmpire Stats:\n")
	fmt.Fprintf(&b, "  Population (raw save units): %s\n", savedInt(c.NumPopsRaw))
	fmt.Fprintf(&b, "  Owned planet records with colony indicators: %d (non-planet carriers excluded)\n", planetCount)
	fmt.Fprintf(&b, "  Fleet Size: %s (Naval Cap: %s)\n", savedFloat(c.FleetSizeRaw), formatNavalCapacity(c))
	fmt.Fprintf(&b, "  Starbase capacity: %s\n", savedInt(c.StarbaseCapRaw))
	fmt.Fprintf(&b, "  Empire Size: %s\n", savedInt(c.EmpireSizeRaw))
	fmt.Fprintln(&b, "\nSaved current-month balance (sum of saved categories):")
	if !c.Budget.CurrentMonth.Recorded || c.Budget.CurrentMonth.Balance == nil {
		fmt.Fprintln(&b, "  not recorded")
	} else {
		fmt.Fprintf(&b, "  %s\n", formatBudgetResources(netBalance))
	}

	writeOverviewResources(&b, c)

	writeCountryFactions(&b, gs, countryID)

	return mcp.NewToolResultText(b.String()), nil
}

// writeOverviewResources prints the amounts get_resource_market reads, on one
// line. Amounts stay as saved text; one that is not a number is quoted.
func writeOverviewResources(b *strings.Builder, c *gamestate.Country) {
	fmt.Fprintln(b, "\nSaved resource amounts (economy module, saved order; stockpile candidates, units unverified):")
	entries, status := economyModuleResources(c)
	if status != "" {
		fmt.Fprintf(b, "  %s\n", status)
	}
	if len(entries) == 0 {
		return
	}
	parts := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		amount := entry.Amount
		if _, err := strconv.ParseFloat(amount, 64); err != nil {
			amount = strconv.Quote(amount)
		}
		part := fmt.Sprintf("%s [%s]=%s", l(entry.Resource), entry.Resource, amount)
		if seen[entry.Resource] {
			part += " [duplicate saved resource key]"
		}
		seen[entry.Resource] = true
		parts = append(parts, part)
	}
	fmt.Fprintf(b, "  %s\n", strings.Join(parts, ", "))
}
