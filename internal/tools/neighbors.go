package tools

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerNeighbors(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_neighbors", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Get saved known-country relations, opinion, trust, and raw military-power ratio; game UI strength categories are not inferred"),
			mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save")),
		),
		handleNeighbors,
	)
}

func handleNeighbors(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	_, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}

	return renderNeighbors(gs, c), nil
}

func renderNeighbors(gs *gamestate.GameState, c *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Known Empires (%s) ===\n", name(c.Adjective))
	fmt.Fprintf(&b, "Your saved military power: %s\n", armyFloat(c.MilitaryPowerRaw))
	fmt.Fprintln(&b, "Military ratios are calculated from saved values, not Stellaris UI strength categories.")
	contacts := 0
	unknownContact := 0
	for _, rel := range c.RelationsManager.Relation {
		if rel.ContactRaw == nil || (*rel.ContactRaw != "yes" && *rel.ContactRaw != "no") {
			unknownContact++
			fmt.Fprintf(&b, "\nRelation: country=%s contact=%s (contact status not established)\n", armyInt(rel.CountryID), operationFlagText(rel.ContactRaw))
			continue
		}
		if *rel.ContactRaw != "yes" {
			continue
		}
		contacts++
		if rel.CountryID == nil {
			fmt.Fprintln(&b, "\nContact: country reference not recorded")
			fmt.Fprintf(&b, "  Saved relation_current=%s trust=%s\n", armyFloat(rel.RelationCurrentRaw), armyFloat(rel.TrustRaw))
			continue
		}
		neighbor, ok := gs.Country[*rel.CountryID]
		if !ok {
			fmt.Fprintf(&b, "\nContact country %d: referenced record unavailable\n", *rel.CountryID)
			fmt.Fprintf(&b, "  Saved relation_current=%s trust=%s\n", armyFloat(rel.RelationCurrentRaw), armyFloat(rel.TrustRaw))
			continue
		}
		fmt.Fprintf(&b, "\n%s (Country %d):\n", name(neighbor.Adjective), *rel.CountryID)
		fmt.Fprintf(&b, "  Saved relation_current=%s trust=%s\n", armyFloat(rel.RelationCurrentRaw), armyFloat(rel.TrustRaw))
		fmt.Fprintf(&b, "  Saved military=%s economy=%s tech=%s\n", armyFloat(neighbor.MilitaryPowerRaw), armyFloat(neighbor.EconomyPowerRaw), armyFloat(neighbor.TechPowerRaw))
		if c.MilitaryPowerRaw != nil && *c.MilitaryPowerRaw > 0 && !math.IsInf(*c.MilitaryPowerRaw, 0) && neighbor.MilitaryPowerRaw != nil && !math.IsNaN(*neighbor.MilitaryPowerRaw) && !math.IsInf(*neighbor.MilitaryPowerRaw, 0) {
			ratio := *neighbor.MilitaryPowerRaw / *c.MilitaryPowerRaw
			if math.IsInf(ratio, 0) || math.IsNaN(ratio) {
				fmt.Fprintln(&b, "  Derived military ratio: unavailable (non-finite result)")
			} else {
				fmt.Fprintf(&b, "  Derived military ratio (neighbor / selected): %g\n", ratio)
			}
		} else {
			fmt.Fprintln(&b, "  Derived military ratio: unavailable")
		}
	}
	if contacts == 0 && unknownContact == 0 {
		fmt.Fprintln(&b, "No saved contact=yes relations.")
	}
	return mcp.NewToolResultText(b.String())
}
