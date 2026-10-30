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

func registerNotifications(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_notifications", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Get active wars, federations, and game status"),
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

	var b strings.Builder
	fmt.Fprintf(&b, "=== Game Status (%s — %s) ===\n", name(c.Adjective), gs.Date)

	// Players
	fmt.Fprintf(&b, "\nPlayers:\n")
	if len(gs.Player) == 0 {
		fmt.Fprintf(&b, "  (none)\n")
	}
	for _, p := range gs.Player {
		fmt.Fprintf(&b, "  - %s (Country %d)\n", p.Name, p.Country)
	}

	// Wars (only show wars without an end_date)
	fmt.Fprintf(&b, "\nActive Wars:\n")
	activeWars := 0
	warIDs := make([]int, 0, len(gs.War))
	for id := range gs.War {
		warIDs = append(warIDs, id)
	}
	sort.Ints(warIDs)
	for _, id := range warIDs {
		w := gs.War[id]
		if w.EndDate != "" {
			continue
		}
		activeWars++
		fmt.Fprintf(&b, "  - ID %d: %s (started %s)\n", id, name(w.Name), w.StartDate)
		for _, side := range []struct {
			label   string
			parties []gamestate.WarParty
		}{{"Attackers", w.Attackers}, {"Defenders", w.Defenders}} {
			if len(side.parties) == 0 {
				continue
			}
			var names []string
			for _, p := range side.parties {
				names = append(names, fmt.Sprintf("%s (ID %d)", countryName(gs, p.Country), p.Country))
			}
			fmt.Fprintf(&b, "    %s: %s\n", side.label, strings.Join(names, ", "))
		}
	}
	if activeWars == 0 {
		fmt.Fprintf(&b, "  No active wars.\n")
	}

	// Federations
	fmt.Fprintf(&b, "\nFederations:\n")
	if len(gs.Federation) == 0 {
		fmt.Fprintf(&b, "  No federations.\n")
	}
	for _, f := range gs.Federation {
		fedType := l(f.FederationProgression.FederationType)
		fmt.Fprintf(&b, "  - %s (%s, leader: %s, %d members, cohesion: %.0f%%)\n",
			name(f.Name), fedType, countryName(gs, f.Leader),
			len(f.Members), f.FederationProgression.Cohesion)
	}

	// Known contacts count
	contacts := 0
	for _, rel := range c.RelationsManager.Relation {
		if rel.Contact {
			contacts++
		}
	}
	fmt.Fprintf(&b, "\nDiplomacy:\n")
	fmt.Fprintf(&b, "  Known empires: %d\n", contacts)
	fmt.Fprintf(&b, "  Victory rank: %d\n", c.VictoryRank)

	return mcp.NewToolResultText(b.String()), nil
}
