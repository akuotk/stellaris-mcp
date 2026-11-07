package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGrandArchive(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_grand_archive", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read a country's saved Grand Archive exhibits, auto_culling_settings, culling_value and specimen_management_settings. Distinguishes missing sections from empty records and keeps saved order. Specimen identity, ownership, display status and setting effects remain unverified. Raw tree/byte tools expose deeply nested or mixed payloads."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player"))), handleGrandArchive)
}

func handleGrandArchive(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderGrandArchive(id, country), nil
}

func renderGrandArchive(countryID int, country *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved Grand Archive records (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Source is country.modules.standard_grand_archive_module. Specimen identity, ownership, display status and setting effects remain unverified.")
	modules := country.GrandArchiveModules
	switch {
	case modules.Unsupported:
		fmt.Fprintln(&b, "Unsupported modules shape or duplicate Grand Archive module; use raw tree/byte tools")
	case !modules.Recorded:
		fmt.Fprintln(&b, "country.modules not recorded")
	case modules.Archive == nil:
		fmt.Fprintln(&b, "standard_grand_archive_module not recorded")
	case modules.Archive.Unsupported:
		fmt.Fprintln(&b, "Unsupported Grand Archive module shape or duplicate known fields; use raw tree/byte tools")
	default:
		archive := modules.Archive
		for _, section := range []struct {
			key   string
			value *gamestate.GrandArchiveValue
		}{
			{"exhibits", archive.Exhibits},
			{"auto_culling_settings", archive.AutoCulling},
			{"culling_value", archive.CullingValue},
			{"specimen_management_settings", archive.SpecimenSettings},
		} {
			if section.value == nil {
				fmt.Fprintf(&b, "%s: not recorded\n", section.key)
				continue
			}
			writeSavedValue(&b, section.key, section.value.Value, 0)
		}
	}
	return mcp.NewToolResultText(b.String())
}

func writeSavedValue(b *strings.Builder, label string, value clausewitz.Value, depth int) {
	fmt.Fprintf(b, "%s%s: ", strings.Repeat("  ", depth), label)
	switch {
	case value.IsScalar():
		fmt.Fprintf(b, "%q (saved scalar)\n", value.Scalar)
	case value.IsObject() && len(value.Object.Pairs) == 0:
		fmt.Fprintln(b, "empty object")
	case value.IsList() && len(value.List) == 0:
		fmt.Fprintln(b, "empty list")
	case value.Mixed:
		fmt.Fprintln(b, "mixed saved block; use raw tree/byte tools for its ordered contents")
	case depth == 2:
		// shortcut: display two child levels, use raw tools when deeper payloads need inspection.
		fmt.Fprintln(b, "nested saved block; use raw tree/byte tools for contents")
	case value.IsObject():
		fmt.Fprintf(b, "object with %d saved fields\n", len(value.Object.Pairs))
		seen := map[string]bool{}
		for index, pair := range value.Object.Pairs {
			child := fmt.Sprintf("Field %d key=%q", index, pair.Key)
			if seen[pair.Key] {
				child += " [duplicate saved key]"
			}
			seen[pair.Key] = true
			writeSavedValue(b, child, pair.Value, depth+1)
		}
	case value.IsList():
		allEmptyObjects := true
		for _, item := range value.List {
			if !item.IsObject() || len(item.Object.Pairs) != 0 {
				allEmptyObjects = false
				break
			}
		}
		if allEmptyObjects {
			fmt.Fprintf(b, "list of %d empty objects\n", len(value.List))
			return
		}
		fmt.Fprintf(b, "list with %d saved entries\n", len(value.List))
		for index, item := range value.List {
			writeSavedValue(b, fmt.Sprintf("Item %d", index), item, depth+1)
		}
	}
}
