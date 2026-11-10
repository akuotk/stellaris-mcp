package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerColonyPopulationRecords(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_colony_population_records",
		mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through saved population and growth records for one entry in the selected country's owned_planets. Includes raw positions, byte ranges and numeric back-reference checks. Saved growth, population and workforce values have unverified units and meanings."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games. Omit for latest.")),
		mcp.WithNumber("country_id", mcp.Description("Country ID. If omitted, all saved player records must refer to the same unique country.")),
		mcp.WithNumber("location_id", mcp.Required(), mcp.Description("ID as saved in country.owned_planets. Uses the colony table when recorded, or the legacy planet table when absent.")),
		mcp.WithString("view", mcp.Enum("growth", "pop_groups", "pop_jobs", "group_fields", "job_fields"), mcp.DefaultString("growth")),
		mcp.WithNumber("record_id", mcp.Description("Saved group or job ID. Required for group_fields and job_fields.")),
		mcp.WithNumber("offset", mcp.Description("Zero-based row offset. Up to 20 rows per page.")),
		mcp.WithString("page_context", mcp.Description("Token required for later pages. Continue with the same save, country, location, view and record."))),
		handleColonyPopulationRecords)
}

func handleColonyPopulationRecords(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	for _, key := range []string{"save_id", "view", "page_context"} {
		if value, ok := args[key]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	view := req.GetString("view", "growth")
	detail := view == "group_fields" || view == "job_fields"
	if view != "growth" && view != "pop_groups" && view != "pop_jobs" && !detail {
		return toolError(fmt.Errorf("view must be growth, pop_groups, pop_jobs, group_fields or job_fields")), nil
	}
	if _, ok := args["location_id"]; !ok {
		return toolError(fmt.Errorf("location_id is required")), nil
	}
	locationID, err := rawInteger(req, "location_id", 0, 0, int(maxSaveRecordID))
	if err != nil {
		return toolError(err), nil
	}
	var countryID, recordID *int
	if _, ok := args["country_id"]; ok {
		id, err := rawInteger(req, "country_id", 0, 0, int(maxSaveRecordID))
		if err != nil {
			return toolError(err), nil
		}
		countryID = &id
	}
	recordKey := ""
	if _, present := args["record_id"]; present != detail {
		return toolError(fmt.Errorf("record_id is required for group_fields and job_fields and is only valid in those views")), nil
	}
	if detail {
		id, err := rawInteger(req, "record_id", 0, 0, int(maxSaveRecordID))
		if err != nil {
			return toolError(err), nil
		}
		recordID, recordKey = &id, strconv.Itoa(id)
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
	saved, err := gamestate.LoadColonyPopulationRecordsFromArchive(ctx, archive.zip, countryID, locationID, view, recordID, offset)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, saved.EntryIndex, "get_colony_population_records", strconv.Itoa(saved.CountryID), strconv.Itoa(locationID), view, recordKey)
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	if offset > saved.RowCount {
		return toolError(fmt.Errorf("offset exceeds saved row count")), nil
	}
	rows := make([]map[string]any, 0, len(saved.Rows))
	for _, row := range saved.Rows {
		var key *string
		if row.Node.Key != nil {
			key = &row.Node.Key.Text
		}
		entry := rawNode(row.Path, key, row.Node)
		entry["role"], entry["child_index"], entry["container_path"] = row.Role, row.Index, row.ContainerPath
		if row.Reference != nil {
			entry["reference"], entry["record_planet_numeric_match"] = row.Reference, row.Backref
			entry["list_reference_occurrences"] = row.ReferenceOccurrences
		}
		rows = append(rows, entry)
	}
	var next *int
	if end := offset + len(rows); end < saved.RowCount {
		next = &end
	}
	result := rawTreePage(archive, saved.EntryIndex)
	result["country_id"], result["country_path"], result["location_id"] = saved.CountryID, saved.CountryPath, locationID
	result["schema"], result["view"], result["view_status"] = saved.Schema, view, saved.ViewStatus
	result["ownership_list"], result["owned_reference_occurrences"], result["owned_reference_paths_preview"] = saved.Ownership, saved.OwnedOccurrences, saved.OwnedPaths
	result["location_table"], result["location"] = saved.LocationTable, saved.Location
	result["planet_link"], result["planet"] = saved.PlanetLink, saved.Planet
	if saved.PopulationTable != nil {
		result["population_table"], result["population_list"] = saved.PopulationTable, saved.PopulationList
	}
	if detail {
		result["record_id"], result["record"] = recordID, saved.Record
		result["record_planet_numeric_match"] = saved.RecordBackref
		result["record_list_occurrences"], result["record_reverse_observed"] = saved.RecordListOccurrences, saved.RecordReverseObserved
	}
	result["rows"], result["row_count"] = rows, saved.RowCount
	result["offset"], result["next_offset"], result["page_context"] = offset, next, token
	result["interpretation"] = "Ownership scope follows the saved country list. Population record planet fields are compared numerically with the selected colony ID. Values, reverse-only records and numeric matches leave gameplay units and status unverified."
	return rawJSON(result)
}
