package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGalacticCommunity(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_galactic_community", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Show saved galactic community members and proposed/passed resolution records, with supporters and opponents")), handleGalacticCommunity)
}

func handleGalacticCommunity(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	return renderCommunity(gs), nil
}

func renderCommunity(gs *gamestate.GameState) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintln(&b, "=== Galactic community (saved records) ===")
	if gs.Community == nil {
		fmt.Fprintln(&b, "Community data absent from this save.")
		return mcp.NewToolResultText(b.String())
	}
	c := gs.Community
	fmt.Fprintf(&b, "Formed: %s\nMembers (%d):\n", c.Formed, len(c.Members))
	for _, id := range c.Members {
		fmt.Fprintf(&b, "  %s (ID %d)\n", countryName(gs, id), id)
	}
	for _, group := range []struct {
		label string
		ids   []int
	}{{"Passed IDs", c.Passed}, {"Proposed IDs", c.Proposed}} {
		fmt.Fprintf(&b, "\n%s (%d):\n", group.label, len(group.ids))
		for _, id := range group.ids {
			r, ok := gs.Resolutions[id]
			if !ok {
				fmt.Fprintf(&b, "  ID %d: record unavailable\n", id)
				continue
			}
			fmt.Fprintf(&b, "  ID %d: %s [%s]", id, l(r.Type), r.Type)
			if r.Country != nil {
				fmt.Fprintf(&b, ", country=%s (%d)", countryName(gs, *r.Country), *r.Country)
			}
			if r.PassDate != "" {
				fmt.Fprintf(&b, ", pass_date=%s", r.PassDate)
			}
			fmt.Fprintln(&b)
			for _, side := range []struct {
				label string
				ids   []int
			}{{"supporters", r.Supporters}, {"opponents", r.Opponents}} {
				if len(side.ids) == 0 {
					continue
				}
				var names []string
				for _, cid := range side.ids {
					names = append(names, fmt.Sprintf("%s (%d)", countryName(gs, cid), cid))
				}
				fmt.Fprintf(&b, "    %s: %s\n", side.label, strings.Join(names, ", "))
			}
		}
	}
	return mcp.NewToolResultText(b.String())
}
