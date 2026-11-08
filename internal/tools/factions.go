package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerFactions(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_factions", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read saved political factions and country faction-module fields. faction_id selects saved member references, reciprocal legacy pop or current pop-group links, and last-month colony population entries. Amounts, political effects and positional array meanings remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")),
		mcp.WithNumber("faction_id", mcp.Description("Matching faction ID for saved member and colony details"))), handleFactions)
}

func handleFactions(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["faction_id"]; ok {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			maxExact := int64(1<<53 - 1)
			maxID = int(maxExact)
		}
		value, err := rawInteger(req, "faction_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &value
	}
	return renderFactions(gs, id, country, selected), nil
}

func renderFactions(gs *gamestate.GameState, countryID int, country *gamestate.Country, selected *int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved factions (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope follows the saved faction country and the selected country's faction references. Amounts and positions retain saved form.")
	const path = "country.modules.standard_pop_factions_module"
	modules := country.FactionModules
	var refs gamestate.SurfaceRefList
	switch {
	case modules.Duplicate:
		fmt.Fprintf(&b, "%s: duplicate saved fields. Use raw tools.\n", path)
	case modules.Unsupported:
		fmt.Fprintf(&b, "%s: unsupported saved shape. Use raw tools.\n", path)
	case modules.Module == nil:
		fmt.Fprintf(&b, "%s: not recorded\n", path)
	default:
		refs = modules.Module.FactionRefs
		fmt.Fprintf(&b, "%s.pop_factions (saved order): %s\n", path, surfaceRefText(refs))
		if selected == nil {
			writeFactionSavedFields(&b, modules.Module.SavedFields)
		}
	}
	if gs.PopFactions.TableIssue || len(gs.PopFactions.DecodeErrors) != 0 {
		fmt.Fprintln(&b, "Faction table decoding incomplete. Use raw tools for omitted records.")
	} else if !gs.PopFactions.Recorded {
		fmt.Fprintln(&b, "Faction table not recorded.")
	}
	scoped := map[int]bool{}
	for _, id := range refs.Values {
		scoped[id] = true
	}
	for id, faction := range gs.PopFactions.Items {
		if faction.Country != nil && *faction.Country == countryID {
			scoped[id] = true
		}
	}
	ids := make([]int, 0, len(scoped))
	if selected == nil {
		for id := range scoped {
			ids = append(ids, id)
		}
		sort.Ints(ids)
	} else {
		if !scoped[*selected] {
			return toolError(fmt.Errorf("faction %d does not match selected country references", *selected))
		}
		ids = append(ids, *selected)
	}
	if len(ids) == 0 {
		fmt.Fprintln(&b, "No matching decoded faction records or country references.")
	}
	for _, id := range ids {
		faction, ok := gs.PopFactions.Items[id]
		membership := armyMembership(refs.Values, id)
		switch {
		case modules.Duplicate:
			membership = "duplicate saved module fields"
		case modules.Unsupported:
			membership = "unsupported saved module shape"
		case refs.Duplicate || refs.Unsupported:
			membership = surfaceRefText(refs)
		}
		if !ok {
			fmt.Fprintf(&b, "\nFaction %d: %s. Selected module membership=%s\n", id, diplomaticRecordIssue(gs.PopFactions.DecodeErrors[id]), membership)
			continue
		}
		fmt.Fprintf(&b, "\nFaction %d: %s type=%s country=%s. Selected module membership=%s\n", id, name(faction.Name), faction.Type, assetCountryReference(gs, faction.Country), membership)
		if faction.Country != nil && *faction.Country != countryID {
			fmt.Fprintln(&b, "  Saved faction country differs from selected country.")
		}
		fmt.Fprintf(&b, "  support_percent=%s faction_approval=%s support_power=%s last_month_pops=%s pops_change=%s unformable_since=%s (saved values)\n", governanceValue(faction.SupportPercent), governanceValue(faction.Approval), governanceValue(faction.SupportPower), governanceValue(faction.LastMonthPops), governanceValue(faction.PopsChange), governanceValue(faction.UnformableSince))
		if selected != nil {
			writeFactionMembers(&b, gs, id, faction.Members)
			writeFactionColonyHistory(&b, gs, faction.ColonyHistory)
			writeFactionSavedFields(&b, faction.SavedDetails)
		}
	}
	return mcp.NewToolResultText(b.String())
}

func writeFactionSavedFields(b *strings.Builder, fields []clausewitz.Pair) {
	seen := map[string]bool{}
	for _, pair := range fields {
		label := "  " + pair.Key
		if seen[pair.Key] {
			label += " [repeated saved key]"
		}
		seen[pair.Key] = true
		if pair.Value.IsList() {
			values := make([]string, 0, len(pair.Value.List))
			for _, item := range pair.Value.List {
				if !item.IsScalar() {
					break
				}
				values = append(values, item.Scalar)
			}
			if len(values) == len(pair.Value.List) {
				fmt.Fprintf(b, "%s (saved order): %q\n", label, values)
				continue
			}
			if pair.Key == "parameters" {
				for i, item := range pair.Value.List {
					writeSavedValue(b, fmt.Sprintf("%s[%d]", label, i), item, 0)
				}
				continue
			}
		}
		writeSavedValue(b, label, pair.Value, 0)
	}
}

func writeFactionMembers(b *strings.Builder, gs *gamestate.GameState, factionID int, members gamestate.SurfaceRefList) {
	fmt.Fprintf(b, "  members (saved order): %s\n", surfaceRefText(members))
	if len(members.Values) != 0 && gs.LegacyFactionPops.TableIssue {
		fmt.Fprintln(b, "  Legacy pop table decoding incomplete. Member namespace coverage may be incomplete.")
	}
	legacyMatches := 0
	seen := map[int]bool{}
	for _, id := range members.Values {
		if seen[id] {
			fmt.Fprintf(b, "    Member %d: duplicate saved member reference\n", id)
			continue
		}
		seen[id] = true
		group, groupOK := gs.PopGroups[id]
		legacy, legacyOK := gs.LegacyFactionPops.Items[id]
		legacyIssue := gs.LegacyFactionPops.DecodeErrors[id]
		switch {
		case groupOK && (legacyOK || legacyIssue != ""):
			fmt.Fprintf(b, "    Member %d: ID appears in both population namespaces. Membership unresolved.\n", id)
		case groupOK:
			writeFactionGroupMembership(b, id, factionID, group.Factions)
		case legacyOK:
			ref := legacy.FactionRef
			switch {
			case legacy.Duplicate:
				fmt.Fprintf(b, "    Legacy pop %d: duplicate pop_faction fields\n", id)
			case ref.Unsupported:
				fmt.Fprintf(b, "    Legacy pop %d: unsupported pop_faction shape\n", id)
			case ref.Value == nil:
				fmt.Fprintf(b, "    Legacy pop %d: pop_faction not recorded\n", id)
			case *ref.Value != factionID:
				fmt.Fprintf(b, "    Legacy pop %d: saved pop_faction=%d differs from faction ID\n", id, *ref.Value)
			default:
				legacyMatches++
			}
		case legacyIssue != "":
			fmt.Fprintf(b, "    Legacy pop %d: %s\n", id, diplomaticRecordIssue(legacyIssue))
		default:
			fmt.Fprintf(b, "    Member %d: population record unavailable\n", id)
		}
	}
	if legacyMatches != 0 {
		fmt.Fprintf(b, "  Legacy pop reciprocal links: %d\n", legacyMatches)
	}
}

func writeFactionGroupMembership(b *strings.Builder, groupID, factionID int, distribution gamestate.PopFactionDistribution) {
	label := fmt.Sprintf("    Pop group %d", groupID)
	switch {
	case distribution.Duplicate:
		fmt.Fprintf(b, "%s: duplicate faction fields or group records\n", label)
	case distribution.Unsupported || len(distribution.Objects) != len(distribution.Pops):
		fmt.Fprintf(b, "%s: unsupported or mismatched faction arrays\n", label)
	default:
		var amounts []int
		for i, id := range distribution.Objects {
			if id == factionID {
				amounts = append(amounts, distribution.Pops[i])
			}
		}
		switch len(amounts) {
		case 0:
			fmt.Fprintf(b, "%s: saved faction back-reference unavailable\n", label)
		case 1:
			fmt.Fprintf(b, "%s: reciprocal faction ID agrees, raw population=%d\n", label, amounts[0])
		default:
			fmt.Fprintf(b, "%s: duplicate faction back-references, raw amounts=%v\n", label, amounts)
		}
	}
}

func writeFactionColonyHistory(b *strings.Builder, gs *gamestate.GameState, history gamestate.FactionColonyHistory) {
	fmt.Fprintln(b, "  last_month_colony_pops (saved order):")
	switch {
	case history.Duplicate:
		fmt.Fprintln(b, "    Duplicate saved fields. Use raw tools.")
	case history.Unsupported:
		fmt.Fprintln(b, "    Unsupported saved shape. Use raw tools.")
	case !history.Recorded:
		fmt.Fprintln(b, "    Not recorded.")
	case len(history.Items) == 0:
		fmt.Fprintln(b, "    Empty.")
	}
	seen := map[int]bool{}
	for i, row := range history.Items {
		switch {
		case row.Duplicate:
			fmt.Fprintf(b, "    Row %d: duplicate saved fields\n", i)
		case row.Unsupported:
			fmt.Fprintf(b, "    Row %d: unsupported saved row shape\n", i)
		default:
			fmt.Fprintf(b, "    Row %d colony=%s num_pops=%s change=%s (saved values)\n", i, armyInt(row.Colony.Value), governanceValue(row.NumPops), governanceValue(row.Change))
			if row.Colony.Unsupported {
				fmt.Fprintln(b, "      Unsupported colony reference shape.")
			}
			if row.Colony.Value != nil {
				id := *row.Colony.Value
				if seen[id] {
					fmt.Fprintln(b, "      Duplicate saved colony reference.")
				}
				seen[id] = true
				if colony, ok := gs.Colonies[id]; ok {
					fmt.Fprintf(b, "      Colony %d: %s\n", id, colonyCarrier(gs, id, colony))
				} else {
					fmt.Fprintln(b, "      Colony record unavailable.")
				}
			}
		}
	}
}
