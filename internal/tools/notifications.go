package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerNotifications(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_notifications", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Get saved war summaries, federations, and game status. Use get_wars for battle details. War activity is not inferred from missing dates."),
			mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save")),
		),
		handleNotifications,
	)
}

func handleNotifications(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	_, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}

	return renderNotifications(gs, c), nil
}

func renderNotifications(gs *gamestate.GameState, c *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Game Status (%s — %s) ===\n", name(c.Adjective), gs.Date)
	fmt.Fprintln(&b, "\nPlayers:")
	if gs.Player == nil {
		fmt.Fprintln(&b, "  not recorded")
	} else if len(gs.Player) == 0 {
		fmt.Fprintln(&b, "  empty")
	}
	for _, p := range gs.Player {
		fmt.Fprintf(&b, "  - %s (country reference: %s)\n", p.Name, armyInt(p.CountryID))
	}

	fmt.Fprintln(&b, "\nSaved War Records (activity not inferred from dates):")
	writeSavedWarSummary(&b, gs, nil)
	writeFederations(&b, gs)
	contacts, unknown := 0, 0
	for _, rel := range c.RelationsManager.Relation {
		if rel.ContactRaw == nil || (*rel.ContactRaw != "yes" && *rel.ContactRaw != "no") {
			unknown++
		} else if *rel.ContactRaw == "yes" {
			contacts++
		}
	}
	fmt.Fprintf(&b, "\nDiplomacy:\n  Saved contact=yes relations: %d\n  Contact status not established: %d\n  Victory rank: %s\n", contacts, unknown, armyInt(c.VictoryRankRaw))
	return mcp.NewToolResultText(b.String())
}
