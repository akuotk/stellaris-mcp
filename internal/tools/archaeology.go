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
	s.AddTool(mcp.NewTool("get_archaeology", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Read archaeological sites matching visible_to, last_excavator_country or saved completed-country entries. Shows raw location/fleet references as candidates, not verified identities. site_id selects saved event/log details. Progress units, numeric location enums, phase and completion status are not inferred."), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")), mcp.WithNumber("site_id", mcp.Description("Matching archaeological site ID for full details; omit for summary"))), handleArchaeology)
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
		if site.ExcavatorFleet != nil {
			if fleet, ok := gs.Fleet[*site.ExcavatorFleet]; ok {
				fmt.Fprintf(&b, "    Possible fleet ID match: %s; no reverse site link validated\n", name(fleet.Name))
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
	return mcp.NewToolResultText(b.String())
}
