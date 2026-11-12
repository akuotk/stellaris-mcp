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
			mcp.WithDescription("Summarize known countries with saved opinion, trust and military power. Pass neighbor_id for both directions of saved relations, including truce, pacts, borders and opinion modifiers. Current diplomatic effects and positional meanings remain unverified."),
			mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save")),
			mcp.WithNumber("neighbor_id", mcp.Description("Other country ID for saved relation details in both directions")),
		),
		handleNeighbors,
	)
}

func handleNeighbors(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}

	if _, ok := req.GetArguments()["neighbor_id"]; ok {
		neighborID, err := rawInteger(req, "neighbor_id", 0, 0, maxRecordID)
		if err != nil {
			return toolError(err), nil
		}
		return renderNeighborRelations(gs, id, c, neighborID), nil
	}
	return renderNeighbors(gs, c), nil
}

func renderNeighbors(gs *gamestate.GameState, c *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Known Empires (%s) ===\n", name(c.Adjective))
	fmt.Fprintf(&b, "Your saved military power: %s\n", savedFloat(c.MilitaryPowerRaw))
	fmt.Fprintln(&b, "Military ratios are calculated from saved values, not Stellaris UI strength categories.")
	fmt.Fprintln(&b, "Pass neighbor_id for saved details in both directions.")
	if !writeRelationManagerState(&b, c.RelationsManager) {
		return mcp.NewToolResultText(b.String())
	}
	contacts := 0
	unknownContact := 0
	for _, rel := range c.RelationsManager.Relation {
		if rel.Unsupported {
			unknownContact++
			fmt.Fprintln(&b, "\nRelation: unsupported saved row shape. Use raw tools.")
			continue
		}
		if rel.ContactRaw == nil || (*rel.ContactRaw != "yes" && *rel.ContactRaw != "no") {
			unknownContact++
			fmt.Fprintf(&b, "\nRelation: country=%s contact=%s (contact status not established)\n", relationRefText(rel.CountryRef, rel.CountryDuplicate), relationMetricText(rel.Contact, rel.ContactDuplicate))
			continue
		}
		if *rel.ContactRaw != "yes" {
			continue
		}
		contacts++
		if rel.CountryID == nil {
			fmt.Fprintf(&b, "\nContact: country reference %s\n", relationRefText(rel.CountryRef, rel.CountryDuplicate))
			writeRelationMetrics(&b, rel)
			continue
		}
		neighbor, ok := gs.Country[*rel.CountryID]
		if !ok {
			fmt.Fprintf(&b, "\nContact country %d: referenced record unavailable\n", *rel.CountryID)
			writeRelationMetrics(&b, rel)
			continue
		}
		fmt.Fprintf(&b, "\n%s (Country %d):\n", name(neighbor.Adjective), *rel.CountryID)
		writeRelationMetrics(&b, rel)
		fmt.Fprintf(&b, "  Saved military=%s economy=%s tech=%s\n", savedFloat(neighbor.MilitaryPowerRaw), savedFloat(neighbor.EconomyPowerRaw), savedFloat(neighbor.TechPowerRaw))
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

func renderNeighborRelations(gs *gamestate.GameState, countryID int, country *gamestate.Country, neighborID int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved bilateral relations (%s, country %d, other country %d) ===\n", name(country.Adjective), countryID, neighborID)
	fmt.Fprintln(&b, "Each direction follows a saved relations_manager. Truce timing, current diplomatic effects and diplo_action_dates positions remain unverified.")
	writeRelationDirection(&b, countryID, neighborID, country.RelationsManager)
	if neighborID != countryID {
		if neighbor, ok := gs.Country[neighborID]; ok {
			writeRelationDirection(&b, neighborID, countryID, neighbor.RelationsManager)
		} else {
			fmt.Fprintf(&b, "\nReverse direction unavailable. Country record %d is unavailable.\n", neighborID)
		}
	}
	return mcp.NewToolResultText(b.String())
}

func writeRelationDirection(b *strings.Builder, from, to int, manager gamestate.RelationsManager) {
	fmt.Fprintf(b, "\nCountry %d relations_manager targeting country %d\n", from, to)
	if !writeRelationManagerState(b, manager) {
		return
	}
	var positions []int
	var unsupported, duplicate, missing int
	for i, rel := range manager.Relation {
		switch {
		case rel.Unsupported || rel.CountryRef.Unsupported:
			unsupported++
		case rel.CountryDuplicate:
			duplicate++
		case rel.CountryID == nil:
			missing++
		case *rel.CountryID == to:
			positions = append(positions, i)
		}
	}
	if unsupported+duplicate+missing > 0 {
		fmt.Fprintf(b, "  Unscoped rows: unsupported=%d duplicate country fields=%d country not recorded=%d. Use raw tools.\n", unsupported, duplicate, missing)
	}
	switch {
	case len(positions) == 0:
		fmt.Fprintln(b, "  No matching saved relation row.")
	case len(positions) > 1:
		fmt.Fprintf(b, "  Duplicate target rows at saved positions %v. Rows are shown separately.\n", positions)
	}
	for _, position := range positions {
		rel := manager.Relation[position]
		fmt.Fprintf(b, "  Row %d owner=%s country=%d contact=%s\n", position, relationRefText(rel.OwnerRef, rel.OwnerDuplicate), to, relationMetricText(rel.Contact, rel.ContactDuplicate))
		if rel.OwnerRef.Value != nil && *rel.OwnerRef.Value != from {
			fmt.Fprintln(b, "  Saved owner differs from the containing country.")
		}
		writeRelationMetrics(b, rel)
		seen := map[string]bool{}
		for _, pair := range rel.SavedDetails {
			label := "  " + pair.Key
			if seen[pair.Key] {
				if pair.Key == "modifier" {
					label += " [additional saved modifier]"
				} else {
					label += " [duplicate saved key]"
				}
			}
			seen[pair.Key] = true
			writeSavedValue(b, label, pair.Value, 0)
		}
		if !seen["truce"] {
			fmt.Fprintln(b, "  truce: not recorded")
		}
	}
}

func writeRelationManagerState(b *strings.Builder, manager gamestate.RelationsManager) bool {
	switch {
	case manager.Duplicate:
		fmt.Fprintln(b, "  relations_manager has duplicate saved fields. Use raw tools.")
	case manager.Unsupported:
		fmt.Fprintln(b, "  relations_manager has an unsupported saved shape. Use raw tools.")
	case !manager.Recorded:
		fmt.Fprintln(b, "  relations_manager not recorded.")
	default:
		return true
	}
	return false
}

func relationMetricText(field gamestate.OptionalString, duplicate bool) string {
	switch {
	case duplicate:
		return "has duplicate saved fields"
	case field.Unsupported:
		return "has an unsupported saved shape"
	case !field.Recorded:
		return "not recorded"
	default:
		return field.Value
	}
}

func relationRefText(ref gamestate.OptionalIntRef, duplicate bool) string {
	switch {
	case duplicate:
		return "has duplicate saved fields"
	case ref.Unsupported:
		return "has an unsupported saved shape"
	default:
		return savedInt(ref.Value)
	}
}

func writeRelationMetrics(b *strings.Builder, rel gamestate.Relation) {
	fmt.Fprintf(b, "  Saved relation_current=%s trust=%s\n", relationMetricText(rel.RelationCurrent, rel.CurrentDuplicate), relationMetricText(rel.Trust, rel.TrustDuplicate))
}
