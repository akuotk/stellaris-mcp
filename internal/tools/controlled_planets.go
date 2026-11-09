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

const controlledPlanetPageSize = 20

func registerControlledPlanets(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_controlled_planets", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through saved controlled_planets references or planet records with a matching controller candidate outside the observed list."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID. If omitted, all saved player records must refer to one country.")),
		mcp.WithString("view", mcp.Description("listed (default) or unlisted")),
		mcp.WithNumber("offset", mcp.Description("First saved entry, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from this view's first page; required for later pages"))), handleControlledPlanets)
}

func handleControlledPlanets(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	for _, key := range []string{"save_id", "page_context", "view"} {
		if value, ok := req.GetArguments()[key]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	view := req.GetString("view", "listed")
	if view != "listed" && view != "unlisted" {
		return toolError(fmt.Errorf("view must be listed or unlisted")), nil
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
	data, err := gamestate.LoadControlledPlanetsFromArchive(archive.zip, countryID)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, -1, "get_controlled_planets", strconv.Itoa(data.CountryID), view)
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	entries := data.Listed
	if view == "unlisted" {
		entries = data.Unlisted
	}
	if offset > len(entries) {
		return toolError(fmt.Errorf("offset exceeds saved entry count")), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved country planet control references ===\nsave_id=%s\ncountry_id=%d\npage_context=%s\nview=%s saved_entries=%d offset=%d\n",
		archive.saveID, data.CountryID, token, view, len(entries), offset)
	fmt.Fprintf(&b, "controlled_planets_status=%s list_occurrences=%d unsupported_list_entries=%d\n",
		controlledPlanetListStatus(data), data.ListOccurrences, data.UnsupportedListEntries)
	fmt.Fprintf(&b, "planet_table_status=%s planet_table_path=%q planet_unindexed_keys=%d colony_table_status=%s colony_table_path=%q colony_unindexed_keys=%d\n",
		data.PlanetTable.Status, data.PlanetTable.Path, data.PlanetTable.UnindexedKeys, data.ColonyTable.Status, data.ColonyTable.Path, data.ColonyTable.UnindexedKeys)
	reverseCoverage := "unavailable"
	if data.PlanetTable.Status == "recorded" || data.PlanetTable.Status == "partial" {
		reverseCoverage = "complete"
		if data.PlanetTable.Status == "partial" || data.UnresolvedPlanetRecords > 0 {
			reverseCoverage = "partial"
		}
	}
	fmt.Fprintf(&b, "reverse_coverage=%s controller_match_records=%d unresolved_planet_records=%d unlisted_candidate_records=%d\n",
		reverseCoverage, data.ControllerMatchRecords, data.UnresolvedPlanetRecords, len(data.Unlisted))
	fmt.Fprintln(&b, "Matches compare saved numeric fields on the stated paths. List membership, owner, controller and colony carrier checks are separate. Military occupation, ownership and colony status remain unverified. Paths select saved values in read_save_tree.")
	end := offset + min(controlledPlanetPageSize, len(entries)-offset)
	for i := offset; i < end; i++ {
		entry := entries[i]
		label := fmt.Sprintf("entry[%d] path=%q kind=%s", i, entry.Path, entry.Kind)
		if entry.Kind != "controller-record" && entry.ID == nil {
			if len(entry.Value.Scalar) > 512 || !galaxyRegionPreviewFits(entry.Value, 0) {
				fmt.Fprintf(&b, "%s: large saved value; use raw tree/byte tools\n", label)
			} else {
				writeSavedValue(&b, label, entry.Value, 0)
			}
			continue
		}
		fmt.Fprint(&b, label)
		if entry.ID != nil {
			fmt.Fprintf(&b, " planet_id=%d", *entry.ID)
		}
		if entry.Kind == "planet-reference" {
			fmt.Fprintf(&b, " saved_reference=%q list_membership=listed list_occurrences=%d", entry.Value.Scalar, entry.ListOccurrences)
		} else {
			fmt.Fprintf(&b, " list_membership=%s", entry.ListMembership)
		}
		fmt.Fprintf(&b, " planet_status=%s planet_candidates=%d", entry.PlanetStatus, entry.PlanetCandidates)
		if entry.PlanetPath != "" {
			fmt.Fprintf(&b, " planet_path=%q", entry.PlanetPath)
		}
		if len(entry.PlanetCandidatePaths) > 0 {
			fmt.Fprintf(&b, " planet_candidate_paths=%q", entry.PlanetCandidatePaths)
		}
		fmt.Fprintln(&b)
		if entry.Controller.Status == "" {
			continue
		}
		writeControlledPlanetField(&b, "controller", entry.Controller, &data.CountryID)
		writeControlledPlanetField(&b, "owner", entry.Owner, &data.CountryID)
		writeControlledPlanetField(&b, "colony", entry.Colony, nil)
		if entry.ColonyStatus == "" {
			continue
		}
		fmt.Fprintf(&b, "  colony_link=%s", entry.ColonyStatus)
		if entry.ColonyPath != "" {
			fmt.Fprintf(&b, " colony_path=%q", entry.ColonyPath)
		}
		if len(entry.ColonyCandidatePaths) > 0 {
			fmt.Fprintf(&b, " colony_candidate_paths=%q", entry.ColonyCandidatePaths)
		}
		if entry.CarrierType.Recorded && !entry.CarrierType.Unsupported && len(entry.CarrierType.Value) <= 512 {
			fmt.Fprintf(&b, " carrier_type=%q", entry.CarrierType.Value)
		}
		fmt.Fprintln(&b)
		if entry.CarrierReference.Status != "" {
			writeControlledPlanetField(&b, "carrier_reference", entry.CarrierReference, nil)
		}
	}
	if end < len(entries) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}

func controlledPlanetListStatus(data gamestate.SavedControlledPlanets) string {
	switch {
	case data.ListOccurrences == 0:
		return "not-recorded"
	case data.ListOccurrences > 1:
		return "duplicate"
	case data.UnsupportedListEntries > 0:
		return "partial"
	case len(data.Listed) == 0 || (len(data.Listed) == 1 && data.Listed[0].Kind == "empty-list"):
		return "empty"
	default:
		return "recorded"
	}
}

func writeControlledPlanetField(b *strings.Builder, name string, field gamestate.ControlledPlanetField, countryID *int) {
	fmt.Fprintf(b, "  %s_status=%s", name, field.Status)
	if field.Status != "not-recorded" {
		fmt.Fprintf(b, " %s_path=%q", name, field.Path)
	}
	if field.ID != nil {
		fmt.Fprintf(b, " %s_saved=%q", name, field.Value.Scalar)
		if countryID != nil {
			relation := "different-number"
			if *field.ID == *countryID {
				relation = "same-number"
			}
			fmt.Fprintf(b, " %s_country_match=%s", name, relation)
		}
	} else if field.Status == "unsupported" && field.Value.IsScalar() && len(field.Value.Scalar) <= 512 {
		fmt.Fprintf(b, " %s_saved=%q", name, field.Value.Scalar)
	}
	fmt.Fprintln(b)
}
