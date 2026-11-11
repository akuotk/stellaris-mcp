package tools

import (
	"context"
	"fmt"
	"io"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSavedRecordLinks(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_saved_record_links", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Walk any gamestate subtree at a positional path (from read_save_tree/search_save_tree) and compare every unquoted nonnegative integer under a recognized field name (leader, owner, planet, fleet, ship, species, system, build_queue, ...) with the matching root table, in saved depth-first order. Works for records without a dedicated tool (storms, ground_combat, trade_deal, focus_cards, saved_leaders, country AI blocks, ...). Field-name matches are heuristics; statuses report only whether a record with that numeric ID exists. Unmapped integer field names are counted so coverage gaps stay visible. 4294967295 is reported as uint32-max-value without lookup, fields directly under flags are never mapped, and galactic_object.colonies uses the colony table when present."),
		mcp.WithString("save_id", mcp.Description("ID from list_save_games; omitted selects latest")),
		mcp.WithString("revision", mcp.Required(), mcp.Description("SHA-256 returned by inspect_save_archive; stale revisions are rejected")),
		mcp.WithString("path", mcp.Required(), mcp.Description("Positional gamestate path, e.g. /12/3; empty walks the whole gamestate")),
		mcp.WithNumber("offset", mcp.DefaultNumber(0)),
		mcp.WithNumber("limit", mcp.Description("Rows per page, 1..200"), mcp.DefaultNumber(50)),
		mcp.WithString("page_context", mcp.Description("Token from a previous page; required for offset > 0"))), handleSavedRecordLinks)
}

func handleSavedRecordLinks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := req.GetArguments()["path"]; !ok {
		return toolError(fmt.Errorf("path is required; use an empty string for the whole gamestate")), nil
	}
	if _, ok := req.GetArguments()["entry_index"]; ok {
		return toolError(fmt.Errorf("entry_index is not supported; links are resolved within the gamestate entry")), nil
	}
	archive, index, f, err := openRawEntry(req)
	if err != nil {
		return toolError(err), nil
	}
	path := req.GetString("path", "")
	offset, err := rawInteger(req, "offset", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	limit, err := rawInteger(req, "limit", 50, 1, gamestate.SavedRecordLinksMaxLimit)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, index, "get_saved_record_links", path)
	if offset > 0 && req.GetString("page_context", "") == "" {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	stream, err := f.Open()
	if err != nil {
		return toolError(fmt.Errorf("open archive entry: %w", err)), nil
	}
	defer stream.Close()
	data, err := io.ReadAll(rawChunkReader{ctx, stream})
	if err != nil {
		return toolError(fmt.Errorf("read archive entry: %w", err)), nil
	}
	root, err := clausewitz.ParseRaw(ctx, data)
	if err != nil {
		return toolError(fmt.Errorf("semantic tree unsupported: %w; use read_save_bytes", err)), nil
	}
	node, key, err := rawAtPath(root, path)
	if err != nil {
		return toolError(err), nil
	}
	field, parent := gamestate.SavedNodeContext(root, path)
	links, err := gamestate.LinkSavedRecord(ctx, root, node, path, field, parent, offset, limit)
	if err != nil {
		return toolError(err), nil
	}
	if offset > links.RowCount {
		return toolError(fmt.Errorf("offset exceeds row count %d", links.RowCount)), nil
	}
	var next *int
	if end := offset + len(links.Rows); end < links.RowCount {
		next = &end
	}
	result := rawTreePage(archive, index)
	result["node"] = rawNode(path, key, node)
	result["named_field"], result["named_parent"] = field, parent
	result["rows"], result["row_count"], result["integer_scalars"] = links.Rows, links.RowCount, links.IntegerScalars
	result["summary"], result["tables_used"] = links.Summary, links.Tables
	if len(links.TablesNotPresent) > 0 {
		result["tables_not_recorded"] = links.TablesNotPresent
	}
	result["unmapped_integer_field_names"], result["unmapped_integer_fields_preview"] = links.UnmappedFields, links.Unmapped
	result["offset"], result["next_offset"], result["page_context"] = offset, next, token
	result["interpretation"] = "Tables are chosen by saved field name (or named parent) only. numeric-candidate means a unique object record with that ID exists; it does not establish that the field refers to it, nor ownership, visibility or current state. Quoted, negative, fractional and non-numeric scalars are skipped. 4294967295 is reported as uint32-max-value without lookup; fields directly under flags are never mapped."
	return rawJSON(result)
}
