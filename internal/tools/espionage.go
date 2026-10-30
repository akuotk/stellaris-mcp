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

func registerEspionage(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_espionage", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Get saved spy network records and raw power; infiltration conversion is not verified"), mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save"))), handleEspionage)
}

func handleEspionage(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderEspionage(gs, id, c), nil
}

func renderEspionage(gs *gamestate.GameState, countryID int, c *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Espionage (%s) ===\n", name(c.Adjective))
	ids := make([]int, 0, len(gs.SpyNetworks))
	for id := range gs.SpyNetworks {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	count := 0
	for _, id := range ids {
		n := gs.SpyNetworks[id]
		if (n.Owner == nil || *n.Owner != countryID) && (n.Target == nil || *n.Target != countryID) {
			continue
		}
		count++
		party := func(value *int) string {
			if value == nil {
				return "not recorded"
			}
			return fmt.Sprintf("%s (ID %d)", countryName(gs, *value), *value)
		}
		fmt.Fprintf(&b, "\nNetwork ID %d: owner %s; target %s\n", id, party(n.Owner), party(n.Target))
		if n.Power == nil {
			fmt.Fprintln(&b, "  Network power: not recorded")
		} else {
			fmt.Fprintf(&b, "  Network power (raw save value; infiltration conversion unknown): %g\n", *n.Power)
		}
		if n.Leader != nil {
			if leader, ok := gs.Leaders[*n.Leader]; ok {
				fmt.Fprintf(&b, "  Leader: %s (ID %d)\n", name(leader.Name.FullNames), *n.Leader)
			} else {
				fmt.Fprintf(&b, "  Leader ID: %d (identity unavailable)\n", *n.Leader)
			}
		}
		if n.Formed != "" {
			fmt.Fprintf(&b, "  Formed: %s\n", n.Formed)
		}
		if len(n.ActiveOperations) > 0 {
			fmt.Fprintf(&b, "  Recorded operation IDs: %v\n", n.ActiveOperations)
		}
	}
	if count == 0 {
		fmt.Fprintln(&b, "No matching network records.")
	}
	return mcp.NewToolResultText(b.String())
}
