package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerArchaeology(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_archaeology", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Read archaeological sites matching visible_to, last_excavator_country or saved completed-country entries. Shows raw location and fleet references, checks reciprocal excavation order IDs and lists remaining orders from the selected country's saved owned_fleets. site_id selects saved event/log details. Progress units, numeric location enums, phase and completion status are not inferred."), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")), mcp.WithNumber("site_id", mcp.Description("Matching archaeological site ID for full details; omit for summary"))), handleArchaeology)
}

func handleArchaeology(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["site_id"]; ok {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			maxExact := int64(1<<53 - 1)
			maxID = int(maxExact)
		}
		siteID, err := rawInteger(req, "site_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &siteID
	}
	return renderArchaeology(gs, id, country, selected), nil
}

func renderArchaeology(gs *gamestate.GameState, countryID int, country *gamestate.Country, selected *int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Archaeological records (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope uses saved visibility, last excavator or completed-country entries. Progress, location enums, completion semantics and event scope meanings are unverified; matching object IDs are candidates only.")
	if gs.ArchaeologySites.TableIssue || len(gs.ArchaeologySites.DecodeErrors) > 0 {
		fmt.Fprintln(&b, "Archaeology decoding incomplete for some records; use raw tree/byte tools for complete data.")
	} else if !gs.ArchaeologySites.SitesRecorded {
		fmt.Fprintln(&b, "archaeological_sites.sites table not recorded")
	}
	reasons := map[int][]string{}
	for id, site := range gs.ArchaeologySites.Items {
		for _, country := range site.VisibleTo {
			if country == countryID {
				reasons[id] = append(reasons[id], "visible_to")
			}
		}
		if site.LastExcavatorCountry != nil && *site.LastExcavatorCountry == countryID {
			reasons[id] = append(reasons[id], "last_excavator_country")
		}
		for _, entry := range site.Completed {
			if entry.Country != nil && *entry.Country == countryID {
				reasons[id] = append(reasons[id], "completed.country")
			}
		}
	}
	ids := make([]int, 0, len(reasons))
	for id := range reasons {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if selected != nil {
		if len(reasons[*selected]) == 0 {
			if !gs.ArchaeologySites.SitesRecorded && !gs.ArchaeologySites.TableIssue {
				return toolError(fmt.Errorf("site %d unavailable because archaeological_sites.sites table not recorded", *selected))
			}
			return toolError(fmt.Errorf("site %d has no matching country references", *selected))
		}
		ids = []int{*selected}
	}
	if len(ids) == 0 {
		fmt.Fprintln(&b, "No matching decoded archaeological records.")
	}
	for _, id := range ids {
		site := gs.ArchaeologySites.Items[id]
		fmt.Fprintf(&b, "\nSite %d: type=%s index=%s days_left=%s; country references=%v\n", id, operationFlagText(site.Type), armyInt(site.Index), armyFloat(site.DaysLeft), reasons[id])
		if _, bad := gs.ArchaeologySites.DecodeErrors[id]; bad {
			fmt.Fprintln(&b, "  Record decoding incomplete; displayed fields may be partial")
		}
		fmt.Fprintf(&b, "  Location (raw): type=%s id=%s\n", operationFlagText(site.Location.Type), armyInt(site.Location.ID))
		if site.Location.ID != nil {
			if planet, ok := gs.Planets.Planet[*site.Location.ID]; ok {
				fmt.Fprintf(&b, "    Possible planet ID match: %s; no reverse site link validated\n", name(planet.Name))
			} else {
				fmt.Fprintln(&b, "    No planet ID match; numeric location type remains unclassified")
			}
		}
		fmt.Fprintf(&b, "  last_excavator_country=%s excavator_fleet=%s\n", armyInt(site.LastExcavatorCountry), armyInt(site.ExcavatorFleet))
		if site.LastExcavatorCountry != nil {
			if _, ok := gs.Country[*site.LastExcavatorCountry]; !ok {
				fmt.Fprintln(&b, "    Last excavator country reference unresolved (sentinel/deleted-state meaning not inferred)")
			}
		}
		if site.ExcavatorFleetAmbiguous {
			fmt.Fprintln(&b, "    Duplicate excavator_fleet fields. Fleet candidate unresolved. Use raw tree/byte tools.")
		} else if site.ExcavatorFleet != nil {
			if fleet, ok := gs.Fleet[*site.ExcavatorFleet]; ok {
				_, siteDecodeError := gs.ArchaeologySites.DecodeErrors[id]
				writeExcavationOrderCandidate(&b, id, fleet, gs.ArchaeologySites.TableIssue || siteDecodeError)
			} else {
				fmt.Fprintln(&b, "    Excavator fleet reference unresolved (sentinel/deleted-state meaning not inferred)")
			}
		}
		if selected == nil {
			continue
		}
		fmt.Fprintf(&b, "  clues=%s last_roll=%s difficulty=%s\n", armyFloat(site.Clues), armyFloat(site.LastRoll), armyFloat(site.Difficulty))
		if site.Locked == nil {
			fmt.Fprintln(&b, "  locked: not recorded")
		} else {
			fmt.Fprintf(&b, "  locked: %t\n", *site.Locked)
		}
		fmt.Fprintf(&b, "  visible_to (raw references): %s\n", colonyReferenceList(site.VisibleTo))
		if site.Completed == nil {
			fmt.Fprintln(&b, "  completed entries: not recorded")
		} else if len(site.Completed) == 0 {
			fmt.Fprintln(&b, "  completed entries: empty")
		}
		for index, entry := range site.Completed {
			fmt.Fprintf(&b, "  completed entry %d: country=%s date=%s (saved record; global site completion not inferred)\n", index, armyInt(entry.Country), operationFlagText(entry.Date))
		}
		if site.Events == nil {
			fmt.Fprintln(&b, "  events: not recorded")
		} else if len(site.Events) == 0 {
			fmt.Fprintln(&b, "  events: empty")
		}
		for index, event := range site.Events {
			fmt.Fprintf(&b, "  event entry %d: event_id=%s expired=%s owner=%s fleet=%s index=%s player_event=%s\n", index, operationFlagText(event.EventID), operationFlagText(event.Expired), armyInt(event.Owner), armyInt(event.Fleet), armyInt(event.Index), operationFlagText(event.PlayerEvent))
			fmt.Fprintf(&b, "    effect=%s picture=%s\n", operationFlagText(event.Effect), operationFlagText(event.Picture))
			if event.Scope == nil {
				fmt.Fprintln(&b, "    scope: not recorded")
			} else {
				fmt.Fprintf(&b, "    scope: preserved opaque object (%d ordered fields); use raw tree tools for scope data/meaning\n", len(event.Scope.Value.Object.Pairs))
			}
		}
		if site.Log == nil {
			fmt.Fprintln(&b, "  log: not recorded")
		} else if len(site.Log) == 0 {
			fmt.Fprintln(&b, "  log: empty")
		}
		for index, entry := range site.Log {
			fmt.Fprintf(&b, "  log entry %d (saved order): date=%s title=%s tooltip=%s clues=%s total=%s roll=%s bonus=%s difficulty=%s\n", index, operationFlagText(entry.Date), operationFlagText(entry.Title), operationFlagText(entry.Tooltip), armyFloat(entry.Clues), armyFloat(entry.Total), armyFloat(entry.Roll), armyFloat(entry.Bonus), armyFloat(entry.Difficulty))
		}
		for _, key := range federationSortedKeys(site.Flags) {
			flag := site.Flags[key]
			if flag.Scalar != nil {
				fmt.Fprintf(&b, "  Flag %s=%s\n", key, *flag.Scalar)
			} else {
				fmt.Fprintf(&b, "  Flag %s: flag_date=%s flag_days=%s (as saved)\n", key, operationFlagText(flag.FlagDate), operationFlagText(flag.FlagDays))
			}
		}
	}
	writeRemainingExcavationOrders(&b, gs, country, reasons, selected)
	return mcp.NewToolResultText(b.String())
}

func writeRemainingExcavationOrders(b *strings.Builder, gs *gamestate.GameState, country *gamestate.Country, reasons map[int][]string, selected *int) {
	manager := country.FleetsManager
	if manager.OwnershipAmbiguous {
		fmt.Fprintln(b, "\nDuplicate country owned_fleets entries or container. Fleet order scope unresolved. Use raw tree/byte tools.")
		return
	}
	seen := map[int]bool{}
	var rows strings.Builder
	for _, entry := range manager.OwnedFleets {
		if entry.FleetID == nil || seen[*entry.FleetID] {
			continue
		}
		fleetID := *entry.FleetID
		seen[fleetID] = true
		fleet, fleetFound := gs.Fleet[fleetID]
		if !fleetFound {
			if selected == nil {
				fmt.Fprintf(&rows, "  Fleet %d referenced by owned_fleets has no decoded fleet record. Excavation order unavailable.\n", fleetID)
			}
			continue
		}
		order := fleet.CurrentOrderRefs.ArchaeologicalSite
		if order.Unsupported && selected == nil {
			fmt.Fprintf(&rows, "  Fleet %d current_order.excavate_archaeological_site_fleet_order.archaeological_site unsupported or duplicate. Use raw tree/byte tools.\n", fleetID)
			continue
		}
		ref := order.Value
		if ref == nil || (selected != nil && *ref != *selected) {
			continue
		}
		site, exists := gs.ArchaeologySites.Items[*ref]
		_, siteError := gs.ArchaeologySites.DecodeErrors[*ref]
		inScope := len(reasons[*ref]) > 0
		unreliable := gs.ArchaeologySites.TableIssue || siteError
		if !unreliable && inScope && site.ExcavatorFleet != nil && *site.ExcavatorFleet == fleetID {
			continue // Already shown from the site record above.
		}
		fmt.Fprintf(&rows, "  Fleet %d (%s) current_order.excavate_archaeological_site_fleet_order.archaeological_site=%d (saved reference)\n", fleetID, name(fleet.Name), *ref)
		switch {
		case unreliable:
			fmt.Fprintln(&rows, "    Site reference unresolved because saved site data is unsupported or duplicate")
		case !exists:
			fmt.Fprintln(&rows, "    Site record unavailable")
		case !inScope:
			fmt.Fprintln(&rows, "    Site has no matching saved country references")
		case site.ExcavatorFleetAmbiguous:
			fmt.Fprintln(&rows, "    Site excavator_fleet fields are duplicate")
		case site.ExcavatorFleet == nil:
			fmt.Fprintln(&rows, "    Site excavator_fleet not recorded")
		default:
			fmt.Fprintf(&rows, "    Site excavator_fleet=%d (saved ID differs)\n", *site.ExcavatorFleet)
		}
	}
	if rows.Len() > 0 {
		fmt.Fprintln(b, "\nRemaining excavation order references (saved owned_fleets order)")
		b.WriteString(rows.String())
	}
}

func writeExcavationOrderCandidate(b *strings.Builder, siteID int, fleet gamestate.Fleet, siteUnsupported bool) {
	fmt.Fprintf(b, "    Possible fleet ID match: %s", name(fleet.Name))
	orders := fleet.CurrentOrderRefs
	ref := orders.ArchaeologicalSite
	switch {
	case siteUnsupported || orders.Unsupported || ref.Unsupported:
		fmt.Fprintln(b, "; site or fleet order unsupported or duplicate. Reciprocal reference unresolved.")
	case ref.Value == nil:
		fmt.Fprintln(b, "; no reverse site link validated")
	case *ref.Value != siteID:
		fmt.Fprintf(b, "; current_order.excavate_archaeological_site_fleet_order.archaeological_site=%d (saved ID differs)\n", *ref.Value)
	default:
		fmt.Fprintf(b, "; current_order.excavate_archaeological_site_fleet_order.archaeological_site=%d (reciprocal saved IDs agree)\n", *ref.Value)
	}
}
