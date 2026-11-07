package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerRelics(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_relics", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read the selected country's saved relics keys, last_activated_relic and last_received_relic. Preserves order, duplicate keys, and raw last_* values. Last-field units and current relic activation eligibility remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player"))), handleRelics)
}

func handleRelics(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderRelics(id, country), nil
}

func renderRelics(countryID int, country *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved relics (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Source is the selected country's saved record. Last-field units, per-relic association and current activation eligibility remain unverified.")
	fmt.Fprintln(&b, "relics (saved order)")
	relics := country.Relics
	switch {
	case relics.Unsupported:
		fmt.Fprintln(&b, "  Unsupported saved shape or duplicate relics fields; use raw tree/byte tools")
	case !relics.Recorded:
		fmt.Fprintln(&b, "  Not recorded")
	case len(relics.Values) == 0:
		fmt.Fprintln(&b, "  Empty")
	}
	seen := map[string]bool{}
	for index, key := range relics.Values {
		fmt.Fprintf(&b, "  Entry %d key=%q", index, key)
		if loc.Has(key) {
			fmt.Fprintf(&b, " name=%q", l(key))
		}
		if seen[key] {
			fmt.Fprint(&b, " [duplicate saved key]")
		}
		seen[key] = true
		fmt.Fprintln(&b)
	}
	for _, entry := range []struct {
		key   string
		value gamestate.OptionalString
	}{
		{"last_activated_relic", country.LastActivatedRelic},
		{"last_received_relic", country.LastReceivedRelic},
	} {
		switch {
		case entry.value.Unsupported:
			fmt.Fprintf(&b, "%s unsupported or duplicate; use raw tree/byte tools\n", entry.key)
		case !entry.value.Recorded:
			fmt.Fprintf(&b, "%s not recorded\n", entry.key)
		default:
			fmt.Fprintf(&b, "%s=%q (saved scalar)\n", entry.key, entry.value.Value)
		}
	}
	return mcp.NewToolResultText(b.String())
}
