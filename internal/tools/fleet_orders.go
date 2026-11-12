package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerFleetOrders(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_fleet_orders",
		mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through saved country fleet references and fleet current_order fields in source order. Explicit fleet, planet, special_project and archaeological_site fields get numeric candidates. Order identifiers, timing, execution and pending status remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games. Omit for latest.")),
		mcp.WithNumber("country_id", mcp.Description("Country ID. If omitted, all saved player records must refer to the same unique country.")),
		mcp.WithNumber("fleet_id", mcp.Description("Fleet ID observed in the selected country's saved owned_fleets. Required for orders.")),
		mcp.WithString("view", mcp.Enum("fleets", "orders"), mcp.Description("Defaults to fleets without fleet_id and orders with fleet_id.")),
		mcp.WithNumber("offset", mcp.Description("Zero-based row offset. Up to 20 rows per page.")),
		mcp.WithString("page_context", mcp.Description("Token required for later pages. Continue with the same save, country, view and fleet."))),
		handleFleetOrders)
}

func handleFleetOrders(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	if err := requireStringArgs(req, "save_id", "view", "page_context"); err != nil {
		return toolError(err), nil
	}
	var countryID, fleetID *int
	if _, ok := args["country_id"]; ok {
		id, err := rawInteger(req, "country_id", 0, 0, maxRecordID)
		if err != nil {
			return toolError(err), nil
		}
		countryID = &id
	}
	defaultView, fleetKey := "fleets", ""
	if _, ok := args["fleet_id"]; ok {
		id, err := rawInteger(req, "fleet_id", 0, 0, maxRecordID)
		if err != nil {
			return toolError(err), nil
		}
		fleetID, fleetKey, defaultView = &id, strconv.Itoa(id), "orders"
	}
	view := req.GetString("view", defaultView)
	if view != "fleets" && view != "orders" {
		return toolError(fmt.Errorf("view must be fleets or orders")), nil
	}
	if (view == "orders") != (fleetID != nil) {
		return toolError(fmt.Errorf("fleet_id is required for orders and is only valid in that view")), nil
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
	saved, err := gamestate.LoadFleetOrdersFromArchive(ctx, archive.zip, countryID, fleetID, offset)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, saved.EntryIndex, "get_fleet_orders", strconv.Itoa(saved.CountryID), view, fleetKey)
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
		entry["role"], entry["child_index"], entry["container_path"] = row.Role, row.Index, row.ParentPath
		if row.OrderPath != "" {
			entry["current_order_path"] = row.OrderPath
		}
		if row.Fleet != nil {
			entry["fleet"], entry["owned_reference_occurrences"] = row.Fleet, row.OwnedOccurrences
			if row.Fleet.RecordStatus == "object" {
				entry["current_order_occurrences"] = row.OrderOccurrences
			}
		}
		if row.RefTarget != "" {
			entry["reference_target"], entry["reference_status"] = row.RefTarget, row.RefStatus
			if row.Reference != nil {
				entry["reference"] = row.Reference
			}
		}
		rows = append(rows, entry)
	}
	var next *int
	if end := offset + len(rows); end < saved.RowCount {
		next = &end
	}
	result := rawTreePage(archive, saved.EntryIndex)
	result["country_id"], result["country_path"], result["view"] = saved.CountryID, saved.CountryPath, view
	result["view_status"], result["owned_fleets"], result["fleet_table"] = saved.ViewStatus, saved.Scope, saved.FleetTable
	if fleetID != nil {
		result["fleet_id"], result["fleet"] = fleetID, saved.Fleet
		result["owned_reference_occurrences"], result["owned_reference_paths_preview"] = saved.OwnedOccurrences, saved.OwnedPaths
		if saved.Fleet != nil && saved.Fleet.RecordStatus == "object" {
			result["current_order_occurrences"] = saved.OrderOccurrences
		}
	}
	if len(saved.Tables) > 0 {
		result["candidate_tables"] = saved.Tables
	}
	result["rows"], result["row_count"] = rows, saved.RowCount
	result["offset"], result["next_offset"], result["page_context"] = offset, next, token
	result["interpretation"] = "Fleet scope follows saved owned_fleets references. Special project candidates come from the selected country. Saved references and order fields leave execution, pending status, timing and code meanings unverified."
	return rawJSON(result)
}
