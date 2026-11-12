package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerCasusBelli(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_casus_belli", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read saved casus_belli entries under a country's standard/exclusive diplomacy modules. Keeps module source, saved order, owner/country references, type and days_left. Shows missing countries and owner/containing-country mismatches. Current war-declaration eligibility, country roles and days_left semantics remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player"))), handleCasusBelli)
}

func handleCasusBelli(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderCasusBelli(gs, id, country), nil
}

func renderCasusBelli(gs *gamestate.GameState, countryID int, country *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved casus_belli records (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope follows the containing country's diplomacy modules. Owner/country roles, current war-declaration eligibility and days_left semantics remain unverified.")
	modules := country.DiplomaticModules
	if modules.Unsupported {
		fmt.Fprintln(&b, "country.modules has an unsupported shape or duplicate diplomacy modules; use raw tree/byte tools")
		return mcp.NewToolResultText(b.String())
	}
	if !modules.Recorded {
		fmt.Fprintln(&b, "country.modules not recorded")
		return mcp.NewToolResultText(b.String())
	}
	for _, source := range []struct {
		key    string
		module gamestate.CasusBelliModule
	}{{"standard_diplomacy_module", modules.Standard}, {"exclusive_diplomacy_module", modules.Exclusive}} {
		fmt.Fprintf(&b, "\ncountry.modules.%s.casus_belli (saved order)\n", source.key)
		module := source.module
		switch {
		case module.Unsupported || module.CasusBelli.Unsupported:
			fmt.Fprintln(&b, "  Unsupported shape or duplicate known fields; use raw tree/byte tools")
		case !module.Recorded:
			fmt.Fprintln(&b, "  Diplomacy module not recorded")
		case !module.CasusBelli.Recorded:
			fmt.Fprintln(&b, "  casus_belli not recorded")
		case len(module.CasusBelli.Items) == 0:
			fmt.Fprintln(&b, "  Empty")
		}
		for index, entry := range module.CasusBelli.Items {
			fmt.Fprintf(&b, "  Entry %d owner=%s country=%s type=%s days_left=%s (saved values)\n", index, savedInt(entry.Owner), savedInt(entry.Country), governanceValue(entry.Type), governanceValue(entry.DaysLeft))
			if entry.Owner != nil && *entry.Owner != countryID {
				fmt.Fprintln(&b, "    owner differs from containing country")
			}
			for _, ref := range []struct {
				key string
				id  *int
			}{{"owner", entry.Owner}, {"country", entry.Country}} {
				if ref.id == nil {
					fmt.Fprintf(&b, "    %s country reference not recorded\n", ref.key)
					continue
				}
				if c, ok := gs.Country[*ref.id]; ok {
					fmt.Fprintf(&b, "    Saved %s=%d resolves to country record: %s\n", ref.key, *ref.id, name(c.Adjective))
				} else {
					fmt.Fprintf(&b, "    Saved %s=%d: referenced country record unavailable\n", ref.key, *ref.id)
				}
			}
		}
	}
	return mcp.NewToolResultText(b.String())
}
