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

const countryTimedModifierPageSize = 20

func registerCountryTimedModifiers(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_country_timed_modifiers", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through one country's saved timed_modifier items and unusual saved entries in source order. Includes positional paths and original value previews. Saved values do not establish units, remaining duration, applied effects or current UI state."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID. If omitted, all saved player records must refer to one country.")),
		mcp.WithNumber("offset", mcp.Description("First saved entry, zero-based; up to 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from the first page; required for later pages"))), handleCountryTimedModifiers)
}

func handleCountryTimedModifiers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	data, err := gamestate.LoadCountryTimedModifiersFromArchive(ctx, archive.zip, countryID)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, -1, "get_country_timed_modifiers", strconv.Itoa(data.CountryID))
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	if offset > len(data.Entries) {
		return toolError(fmt.Errorf("offset exceeds saved entry count")), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved country timed modifiers ===\nsave_id=%s\nrevision=%s\ncountry_id=%d\npage_context=%s\n", archive.saveID, archive.revision, data.CountryID, token)
	fmt.Fprintf(&b, "timed_modifier_occurrences=%d items_occurrences=%d duplicate_items_wrappers=%d saved_items=%d saved_entries=%d offset=%d\n",
		data.WrapperCount, data.ItemsCount, data.DuplicateItemsWrappers, data.ItemCount, len(data.Entries), offset)
	fmt.Fprintln(&b, "Use the returned save_id and revision with read_save_tree (path) or read_save_bytes (offset=source_offset, limit up to source_bytes) for the uncompressed gamestate. Saved values alone do not establish units, remaining duration, effect application or current UI state.")
	end := offset + min(countryTimedModifierPageSize, len(data.Entries)-offset)
	for i := offset; i < end; i++ {
		entry := data.Entries[i]
		fmt.Fprintf(&b, "entry[%d] role=%s container_kind=%s ", i, entry.Kind, entry.ContainerKind)
		writeCountryTimedModifierRaw(&b, entry.Path, entry.Node, data.Source, 384)
		if entry.Kind != "item" {
			continue
		}
		fields := map[string]int{}
		repeated := false
		for _, child := range entry.Node.Children {
			if child.Key != nil {
				fields[child.Key.Text]++
				if child.Key.Text == "modifier" && child.Scalar != nil && data.KeyCounts[child.Scalar.Text] > 1 {
					repeated = true
				}
			}
		}
		fmt.Fprintf(&b, "  modifier_fields=%d repeated_saved_key=%t\n", fields["modifier"], repeated)
		const fieldPreviewCount = 8
		for j, child := range entry.Node.Children[:min(fieldPreviewCount, len(entry.Node.Children))] {
			fmt.Fprintf(&b, "  field[%d] ", j)
			if child.Key != nil && fields[child.Key.Text] > 1 {
				fmt.Fprint(&b, "duplicate_field_key=true ")
			}
			if child.Key != nil && child.Key.Text == "modifier" && child.Scalar != nil {
				if count := data.KeyCounts[child.Scalar.Text]; count > 1 {
					fmt.Fprintf(&b, "repeated_saved_key=true saved_key_occurrences=%d ", count)
				}
			}
			writeCountryTimedModifierRaw(&b, entry.Path+"/"+strconv.Itoa(j), child, data.Source, 128)
		}
		if remaining := len(entry.Node.Children) - fieldPreviewCount; remaining > 0 {
			fmt.Fprintf(&b, "  fields_without_inline_preview=%d; inspect entry path for all fields\n", remaining)
		}
	}
	if end < len(data.Entries) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}

func writeCountryTimedModifierRaw(b *strings.Builder, path string, node *clausewitz.RawNode, source []byte, limit int) {
	size := node.End - node.EntryStart
	fmt.Fprintf(b, "path=%q kind=%s source_offset=%d source_bytes=%d raw_preview=%q",
		path, node.Kind, node.EntryStart, size, source[node.EntryStart:node.EntryStart+min(size, limit)])
	if size > limit {
		fmt.Fprint(b, " raw_preview_truncated=true")
	}
	fmt.Fprintln(b)
}
