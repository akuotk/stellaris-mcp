package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const completedTechnologyPageSize = 50

func registerCompletedTechnologies(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_completed_technologies", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through one saved country's completed technology fields in order, with adjacent integer levels and raw fallback for unusual data."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID. If omitted, all saved player records must refer to one country.")),
		mcp.WithNumber("offset", mcp.Description("First saved entry, zero-based; 50 per page")),
		mcp.WithString("page_context", mcp.Description("Token from the first page; required for later pages"))), handleCompletedTechnologies)
}

func handleCompletedTechnologies(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	for _, key := range []string{"save_id", "page_context"} {
		if value, ok := req.GetArguments()[key]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	var countryID *int
	if _, ok := req.GetArguments()["country_id"]; ok {
		maxID := int(min(int64(^uint(0)>>1), int64(9007199254740991)))
		id, err := rawInteger(req, "country_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		countryID = &id
	}
	offset, err := rawInteger(req, "offset", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	if offset > 0 && req.GetString("page_context", "") == "" {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(fmt.Errorf("cannot open selected save")), nil
	}
	data, err := gamestate.LoadCompletedTechnologiesFromArchive(archive.zip, countryID)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, -1, "get_completed_technologies", strconv.Itoa(data.CountryID))
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	if offset > len(data.Entries) {
		return toolError(fmt.Errorf("offset exceeds saved entry count")), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved completed technologies ===\nsave_id=%s\ncountry_id=%d\npage_context=%s\n", archive.saveID, data.CountryID, token)
	fmt.Fprintf(&b, "tech_status=%s tech_status_occurrences=%d unsupported_tech_status_occurrences=%d technology_entries=%d saved_entries=%d offset=%d\n",
		completedTechnologyStatus(data), data.TechStatusCount, data.UnsupportedCount, data.TechnologyCount, len(data.Entries), offset)
	fmt.Fprintln(&b, "An integer level is paired with a saved technology key only when it immediately follows the key in the same block. Paths select saved values in read_save_tree.")
	end := offset + min(completedTechnologyPageSize, len(data.Entries)-offset)
	for i := offset; i < end; i++ {
		entry := data.Entries[i]
		label := fmt.Sprintf("entry[%d] path=%q kind=%s", i, entry.Path, entry.Kind)
		if entry.RepeatedKey {
			label += " repeated_saved_key=true"
		}
		if entry.Kind == "technology" && entry.Key == "" {
			label += " technology_status=unsupported"
		}
		switch {
		case entry.Kind == "no-completed-fields":
			fmt.Fprintln(&b, label)
		case entry.Key == "" || len(entry.Key) > 512:
			writeCompletedTechnologyRaw(&b, label, entry.Value)
		default:
			fmt.Fprintf(&b, "%s technology_key=%q", label, entry.Key)
			if loc != nil && loc.Has(entry.Key) {
				fmt.Fprintf(&b, " localized_name=%q", loc.Resolve(entry.Key))
			} else {
				fmt.Fprint(&b, " localization=unavailable")
			}
			fmt.Fprintln(&b)
		}
		if entry.Kind != "technology" {
			continue
		}
		switch {
		case entry.Level != nil:
			fmt.Fprintf(&b, "  level=%d level_path=%q (adjacent saved integer)\n", *entry.Level, entry.LevelPath)
		case entry.AdjacentLevel != nil:
			writeCompletedTechnologyRaw(&b, fmt.Sprintf("  level_status=unresolved level_path=%q", entry.LevelPath), *entry.AdjacentLevel)
		default:
			fmt.Fprintln(&b, "  level_status=not-adjacent-or-not-recorded")
		}
	}
	if end < len(data.Entries) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}

func completedTechnologyStatus(data gamestate.SavedCompletedTechnologies) string {
	switch {
	case data.TechStatusCount == 0:
		return "not-recorded"
	case data.TechStatusCount > 1:
		return "duplicate"
	case data.UnsupportedCount > 0:
		return "unsupported"
	case len(data.Entries) == 1 && data.Entries[0].Kind == "empty-tech-status":
		return "empty"
	case data.TechnologyCount == 0:
		return "no-completed-technologies"
	default:
		return "recorded"
	}
}

func writeCompletedTechnologyRaw(b *strings.Builder, label string, value clausewitz.Value) {
	switch {
	case value.IsScalar() && len(value.Scalar) > 512:
		fmt.Fprintf(b, "%s: large saved scalar (%d bytes); use read_save_scalar\n", label, len(value.Scalar))
	case value.IsObject() && len(value.Object.Pairs) > 0:
		fmt.Fprintf(b, "%s: object with %d saved fields; use read_save_tree\n", label, len(value.Object.Pairs))
	case value.IsList() && len(value.List) > 0:
		fmt.Fprintf(b, "%s: list with %d saved entries; use read_save_tree\n", label, len(value.List))
	default:
		writeSavedValue(b, label, value, 0)
	}
}
