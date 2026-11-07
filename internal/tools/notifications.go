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
			mcp.WithDescription("Get saved war records, federations, and game status; war activity is not inferred from missing dates"),
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
	if gs.War == nil {
		fmt.Fprintln(&b, "  not recorded")
	} else if len(gs.War) == 0 {
		fmt.Fprintln(&b, "  empty")
	}
	warIDs := make([]int, 0, len(gs.War))
	for id := range gs.War {
		warIDs = append(warIDs, id)
	}
	sort.Ints(warIDs)
	for _, id := range warIDs {
		w := gs.War[id]
		fmt.Fprintf(&b, "  - ID %d: %s (saved start=%s, saved end=%s)\n", id, name(w.Name), governanceValue(w.StartDateRaw), governanceValue(w.EndDateRaw))
		for _, side := range []struct {
			label   string
			parties []gamestate.WarParty
		}{{"Attackers", w.Attackers}, {"Defenders", w.Defenders}} {
			if side.parties == nil {
				fmt.Fprintf(&b, "    %s: not recorded\n", side.label)
				continue
			}
			if len(side.parties) == 0 {
				fmt.Fprintf(&b, "    %s: empty\n", side.label)
				continue
			}
			var names []string
			for _, party := range side.parties {
				if party.CountryID == nil {
					names = append(names, "country reference not recorded")
					continue
				}
				if _, ok := gs.Country[*party.CountryID]; !ok {
					names = append(names, fmt.Sprintf("Country %d (record unavailable)", *party.CountryID))
				} else {
					names = append(names, fmt.Sprintf("%s (ID %d)", countryName(gs, *party.CountryID), *party.CountryID))
				}
			}
			fmt.Fprintf(&b, "    %s: %s\n", side.label, strings.Join(names, ", "))
		}
	}
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
