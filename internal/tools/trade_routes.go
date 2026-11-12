package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTradeRoutes(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_trade_routes", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read saved trade route records or their manager entries in saved order. Shows manager-to-route key matches and numeric candidates for route fields with unverified namespaces."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithString("kind", mcp.Description("routes (default) or manager")),
		mcp.WithNumber("offset", mcp.Description("First saved entry, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from first page; required for later pages"))), handleTradeRoutes)
}

func handleTradeRoutes(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := requireStringArgs(req, "kind", "save_id", "page_context"); err != nil {
		return toolError(err), nil
	}
	kind := req.GetString("kind", "routes")
	if kind != "routes" && kind != "manager" {
		return toolError(fmt.Errorf("kind must be routes or manager")), nil
	}
	offset, err := rawInteger(req, "offset", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, -1, "get_trade_routes", kind)
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	if !verified {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	data, err := gamestate.LoadTradeRoutesFromArchive(archive.zip)
	if err != nil {
		return toolError(err), nil
	}
	entries := data.Routes
	if kind == "manager" {
		entries = data.Manager
	}
	if offset > len(entries) {
		return toolError(fmt.Errorf("offset exceeds saved entry count")), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved trade routes (save-wide) ===\nsave_id=%s\npage_context=%s\nkind=%s saved_entries=%d offset=%d\n", archive.saveID, token, kind, len(entries), offset)
	fmt.Fprintf(&b, "route_roots=%d manager_roots=%d manager_list_fields=%d route_lookup_incomplete=%t manager_lookup_incomplete=%t\n", data.RouteRoots, data.ManagerRoots, data.ManagerLists, data.RouteIncomplete, data.ManagerIncomplete)
	fmt.Fprintln(&b, "Counts include repeated saved roots and fields. Manager matches describe recorded numeric keys. Owner, endpoints and path IDs have unverified namespaces, and saved records do not establish current activity. Use raw tools for deep or mixed values.")
	end := offset + min(20, len(entries)-offset)
	for i := offset; i < end; i++ {
		entry := entries[i]
		label := fmt.Sprintf("entry[%d] %s", i, entry.Path)
		if entry.Unsupported {
			label += " (unsupported for lookup; use raw tools)"
		}
		writeSavedValue(&b, label, entry.Value, 0)
		if entry.ID != nil {
			writeTradeRouteLinks(&b, data, *entry.ID)
		}
		if kind == "routes" && entry.ID != nil && entry.Value.IsObject() {
			writeTradeRouteDetails(&b, entry, data.CandidateTables)
		}
	}
	if end < len(entries) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}

func writeTradeRouteLinks(b *strings.Builder, data gamestate.SavedTradeRoutes, id int) {
	key := data.RouteKeys[id]
	var routeStatus string
	switch {
	case key.Occurrences > 1:
		routeStatus = "duplicate"
	case data.RouteIncomplete:
		routeStatus = "unsupported"
	case key.Occurrences == 0:
		routeStatus = "missing"
	case key.Objects == 0:
		routeStatus = "unsupported"
	default:
		routeStatus = "unique"
	}
	count := data.ManagerIDs[id]
	var managerStatus string
	switch {
	case count > 1:
		managerStatus = "duplicate"
	case data.ManagerIncomplete:
		managerStatus = "unsupported"
	case data.ManagerLists == 0:
		managerStatus = "not-recorded"
	case count == 0:
		managerStatus = "outside-list"
	default:
		managerStatus = "unique"
	}
	fmt.Fprintf(b, "  route_status=%s route_key_occurrences=%d manager_status=%s manager_id_occurrences=%d\n",
		routeStatus, key.Occurrences, managerStatus, count)
}

func writeTradeRouteDetails(b *strings.Builder, entry gamestate.SavedTradeRouteEntry, tables []gamestate.TradeRouteCandidateTable) {
	paths := 0
	for i, pair := range entry.Value.Object.Pairs {
		if pair.Key != "path" {
			continue
		}
		paths++
		if pair.Value.IsList() {
			writeSavedValue(b, fmt.Sprintf("  path field[%d] in saved order", i), pair.Value, 0)
		} else if !pair.Value.IsObject() || len(pair.Value.Object.Pairs) > 0 {
			fmt.Fprintf(b, "  path field[%d] has unsupported shape; use raw tools\n", i)
		}
	}
	switch {
	case paths == 0:
		fmt.Fprintln(b, "  path not recorded")
	case paths > 1:
		fmt.Fprintln(b, "  Multiple saved path fields; each is shown separately.")
	}
	for _, ref := range entry.References() {
		switch {
		case ref.ID.Unsupported:
			fmt.Fprintf(b, "  %s unsupported or duplicate\n", ref.Path)
		case ref.ID.Value == nil:
			fmt.Fprintf(b, "  %s not recorded\n", ref.Path)
		default:
			id := *ref.ID.Value
			fmt.Fprintf(b, "  %s=%d numeric key lookup", ref.Path, id)
			for _, candidate := range tables {
				table := candidate.Table
				_, exists := table.Items[id]
				status := "no-candidate"
				switch {
				case exists:
					status = "candidate"
				case !table.Recorded:
					status = "table-not-recorded"
				case strings.HasPrefix(table.DecodeErrors[id], "duplicate "):
					status = "duplicate"
				case table.TableIssue || table.DecodeErrors[id] != "":
					status = "unsupported"
				}
				fmt.Fprintf(b, " %s=%s", candidate.Label, status)
			}
			fmt.Fprintln(b)
		}
	}
}
