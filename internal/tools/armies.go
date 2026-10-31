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

func registerArmies(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_armies", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Read armies referenced by country.owned_armies or army.owner; resolve colony/carrier, transport ship/fleet, species and leader links. Reports missing/inconsistent back-references. Health, morale and experience remain raw; army_id selects details."), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")), mcp.WithNumber("army_id", mcp.Description("Owned/referenced army ID for full details; omit for summary"))), handleArmies)
}

func handleArmies(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	countryID, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["army_id"]; ok {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			maxExact := int64(1<<53 - 1)
			maxID = int(maxExact)
		}
		id, err := rawInteger(req, "army_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &id
	}
	return renderArmies(gs, countryID, country, selected), nil
}

func renderArmies(gs *gamestate.GameState, countryID int, country *gamestate.Country, selected *int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Armies (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope uses saved army.owner or country.owned_armies references. Statistics are raw save values; battle strength, UI scaling and transit phases are not inferred.")
	if gs.Armies.TableIssue || len(gs.Armies.DecodeErrors) > 0 {
		fmt.Fprintln(&b, "Army decoding is incomplete for some saved records; use raw tree/byte tools for complete data.")
	}
	references := map[int]int{}
	idsSet := map[int]bool{}
	for _, id := range country.OwnedArmies {
		references[id]++
		idsSet[id] = true
	}
	for id, army := range gs.Armies.Items {
		if army.Owner != nil && *army.Owner == countryID {
			idsSet[id] = true
		}
	}
	ids := make([]int, 0, len(idsSet))
	for id := range idsSet {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if selected != nil {
		if !idsSet[*selected] {
			return toolError(fmt.Errorf("army %d is not owned/referenced by selected country", *selected))
		}
		ids = []int{*selected}
	}
	if len(ids) == 0 {
		fmt.Fprintln(&b, "No matching decoded army records.")
	}
	for _, id := range ids {
		army, ok := gs.Armies.Items[id]
		if !ok {
			fmt.Fprintf(&b, "\nArmy %d: referenced record unavailable\n", id)
			continue
		}
		fmt.Fprintf(&b, "\nArmy %d: %s; type=%s owner=%s colony=%s ship=%s\n", id, name(army.Name), army.Type, armyInt(army.Owner), armyInt(army.Colony), armyInt(army.Ship))
		if _, bad := gs.Armies.DecodeErrors[id]; bad {
			fmt.Fprintln(&b, "  Record decoding incomplete; displayed fields may be partial")
		}
		switch {
		case country.OwnedArmies == nil:
			fmt.Fprintln(&b, "  country.owned_armies: not recorded")
		case references[id] == 0:
			fmt.Fprintln(&b, "  army.owner matches country, but country.owned_armies lacks this ID")
		default:
			fmt.Fprintf(&b, "  country.owned_armies membership: %d reference(s)\n", references[id])
		}
		if references[id] > 1 {
			fmt.Fprintln(&b, "  Duplicate country.owned_armies references recorded")
		}
		if references[id] > 0 && army.Owner != nil && *army.Owner != countryID {
			fmt.Fprintln(&b, "  Ownership mismatch: country list and army.owner disagree")
		}
		if selected == nil {
			continue
		}
		fmt.Fprintf(&b, "  health=%s max_health=%s morale=%s experience=%s\n", armyFloat(army.Health), armyFloat(army.MaxHealth), armyFloat(army.Morale), armyFloat(army.Experience))
		if army.JumpDriveCooldown == "" {
			fmt.Fprintln(&b, "  jump_drive_cooldown: not recorded")
		} else {
			fmt.Fprintf(&b, "  jump_drive_cooldown (as saved): %s\n", army.JumpDriveCooldown)
		}
		if army.Species == nil {
			fmt.Fprintln(&b, "  Species: not recorded")
		} else {
			fmt.Fprintf(&b, "  Species: %s\n", populationSpecies(gs, *army.Species))
		}
		if army.Leader == nil {
			fmt.Fprintln(&b, "  Leader: not recorded")
		} else if leader, ok := gs.Leaders[*army.Leader]; ok {
			fmt.Fprintf(&b, "  Leader %d: %s\n", *army.Leader, name(leader.Name.FullNames))
		} else {
			fmt.Fprintf(&b, "  Leader %d: record unavailable\n", *army.Leader)
		}
		if army.Colony != nil {
			writeArmyColony(&b, gs, id, *army.Colony, false)
		}
		if army.SpawningColony != nil {
			writeArmyColony(&b, gs, id, *army.SpawningColony, true)
		}
		if army.Planet != nil {
			if planet, ok := gs.Planets.Planet[*army.Planet]; ok {
				fmt.Fprintf(&b, "  Planet reference (as saved) %d: possible planet ID match %s\n", *army.Planet, name(planet.Name))
			} else {
				fmt.Fprintf(&b, "  Planet reference (as saved) %d: record unavailable\n", *army.Planet)
			}
		}
		if army.Ship != nil {
			writeArmyTransport(&b, gs, id, *army.Ship)
		}
		if army.Colony != nil && army.Ship != nil {
			fmt.Fprintln(&b, "  Both colony and ship location references are saved; movement/battle phase is not inferred")
		}
		if army.Colony == nil && army.Ship == nil && army.Planet == nil {
			fmt.Fprintln(&b, "  No location reference recorded")
		}
	}
	return mcp.NewToolResultText(b.String())
}

func writeArmyColony(b *strings.Builder, gs *gamestate.GameState, armyID, colonyID int, spawning bool) {
	label := "Colony"
	if spawning {
		label = "Spawning colony"
	}
	colony, ok := gs.Colonies[colonyID]
	if !ok {
		fmt.Fprintf(b, "  %s %d: record unavailable\n", label, colonyID)
		return
	}
	fmt.Fprintf(b, "  %s %d: %s\n", label, colonyID, colonyCarrier(gs, colonyID, colony))
	refs := colony.Armies
	field := "colony.army"
	if spawning {
		refs = colony.SpawnedArmies
		field = "colony.spawned_armies"
	}
	fmt.Fprintf(b, "    %s membership: %s\n", field, armyMembership(refs, armyID))
}

func writeArmyTransport(b *strings.Builder, gs *gamestate.GameState, armyID, shipID int) {
	ship, ok := gs.Ships[shipID]
	if !ok {
		fmt.Fprintf(b, "  Ship reference %d: record unavailable\n", shipID)
		return
	}
	fmt.Fprintf(b, "  Ship reference %d: %s\n", shipID, name(ship.Name))
	if ship.ArmyID == nil {
		fmt.Fprintln(b, "    ship.army: not recorded")
	} else if *ship.ArmyID != armyID {
		fmt.Fprintf(b, "    ship.army back-reference mismatch: %d\n", *ship.ArmyID)
	} else {
		fmt.Fprintln(b, "    army/ship back-reference validated")
	}
	if ship.FleetID == nil {
		fmt.Fprintln(b, "    ship.fleet: not recorded")
		return
	}
	fleet, ok := gs.Fleet[*ship.FleetID]
	if !ok {
		fmt.Fprintf(b, "    Fleet %d: record unavailable\n", *ship.FleetID)
		return
	}
	fmt.Fprintf(b, "    Fleet %d: %s; fleet.ships membership: %s\n", *ship.FleetID, name(fleet.Name), armyMembership(fleet.Ships, shipID))
}

func armyMembership(ids []int, id int) string {
	if ids == nil {
		return "list not recorded"
	}
	count := 0
	for _, ref := range ids {
		if ref == id {
			count++
		}
	}
	if count == 0 {
		return "missing reverse reference"
	}
	if count > 1 {
		return fmt.Sprintf("%d duplicate references", count)
	}
	return "validated"
}
func armyInt(value *int) string {
	if value == nil {
		return "not recorded"
	}
	return fmt.Sprint(*value)
}
func armyFloat(value *float64) string {
	if value == nil {
		return "not recorded"
	}
	return fmt.Sprintf("%g", *value)
}
