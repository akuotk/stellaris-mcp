package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerEmpireOverview(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_empire_overview", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Get empire overview: ethics, civics, government, resources, pops, power ratings"),
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

	// Net balance from budget
	netBalance := make(map[string]float64)
	for _, resources := range c.Budget.CurrentMonth.Balance {
		for k, v := range resources {
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
	fmt.Fprintf(&b, "  Military: %s\n", armyFloat(c.MilitaryPowerRaw))
	fmt.Fprintf(&b, "  Economy:  %s\n", armyFloat(c.EconomyPowerRaw))
	fmt.Fprintf(&b, "  Tech:     %s\n", armyFloat(c.TechPowerRaw))
	fmt.Fprintf(&b, "  Victory Rank: %s\n", armyInt(c.VictoryRankRaw))
	fmt.Fprintf(&b, "\nEmpire Stats:\n")
	fmt.Fprintf(&b, "  Population (raw save units): %d\n", c.NumPops)
	fmt.Fprintf(&b, "  Planets: %d\n", planetCount)
	fmt.Fprintf(&b, "  Fleet Size: %.0f (Naval Cap: %s)\n", c.FleetSize, formatNavalCapacity(c))
	fmt.Fprintf(&b, "  Starbases: %d capacity\n", c.StarbaseCap)
	fmt.Fprintf(&b, "  Empire Size: %d\n", c.EmpireSize)
	fmt.Fprintln(&b, "\nSaved current-month balance (sum of saved categories):")
	if !c.Budget.CurrentMonth.Recorded || c.Budget.CurrentMonth.Balance == nil {
		fmt.Fprintln(&b, "  not recorded")
	} else {
		fmt.Fprintf(&b, "  %s\n", formatBudgetResources(netBalance))
	}

	writeCountryFactions(&b, gs, countryID)

	return mcp.NewToolResultText(b.String()), nil
}
