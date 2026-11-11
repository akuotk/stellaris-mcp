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

func registerStarbases(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_starbases", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Read starbase records with saved construction-queue owner or candidate station-ship fleet references matching this country. Shows station/design/system/fleet and queue links, preserving missing/inconsistent references. These paths do not prove starbase ownership. starbase_id selects modules/buildings/orbitals details; slot meanings remain raw."), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")), mcp.WithNumber("starbase_id", mcp.Description("Matching starbase ID for details; omit for summary"))), handleStarbases)
}

func handleStarbases(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["starbase_id"]; ok {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			maxExact := int64(1<<53 - 1)
			maxID = int(maxExact)
		}
		baseID, err := rawInteger(req, "starbase_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &baseID
	}
	return renderStarbases(gs, id, country, selected), nil
}

func renderStarbases(gs *gamestate.GameState, countryID int, country *gamestate.Country, selected *int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Starbase reference records (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope uses saved queue.owner or country fleet references via a candidate station ship. These paths do not prove starbase ownership; ID matches remain candidates and slot/orbital meanings are not inferred.")
	if !gs.Starbases.Recorded {
		fmt.Fprintln(&b, "Starbase table not recorded.")
	}
	if gs.Starbases.TableIssue || len(gs.Starbases.DecodeErrors) > 0 {
		fmt.Fprintln(&b, "Starbase decoding incomplete for some records; malformed or duplicate records are excluded from country paths. Use raw tree/byte tools for complete data.")
	}
	if gs.Construction.DecodeError != "" {
		fmt.Fprintln(&b, "Construction decoding incomplete; queue owner paths are unavailable. Use raw tree/byte tools for complete queue data.")
	}
	// These are global saved references, independent of the selected country's scope.
	maxUint32Refs, otherUnmatched, unknownLists := 0, 0, 0
	for _, system := range gs.Systems {
		if system.StarbaseRefs.Unsupported {
			unknownLists++
		}
		for _, ref := range system.StarbaseRefs.Values {
			if ref >= 0 && uint64(ref) == uint64(4294967295) {
				maxUint32Refs++
				continue
			}
			if _, ok := gs.Starbases.Items[ref]; !ok {
				otherUnmatched++
			}
		}
	}
	fmt.Fprintf(&b, "System starbase reference survey (all saved systems): raw 4294967295=%d (meaning unverified); other IDs without a decoded base record=%d", maxUint32Refs, otherUnmatched)
	if unknownLists > 0 {
		fmt.Fprintf(&b, "; undecodable starbase lists=%d (their entries are not included in the counts above)", unknownLists)
	}
	fmt.Fprintln(&b)
	if !gs.Starbases.Recorded || gs.Starbases.TableIssue || len(gs.Starbases.DecodeErrors) > 0 || unknownLists > 0 {
		fmt.Fprintln(&b, "  Survey coverage incomplete; use raw tools for unknown tables or lists.")
	}
	fleets := map[int]bool{}
	missingFleetRefs := 0
	for _, ref := range country.FleetsManager.OwnedFleets {
		if ref.FleetID == nil {
			missingFleetRefs++
		} else {
			fleets[*ref.FleetID] = true
		}
	}
	if missingFleetRefs > 0 {
		fmt.Fprintf(&b, "Country fleet entries without fleet ID: %d; these are not treated as ID zero\n", missingFleetRefs)
	}
	routes := map[int][]string{}
	for id, base := range gs.Starbases.Items {
		if _, bad := gs.Starbases.DecodeErrors[id]; bad {
			continue
		}
		for _, ref := range []struct {
			label string
			id    *int
		}{{"build_queue.owner", base.BuildQueue}, {"shipyard_build_queue.owner", base.ShipyardBuildQueue}} {
			if ref.id != nil && gs.Construction.DecodeError == "" {
				if queue, ok := gs.Construction.QueueManager.Queues[*ref.id]; ok && queue.Owner != nil && *queue.Owner == countryID {
					routes[id] = append(routes[id], ref.label)
				}
			}
		}
		if base.Station != nil {
			if ship, ok := gs.Ships[*base.Station]; ok && ship.FleetID != nil && fleets[*ship.FleetID] {
				routes[id] = append(routes[id], "candidate station ship.fleet in country fleet references")
			}
		}
	}
	ids := make([]int, 0, len(routes))
	for id := range routes {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if selected != nil {
		if len(routes[*selected]) == 0 {
			return toolError(fmt.Errorf("starbase %d has no matching country reference path", *selected))
		}
		ids = []int{*selected}
	}
	if len(ids) == 0 {
		fmt.Fprintln(&b, "No matching decoded starbase records.")
	}
	for _, id := range ids {
		base := gs.Starbases.Items[id]
		fmt.Fprintf(&b, "\nStarbase %d: level=%s type=%s construction_type=%s; country paths=%v\n", id, operationFlagText(base.Level), operationFlagText(base.Type), operationFlagText(base.ConstructionType), routes[id])
		fmt.Fprintf(&b, "  station=%s design=%s upgrade=%s growth_stage=%s (saved values)\n", armyInt(base.Station), armyInt(base.ShipDesign.Design), armyInt(base.ShipDesign.Upgrade), armyInt(base.ShipDesign.GrowthStage))
		if base.ShipDesign.Design != nil {
			if design, ok := gs.ShipDesign[*base.ShipDesign.Design]; ok {
				fmt.Fprintf(&b, "  Saved design ID record: %s\n", name(design.Name))
			} else {
				fmt.Fprintln(&b, "  Saved design ID record unavailable")
			}
		}
		if base.Modules.Unsupported || base.Buildings.Unsupported || base.Orbitals.Unsupported {
			fmt.Fprintln(&b, "  Slot decoding incomplete; use raw tree/byte tools for complete slots")
		}
		var station *gamestate.Ship
		if base.Station != nil {
			if ship, ok := gs.Ships[*base.Station]; ok {
				station = &ship
				fmt.Fprintf(&b, "  Possible station ship ID match: %s; reverse base identity not verified\n", name(ship.Name))
				if base.ShipDesign.Design != nil && ship.ShipDesignImplementation.Design != nil {
					if *base.ShipDesign.Design == *ship.ShipDesignImplementation.Design {
						fmt.Fprintln(&b, "    Base/ship saved design IDs agree")
					} else {
						fmt.Fprintln(&b, "    Base/ship saved design IDs disagree")
					}
				}
				fmt.Fprintf(&b, "    ship.fleet=%s\n", armyInt(ship.FleetID))
				if ship.FleetID != nil {
					if fleet, ok := gs.Fleet[*ship.FleetID]; ok {
						fmt.Fprintf(&b, "    Fleet record %d: %s; fleet.ships membership: %s\n", *ship.FleetID, name(fleet.Name), armyMembership(fleet.Ships, *base.Station))
					} else {
						fmt.Fprintln(&b, "    Referenced fleet record unavailable")
					}
				}
				if ship.Coordinate.Unsupported {
					fmt.Fprintln(&b, "    ship.coordinate: unsupported; use raw tools")
				} else if !ship.Coordinate.Recorded {
					fmt.Fprintln(&b, "    ship.coordinate: not recorded")
				} else {
					fmt.Fprintf(&b, "    ship.coordinate.origin=%s\n", armyInt(ship.Coordinate.Origin))
				}
			} else {
				fmt.Fprintln(&b, "  station ID has no decoded ship candidate; namespace/sentinel meaning unverified")
			}
		}
		systemIDs := []int{}
		unsupportedSystems := 0
		missingSystemRefs := 0
		for systemID, system := range gs.Systems {
			if system.StarbaseRefs.Unsupported {
				unsupportedSystems++
			}
			if !system.StarbaseRefs.Recorded {
				missingSystemRefs++
			}
			for _, ref := range system.StarbaseRefs.Values {
				if ref == id {
					systemIDs = append(systemIDs, systemID)
					break
				}
			}
		}
		sort.Ints(systemIDs)
		for _, systemID := range systemIDs {
			system := gs.Systems[systemID]
			fmt.Fprintf(&b, "  System candidate %d: %s; system.starbases membership: %s\n", systemID, name(system.Name), armyMembership(system.StarbaseRefs.Values, id))
			if station != nil && !station.Coordinate.Unsupported && station.Coordinate.Origin != nil {
				if *station.Coordinate.Origin == systemID {
					fmt.Fprintln(&b, "    Candidate station coordinate ID agrees with system reference; ownership not inferred")
				} else {
					fmt.Fprintln(&b, "    Candidate station coordinate ID differs from referencing system ID")
				}
			}
		}
		if len(systemIDs) == 0 {
			fmt.Fprintln(&b, "  No matching decoded system.starbases references")
		}
		if unsupportedSystems > 0 {
			fmt.Fprintf(&b, "  Unsupported system.starbases fields: %d; reference coverage incomplete\n", unsupportedSystems)
		}
		if missingSystemRefs > 0 {
			fmt.Fprintf(&b, "  Systems without recorded starbase references: %d\n", missingSystemRefs)
		}
		if selected == nil {
			continue
		}
		writeStarbaseQueue(&b, gs, countryID, "build_queue", base.BuildQueue)
		writeStarbaseQueue(&b, gs, countryID, "shipyard_build_queue", base.ShipyardBuildQueue)
		writeStarbaseSlots(&b, "modules", base.Modules)
		writeStarbaseSlots(&b, "buildings", base.Buildings)
		writeStarbaseSlots(&b, "orbitals (raw key/value; namespace unverified)", base.Orbitals)
	}
	return mcp.NewToolResultText(b.String())
}

func writeStarbaseQueue(b *strings.Builder, gs *gamestate.GameState, countryID int, label string, id *int) {
	if id == nil {
		fmt.Fprintf(b, "  %s: not recorded\n", label)
		return
	}
	if gs.Construction.DecodeError != "" {
		fmt.Fprintf(b, "  %s %d: queue decoding incomplete; use raw tools\n", label, *id)
		return
	}
	queue, ok := gs.Construction.QueueManager.Queues[*id]
	if !ok {
		fmt.Fprintf(b, "  %s %d: referenced queue unavailable\n", label, *id)
		return
	}
	fmt.Fprintf(b, "  %s %d: owner=%s type=%s location.type=%s location.id=%s items=%s (saved values; location namespace unverified)\n", label, *id, armyInt(queue.Owner), queue.Type, armyInt(queue.Location.Type), armyInt(queue.Location.ID), colonyReferenceList(queue.Items))
	if queue.Owner != nil && *queue.Owner != countryID {
		fmt.Fprintln(b, "    Queue owner differs from selected country")
	}
}

func writeStarbaseSlots(b *strings.Builder, label string, slots gamestate.StarbaseSlots) {
	if !slots.Recorded {
		fmt.Fprintf(b, "  %s: not recorded\n", label)
		return
	}
	if slots.Unsupported {
		fmt.Fprintf(b, "  %s: unsupported saved entries; supported entries below may be partial, use raw tools\n", label)
	}
	if len(slots.Entries) == 0 {
		if !slots.Unsupported {
			fmt.Fprintf(b, "  %s: empty\n", label)
		}
		return
	}
	seen := map[string]bool{}
	for index, entry := range slots.Entries {
		fmt.Fprintf(b, "  %s[%d] (saved order): key=%s value=%s", label, index, entry.Key, entry.Value)
		if seen[entry.Key] {
			fmt.Fprint(b, " [duplicate slot key]")
		}
		seen[entry.Key] = true
		fmt.Fprintln(b)
	}
}
