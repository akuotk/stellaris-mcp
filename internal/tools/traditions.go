package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTraditionsAscension(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_traditions_ascension", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Get adopted traditions and ascension perks"),
			mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save")),
		),
		handleTraditionsAscension,
	)
}

func handleTraditionsAscension(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	_, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Traditions & Ascension Perks (%s) ===\n", name(c.Adjective))

	writeSavedTraditionList(&b, "Traditions", c.Traditions)
	writeSavedTraditionList(&b, "Ascension perks", c.AscensionPerks)
	writeSavedTraditionList(&b, "Tradition categories (saved keys; completion not inferred)", c.TraditionCategories)
	if !c.LastPickedTradition.Recorded {
		fmt.Fprintln(&b, "\nLast picked tradition: not recorded")
	} else if c.LastPickedTradition.Unsupported {
		fmt.Fprintln(&b, "\nLast picked tradition: unsupported shape; inspect raw save tree")
	} else {
		fmt.Fprintf(&b, "\nLast picked tradition: %s [%s]\n", l(c.LastPickedTradition.Value), c.LastPickedTradition.Value)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func writeSavedTraditionList(b *strings.Builder, label string, list gamestate.OptionalStringList) {
	fmt.Fprintf(b, "\n%s:\n", label)
	if !list.Recorded {
		fmt.Fprintln(b, "  not recorded")
		return
	}
	if list.Unsupported {
		fmt.Fprintln(b, "  unsupported shape; inspect raw save tree")
		return
	}
	if len(list.Values) == 0 {
		fmt.Fprintln(b, "  empty")
		return
	}
	for _, key := range list.Values {
		fmt.Fprintf(b, "  - %s [%s]\n", l(key), key)
	}
}
