package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const countryObservationPageSize = 20

func registerCountryObservationRefs(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_country_observation_refs", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through one country's saved sensor_range_fleets or surveyed_deposit_holders and compare numeric keys across saved tables."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID. If omitted, all saved player records must refer to one country.")),
		mcp.WithString("kind", mcp.Description("sensor_range_fleets (default) or surveyed_deposit_holders")),
		mcp.WithNumber("offset", mcp.Description("First saved entry, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from this kind's first page; required for later pages"))), handleCountryObservationRefs)
}

func handleCountryObservationRefs(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := requireStringArgs(req, "save_id", "page_context", "kind"); err != nil {
		return toolError(err), nil
	}
	kind := req.GetString("kind", gamestate.SensorRangeFleetsKind)
	if kind != gamestate.SensorRangeFleetsKind && kind != gamestate.SurveyedDepositHoldersKind {
		return toolError(fmt.Errorf("kind must be sensor_range_fleets or surveyed_deposit_holders")), nil
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
	data, err := gamestate.LoadCountryObservationRefsFromArchive(archive.zip, countryID, kind)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, -1, "get_country_observation_refs", strconv.Itoa(data.CountryID), kind)
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	if offset > len(data.Entries) {
		return toolError(fmt.Errorf("offset exceeds saved entry count")), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved country observation references ===\nsave_id=%s\ncountry_id=%d\npage_context=%s\nkind=%s saved_entries=%d offset=%d\n",
		archive.saveID, data.CountryID, token, kind, len(data.Entries), offset)
	fmt.Fprintf(&b, "field_status=%s field_occurrences=%d unsupported_entries=%d\n", countryObservationFieldStatus(data), data.FieldOccurrences, data.UnsupportedEntries)
	fmt.Fprintln(&b, "Table matches are numeric candidates. Sensor visibility, survey state, ownership and holder type remain unverified. Paths select saved values in read_save_tree.")
	for _, table := range data.Tables {
		fmt.Fprintf(&b, "table=%s table_status=%s table_path=%q unindexed_keys=%d", table.Label, table.Status, table.Path, table.UnindexedKeys)
		if len(table.DuplicatePaths) > 0 {
			fmt.Fprintf(&b, " duplicate_paths=%q", table.DuplicatePaths)
		}
		fmt.Fprintln(&b)
	}
	end := offset + min(countryObservationPageSize, len(data.Entries)-offset)
	for i := offset; i < end; i++ {
		entry := data.Entries[i]
		label := fmt.Sprintf("entry[%d] path=%q kind=%s", i, entry.Path, entry.Kind)
		if entry.Kind != "sensor-reference" && entry.Kind != "holder-reference" {
			if len(entry.Value.Scalar) > 512 || !galaxyRegionPreviewFits(entry.Value, 0) {
				fmt.Fprintf(&b, "%s: large saved value; use raw tree/byte tools\n", label)
			} else {
				writeSavedValue(&b, label, entry.Value, 0)
			}
			continue
		}
		fmt.Fprintf(&b, "%s id_occurrences=%d", label, entry.IDOccurrences)
		if entry.PairOccurrences > 0 {
			fmt.Fprintf(&b, " id_type_pair_occurrences=%d", entry.PairOccurrences)
		}
		fmt.Fprintln(&b)
		writeControlledPlanetField(&b, "id", entry.Identifier, nil)
		if entry.Kind == "holder-reference" {
			writeControlledPlanetField(&b, "type", entry.HolderType, nil)
		}
		if entry.Identifier.ID == nil {
			continue
		}
		comparison := data.Comparisons[*entry.Identifier.ID]
		for _, candidate := range comparison.Candidates {
			fmt.Fprintf(&b, "  candidate_table=%s status=%s occurrences=%d", candidate.Table, candidate.Status, candidate.Occurrences)
			if candidate.Path != "" {
				fmt.Fprintf(&b, " path=%q", candidate.Path)
			}
			if len(candidate.DuplicatePaths) > 0 {
				fmt.Fprintf(&b, " duplicate_paths=%q", candidate.DuplicatePaths)
			}
			fmt.Fprintln(&b)
		}
		if comparison.PlanetSurveyedBy.Status != "" {
			writeControlledPlanetField(&b, "planet_candidate_surveyed_by", comparison.PlanetSurveyedBy, &data.CountryID)
		}
	}
	if end < len(data.Entries) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}

func countryObservationFieldStatus(data gamestate.SavedCountryObservationRefs) string {
	switch {
	case data.FieldOccurrences == 0:
		return "not-recorded"
	case data.FieldOccurrences > 1:
		return "duplicate"
	case data.UnsupportedEntries > 0:
		return "partial"
	case len(data.Entries) == 0 || (len(data.Entries) == 1 && data.Entries[0].Kind == "empty-list"):
		return "empty"
	default:
		return "recorded"
	}
}
