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

func registerEmpireAssets(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_empire_assets", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("List this country's saved agreements, situations and megastructures; values not interpreted are labeled raw"), mcp.WithNumber("country_id", mcp.Description("Country ID (default 0)"), mcp.DefaultNumber(0))), handleEmpireAssets)
}

func handleEmpireAssets(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadLatestSave()
	if err != nil {
		return toolError(err), nil
	}
	id, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderEmpireAssets(gs, id, c), nil
}

func renderEmpireAssets(gs *gamestate.GameState, id int, c *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Agreements, situations and megastructures (%s) ===\n", name(c.Adjective))
	ids := make([]int, 0, len(gs.Agreements.Items))
	for k := range gs.Agreements.Items {
		ids = append(ids, k)
	}
	sort.Ints(ids)
	fmt.Fprintln(&b, "\nAgreements (owner/target as recorded; roles not inferred):")
	count := 0
	for _, k := range ids {
		a := gs.Agreements.Items[k]
		if a.Owner == nil || a.Target == nil || (*a.Owner != id && *a.Target != id) {
			continue
		}
		count++
		fmt.Fprintf(&b, "  ID %d: owner %s (%d), target %s (%d); status=%s, preset=%s, added=%s\n", k, countryName(gs, *a.Owner), *a.Owner, countryName(gs, *a.Target), *a.Target, a.Status, a.Terms.Preset, a.DateAdded)
	}
	if count == 0 {
		fmt.Fprintln(&b, "  No matching records.")
	}
	ids = ids[:0]
	for k := range gs.Situations.Items {
		ids = append(ids, k)
	}
	sort.Ints(ids)
	count = 0
	fmt.Fprintln(&b, "\nSituations (progress in raw save units):")
	for _, k := range ids {
		v := gs.Situations.Items[k]
		if v.Country == nil || *v.Country != id {
			continue
		}
		count++
		fmt.Fprintf(&b, "  ID %d: %s [%s]; progress=%.2f, last month=%.2f, approach=%s\n", k, l(v.Type), v.Type, v.Progress, v.LastMonthProgress, v.Approach)
	}
	if count == 0 {
		fmt.Fprintln(&b, "  No matching records.")
	}
	ids = ids[:0]
	for k := range gs.Megastructures {
		ids = append(ids, k)
	}
	sort.Ints(ids)
	count = 0
	fmt.Fprintln(&b, "\nMegastructures:")
	for _, k := range ids {
		v := gs.Megastructures[k]
		if v.Owner == nil || *v.Owner != id {
			continue
		}
		count++
		system := "system unavailable"
		if v.Coordinate.Origin != nil {
			system = fmt.Sprintf("system ID %d", *v.Coordinate.Origin)
			if s, ok := gs.Systems[*v.Coordinate.Origin]; ok {
				system = fmt.Sprintf("%s (system ID %d)", name(s.Name), *v.Coordinate.Origin)
			}
		}
		fmt.Fprintf(&b, "  ID %d: %s [%s], %s\n", k, l(v.Type), v.Type, system)
	}
	if count == 0 {
		fmt.Fprintln(&b, "  No matching records.")
	}
	return mcp.NewToolResultText(b.String())
}
