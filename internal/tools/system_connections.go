package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSystemConnections(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_system_connections", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read a saved system's hyperlane entries, bypass records and natural wormhole references. Checks reciprocal IDs and exposes missing or differing saved references. Visibility, travel eligibility, units and route cost remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("system_id", mcp.Required(), mcp.Description("Saved galactic_object ID"))), handleSystemConnections)
}

func handleSystemConnections(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := req.GetArguments()["system_id"]; !ok {
		return toolError(fmt.Errorf("system_id is required")), nil
	}
	id, err := rawInteger(req, "system_id", 0, 0, maxRecordID)
	if err != nil {
		return toolError(err), nil
	}
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	return renderSystemConnections(gs, id), nil
}

func renderSystemConnections(gs *gamestate.GameState, id int) *mcp.CallToolResult {
	system, ok := gs.Systems[id]
	if !ok {
		return toolError(fmt.Errorf("system %d has no decoded record", id))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved connections for system %d (%s) ===\n", id, name(system.Name))
	fmt.Fprintln(&b, "Visibility, travel eligibility, field units and route costs remain unverified. Matching IDs are evidence about saved references.")
	fmt.Fprintln(&b, "\nhyperlane (saved order)")
	switch {
	case system.Hyperlanes.Unsupported:
		fmt.Fprintln(&b, "  Unsupported or duplicate saved list. Use raw tree/byte tools.")
	case !system.Hyperlanes.Recorded:
		fmt.Fprintln(&b, "  Not recorded")
	case len(system.Hyperlanes.Items) == 0:
		fmt.Fprintln(&b, "  Empty")
	}
	seen := map[int]bool{}
	for index, edge := range system.Hyperlanes.Items {
		if edge.Unsupported {
			fmt.Fprintf(&b, "  Entry %d unsupported or duplicate fields. Use raw tools.\n", index)
			continue
		}
		fmt.Fprintf(&b, "  Entry %d to=%s length=%s bridge=%s\n", index, savedInt(edge.To), governanceValue(edge.Length), governanceValue(edge.Bridge))
		if edge.To == nil {
			continue
		}
		if seen[*edge.To] {
			fmt.Fprintln(&b, "    Duplicate saved target ID")
		}
		seen[*edge.To] = true
		writeHyperlaneReverse(&b, gs, id, edge)
	}
	fmt.Fprintln(&b, "\nbypasses (saved order)")
	writeConnectionReferenceList(&b, "system.bypasses", system.BypassRefs)
	if !gs.Bypasses.Recorded && (len(system.BypassRefs.Values) > 0 || len(system.WormholeRefs.Values) > 0) {
		fmt.Fprintln(&b, "  Bypass table not recorded")
	}
	if gs.Bypasses.TableIssue || len(gs.Bypasses.DecodeErrors) > 0 {
		fmt.Fprintln(&b, "  Bypass table decoding incomplete. Use raw tools for unsupported records.")
	}
	for _, bypassID := range system.BypassRefs.Values {
		writeSavedBypass(&b, gs, bypassID)
	}
	fmt.Fprintln(&b, "\nnatural_wormholes (saved order)")
	writeConnectionReferenceList(&b, "system.natural_wormholes", system.WormholeRefs)
	if !gs.NaturalWormholes.Recorded && len(system.WormholeRefs.Values) > 0 {
		fmt.Fprintln(&b, "  Natural wormhole table not recorded")
	}
	if gs.NaturalWormholes.TableIssue || len(gs.NaturalWormholes.DecodeErrors) > 0 {
		fmt.Fprintln(&b, "  Natural wormhole table decoding incomplete. Use raw tools for unsupported records.")
	}
	for _, wormholeID := range system.WormholeRefs.Values {
		writeSavedWormhole(&b, gs, id, system.BypassRefs, wormholeID)
	}
	return mcp.NewToolResultText(b.String())
}

func writeHyperlaneReverse(b *strings.Builder, gs *gamestate.GameState, sourceID int, edge gamestate.SavedHyperlane) {
	if *edge.To == sourceID {
		fmt.Fprintln(b, "    Saved target equals the source system ID")
		return
	}
	target, ok := gs.Systems[*edge.To]
	if !ok {
		fmt.Fprintln(b, "    Target system record unavailable")
		return
	}
	fmt.Fprintf(b, "    Target system %d %s\n", *edge.To, name(target.Name))
	if target.Hyperlanes.Unsupported || !target.Hyperlanes.Recorded {
		fmt.Fprintln(b, "    Reverse hyperlane list unavailable or unsupported")
		return
	}
	matches, unsupported := 0, 0
	for index, reverse := range target.Hyperlanes.Items {
		if reverse.Unsupported {
			unsupported++
			continue
		}
		if reverse.To == nil || *reverse.To != sourceID {
			continue
		}
		matches++
		fmt.Fprintf(b, "    Reverse saved entry %d to=%d length=%s bridge=%s\n", index, sourceID, governanceValue(reverse.Length), governanceValue(reverse.Bridge))
		if edge.Length != nil && reverse.Length != nil && *edge.Length != *reverse.Length {
			fmt.Fprintln(b, "      Saved length texts differ")
		}
		if edge.Bridge != nil && reverse.Bridge != nil && *edge.Bridge != *reverse.Bridge {
			fmt.Fprintln(b, "      Saved bridge values differ")
		}
	}
	if matches == 0 {
		fmt.Fprintln(b, "    No decoded reverse hyperlane entry")
	} else if matches > 1 {
		fmt.Fprintf(b, "    Multiple reverse saved entries=%d\n", matches)
	}
	if unsupported > 0 {
		fmt.Fprintf(b, "    Unsupported target hyperlane entries=%d. Reverse coverage incomplete.\n", unsupported)
	}
}

func writeSavedBypass(b *strings.Builder, gs *gamestate.GameState, id int) {
	bypass, ok := gs.Bypasses.Items[id]
	if !ok {
		fmt.Fprintf(b, "  Bypass %d referenced record unavailable or unsupported\n", id)
		return
	}
	fmt.Fprintf(b, "  Bypass %d type=%s active=%s linked_to=%s\n", id, governanceValue(bypass.Type), governanceValue(bypass.Active), savedInt(bypass.LinkedTo))
	fmt.Fprintf(b, "    is_manual_lock=%s lock_country=%s lock_remaining_days=%s (saved values)\n", governanceValue(bypass.ManualLock), savedInt(bypass.LockCountry), governanceValue(bypass.LockRemainingDays))
	if bypass.Owner == nil {
		fmt.Fprintln(b, "    owner not recorded")
	} else {
		fmt.Fprintf(b, "    owner.type=%s owner.id=%s (namespace unverified)\n", governanceValue(bypass.Owner.Type), savedInt(bypass.Owner.ID))
	}
	if bypass.LinkedTo != nil {
		if *bypass.LinkedTo == id {
			fmt.Fprintln(b, "    Saved linked_to equals the source bypass ID")
		} else if other, ok := gs.Bypasses.Items[*bypass.LinkedTo]; ok {
			fmt.Fprintf(b, "    linked_to bypass ID candidate %d with linked_to=%s\n", *bypass.LinkedTo, savedInt(other.LinkedTo))
			if other.LinkedTo != nil && *other.LinkedTo == id {
				fmt.Fprintln(b, "      Reciprocal saved linked_to IDs agree")
			} else if other.LinkedTo != nil {
				fmt.Fprintln(b, "      Saved linked_to IDs disagree")
			}
		} else {
			fmt.Fprintln(b, "    linked_to bypass ID candidate unavailable")
		}
	}
	writeBypassConnections(b, gs, id, "connections", bypass.Connections)
	writeBypassConnections(b, gs, id, "active_connections", bypass.ActiveConnections)
}

func writeBypassConnections(b *strings.Builder, gs *gamestate.GameState, sourceID int, key string, refs gamestate.OptionalIntList) {
	writeConnectionReferenceList(b, key, refs)
	for index, id := range refs.Values {
		if id == sourceID {
			fmt.Fprintf(b, "    %s[%d] equals the source bypass ID %d\n", key, index, id)
			continue
		}
		other, ok := gs.Bypasses.Items[id]
		if !ok {
			fmt.Fprintf(b, "    %s[%d] bypass ID candidate %d unavailable\n", key, index, id)
			continue
		}
		reverse := other.Connections
		if key == "active_connections" {
			reverse = other.ActiveConnections
		}
		membership := "unsupported (use raw tools)"
		if !reverse.Unsupported {
			membership = armyMembership(reverse.Values, sourceID)
		}
		fmt.Fprintf(b, "    %s[%d] bypass ID candidate %d reverse %s membership=%s\n", key, index, id, key, membership)
	}
}

func writeSavedWormhole(b *strings.Builder, gs *gamestate.GameState, systemID int, bypassRefs gamestate.OptionalIntList, id int) {
	wormhole, ok := gs.NaturalWormholes.Items[id]
	if !ok {
		fmt.Fprintf(b, "  Natural wormhole %d referenced record unavailable or unsupported\n", id)
		return
	}
	fmt.Fprintf(b, "  Natural wormhole %d bypass=%s\n", id, savedInt(wormhole.Bypass))
	switch {
	case wormhole.Coordinate.Unsupported:
		fmt.Fprintln(b, "    coordinate unsupported or duplicate. Use raw tools.")
	case wormhole.Coordinate.Origin == nil:
		fmt.Fprintln(b, "    coordinate.origin not recorded")
	case *wormhole.Coordinate.Origin == systemID:
		fmt.Fprintf(b, "    coordinate.origin=%d agrees with the referencing system ID\n", *wormhole.Coordinate.Origin)
	default:
		fmt.Fprintf(b, "    coordinate.origin=%d differs from the referencing system ID\n", *wormhole.Coordinate.Origin)
	}
	if wormhole.Bypass == nil {
		return
	}
	membership := "unsupported (use raw tools)"
	if !bypassRefs.Unsupported {
		membership = armyMembership(bypassRefs.Values, *wormhole.Bypass)
	}
	fmt.Fprintf(b, "    wormhole.bypass in system.bypasses membership=%s\n", membership)
	if candidate, ok := gs.Bypasses.Items[*wormhole.Bypass]; ok {
		if candidate.Owner != nil && candidate.Owner.ID != nil {
			fmt.Fprintf(b, "    Bypass candidate owner.id=%d. Owner namespace unverified.\n", *candidate.Owner.ID)
			if *candidate.Owner.ID != id {
				fmt.Fprintln(b, "    Bypass candidate owner.id differs from the wormhole ID")
			}
		} else {
			fmt.Fprintln(b, "    Bypass candidate owner.id not recorded")
		}
	} else {
		fmt.Fprintln(b, "    Referenced bypass ID candidate unavailable")
	}
}

func writeConnectionReferenceList(b *strings.Builder, key string, refs gamestate.OptionalIntList) {
	if refs.Unsupported {
		fmt.Fprintf(b, "  %s unsupported or duplicate. Use raw tools.\n", key)
		return
	}
	fmt.Fprintf(b, "  %s (saved order)=%s\n", key, colonyReferenceList(refs.Values))
}
