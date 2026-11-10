package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerCountryControlGroups(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_country_control_groups",
		mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through one country's saved control group slots and list entries in source order. The saved id field is compared separately with fleet, ship and ship-design keys. These numeric candidates leave the meaning of type, UI bindings and ownership unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games. Omit for latest.")),
		mcp.WithNumber("country_id", mcp.Description("Country ID. If omitted, every saved player record must refer to the same unique country.")),
		mcp.WithNumber("offset", mcp.Description("Zero-based saved row offset. Up to 20 rows per page.")),
		mcp.WithString("page_context", mcp.Description("Token required for later pages. Continue with the same save and country."))),
		handleCountryControlGroups)
}

func handleCountryControlGroups(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	for _, key := range []string{"save_id", "page_context"} {
		if value, ok := req.GetArguments()[key]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	var countryID *int
	if _, ok := req.GetArguments()["country_id"]; ok {
		maxID := int(min(int64(^uint(0)>>1), int64(1<<53-1)))
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
	saved, err := gamestate.LoadCountryControlGroupsFromArchive(ctx, archive.zip, countryID, offset)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, saved.EntryIndex, "get_country_control_groups", strconv.Itoa(saved.CountryID))
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	rows := make([]map[string]any, 0, len(saved.Rows))
	for _, row := range saved.Rows {
		var key *string
		if row.Node.Key != nil {
			key = &row.Node.Key.Text
		}
		entry := rawNode(row.Path, key, row.Node)
		entry["role"], entry["child_index"] = row.Role, row.Index
		entry["container_path"], entry["container_kind"] = row.ContainerPath, row.ContainerKind
		if row.SlotPath != "" {
			entry["slot_path"] = row.SlotPath
		}
		if row.ID.Status != "" {
			entry["id_field"], entry["type_field"] = rawControlGroupField(row.ID), rawControlGroupField(row.Type)
		}
		if len(row.NumericCandidates) > 0 {
			entry["numeric_candidates"] = row.NumericCandidates
		}
		rows = append(rows, entry)
	}
	var next *int
	if end := offset + len(rows); end < saved.RowCount {
		next = &end
	}
	result := rawTreePage(archive, saved.EntryIndex)
	result["country_id"], result["country_path"] = saved.CountryID, saved.CountryPath
	result["rows"], result["row_count"], result["field_occurrences"] = rows, saved.RowCount, saved.FieldOccurrences
	result["candidate_tables"] = saved.Tables
	result["offset"], result["next_offset"], result["page_context"] = offset, next, token
	result["interpretation"] = "Candidates compare the saved id value with table keys. The meaning of slot keys, type values, UI bindings and ownership is unverified."
	return rawJSON(result)
}

func rawControlGroupField(field gamestate.ControlGroupField) map[string]any {
	entries := make([]map[string]any, 0, len(field.Preview))
	for _, entry := range field.Preview {
		entries = append(entries, rawNode(entry.Path, &entry.Node.Key.Text, entry.Node))
	}
	return map[string]any{
		"status": field.Status, "occurrences": field.Occurrences,
		"entries_preview": entries, "preview_truncated": len(entries) < field.Occurrences,
	}
}
