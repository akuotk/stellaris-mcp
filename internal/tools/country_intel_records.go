package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const countryIntelPageSize = 20

func registerCountryIntelRecords(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_country_intel_records", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through saved country intelligence records or arrays in source order. Pair heads are compared to country and spy-network keys as numeric candidates. Array positions, intel and stale_intel are saved data with unverified meaning and units. Use the returned raw paths or byte ranges for full values."),
		mcp.WithString("kind", mcp.Enum(gamestate.CountryIntelManagerKind, gamestate.CountryIntelLevelKind, gamestate.CountryHighestIntelKind), mcp.DefaultString(gamestate.CountryIntelManagerKind)),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games. Omit for latest.")),
		mcp.WithNumber("country_id", mcp.Description("Country ID. If omitted, every saved player record must refer to the same unique country.")),
		mcp.WithNumber("offset", mcp.Description("Zero-based offset among saved rows. Up to 20 per page.")),
		mcp.WithString("page_context", mcp.Description("Token required for later pages. Continue with the same save, country and kind."))), handleCountryIntelRecords)
}

func handleCountryIntelRecords(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := requireStringArgs(req, "save_id", "kind", "page_context"); err != nil {
		return toolError(err), nil
	}
	kind := req.GetString("kind", gamestate.CountryIntelManagerKind)
	if err := gamestate.ValidateCountryIntelKind(kind); err != nil {
		return toolError(err), nil
	}
	var countryID *int
	if _, ok := req.GetArguments()["country_id"]; ok {
		id, err := rawInteger(req, "country_id", 0, 0, maxRecordID)
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
	saved, err := gamestate.LoadCountryIntelRecordsFromArchive(ctx, archive.zip, countryID, kind)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, saved.EntryIndex, "get_country_intel_records", strconv.Itoa(saved.CountryID), kind)
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	if offset > len(saved.Rows) {
		return toolError(fmt.Errorf("offset exceeds saved row count")), nil
	}
	end := offset + min(countryIntelPageSize, len(saved.Rows)-offset)
	rows := make([]map[string]any, 0, end-offset)
	for _, row := range saved.Rows[offset:end] {
		rows = append(rows, countryIntelRecordRow(row, &saved))
	}
	var next *int
	if end < len(saved.Rows) {
		next = &end
	}
	result := rawTreePage(archive, saved.EntryIndex)
	result["country_id"], result["country_path"], result["kind"] = saved.CountryID, saved.CountryPath, kind
	result["rows"], result["row_count"], result["field_occurrences"] = rows, len(saved.Rows), saved.FieldOccurrences
	result["offset"], result["next_offset"], result["page_context"] = offset, next, token
	if kind == gamestate.CountryIntelManagerKind {
		result["intel_occurrences"] = saved.IntelOccurrences
		tables := make([]map[string]any, 0, len(saved.Tables))
		for _, table := range saved.Tables {
			info := map[string]any{"table": table.Label, "status": table.Status, "path": table.Path, "unindexed_keys": table.UnindexedKeys}
			if len(table.DuplicatePaths) > 0 {
				info["duplicate_paths_preview"] = table.DuplicatePaths
			}
			tables = append(tables, info)
		}
		result["candidate_tables"] = tables
		result["interpretation"] = "Equal numbers are table-key candidates. The identifier namespace, intel units and current visibility are unverified."
	} else {
		result["interpretation"] = "Positions are saved child indexes. Country-ID mappings, links between arrays and element meanings are unverified."
	}
	return rawJSON(result)
}

func countryIntelRecordRow(row gamestate.CountryIntelRow, saved *gamestate.SavedCountryIntelRecords) map[string]any {
	var key *string
	if row.Node.Key != nil {
		key = &row.Node.Key.Text
	}
	entry := rawNode(row.Path, key, row.Node)
	entry["role"], entry["child_index"] = row.Role, row.Index
	entry["container_path"], entry["container_kind"] = row.ContainerPath, row.ContainerKind
	if (row.Role == "pair" || row.Role == "unsupported-pair") && row.Node.Kind == "list" && len(row.Node.Children) == 2 {
		head, payload := row.Node.Children[0], row.Node.Children[1]
		entry["head"] = rawNode(row.Path+"/0", nil, head)
		entry["payload"] = rawNode(row.Path+"/1", nil, payload)
		fields := make([]map[string]any, 0, 8)
		counts := map[string]int{"intel": 0, "stale_intel": 0}
		for i, child := range payload.Children {
			if child.Key == nil || (child.Key.Text != "intel" && child.Key.Text != "stale_intel") {
				continue
			}
			key := child.Key.Text
			counts[key]++
			if counts[key] <= 4 {
				fields = append(fields, rawNode(row.Path+"/1/"+strconv.Itoa(i), &key, child))
			}
		}
		entry["payload_fields_preview"], entry["payload_field_occurrences"] = fields, counts
		entry["payload_fields_preview_truncated"] = len(fields) < counts["intel"]+counts["stale_intel"]
	}
	if row.HeadID != nil {
		entry["head_occurrences"] = saved.HeadOccurrences[*row.HeadID]
		candidates := make([]map[string]any, 0, len(saved.Tables))
		for _, candidate := range saved.Candidates(*row.HeadID) {
			info := map[string]any{"table": candidate.Table, "status": candidate.Status, "occurrences": candidate.Occurrences}
			if candidate.Path != "" {
				info["path"] = candidate.Path
			}
			if len(candidate.DuplicatePaths) > 0 {
				info["duplicate_paths_preview"] = candidate.DuplicatePaths
			}
			candidates = append(candidates, info)
		}
		entry["numeric_candidates"] = candidates
	}
	return entry
}
