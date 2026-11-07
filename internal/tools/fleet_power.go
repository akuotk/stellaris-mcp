package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerFleetPower(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_fleet_power", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("List fleets with power (summary). Pass fleet_id for ship-level detail with weapons and HP."),
			mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save")),
			mcp.WithNumber("fleet_id", mcp.Description("Fleet ID for detailed view (omit for summary)"), mcp.DefaultNumber(-1)),
		),
		handleFleetPower,
	)
}

func handleFleetPower(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}

	fleetID := int(req.GetFloat("fleet_id", -1))
	if fleetID >= 0 {
		return fleetDetail(gs, fleetID)
	}

	_, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return fleetSummary(gs, c)
}

func fleetSummary(gs *gamestate.GameState, c *gamestate.Country) (*mcp.CallToolResult, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Fleet Power (%s) ===\n", name(c.Adjective))
	fmt.Fprintf(&b, "Total Military Power: %s\n", armyFloat(c.MilitaryPowerRaw))
	fmt.Fprintf(&b, "Fleet Size: %.0f (Naval Cap: %s)\n", c.FleetSize, formatNavalCapacity(c))
	fmt.Fprintf(&b, "Use get_fleet_power with fleet_id for ship details.\n\n")

	fmt.Fprintf(&b, "%-8s %-30s %-25s %8s %5s\n", "ID", "Name", "Class", "Power", "Ships")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 80))

	milPower := 0.0
	missingFleets := 0
	missingMilitaryPower := 0
	for _, of := range c.FleetsManager.OwnedFleets {
		if of.FleetID == nil {
			missingFleets++
			fmt.Fprintln(&b, "Fleet reference: not recorded")
			continue
		}
		fleet, ok := gs.Fleet[*of.FleetID]
		if !ok {
			missingFleets++
			fmt.Fprintf(&b, "Fleet %d: referenced record unavailable\n", *of.FleetID)
			continue
		}
		role := l(fleet.ShipClass)
		if fleet.Station {
			role += " (station)"
		}
		if fleet.Civilian {
			role += " (civilian)"
		}
		fmt.Fprintf(&b, "%-8d %-30s %-25s %8s %5d\n",
			*of.FleetID, truncate(name(fleet.Name), 30), role, armyFloat(fleet.MilitaryPowerRaw), len(fleet.Ships))
		if fleet.ShipClass == "shipclass_military" {
			if fleet.MilitaryPowerRaw == nil {
				missingMilitaryPower++
			} else {
				milPower += *fleet.MilitaryPowerRaw
			}
		}
	}

	fmt.Fprintf(&b, "\nMilitary-class fleet power: %.1f\n", milPower)
	if missingFleets > 0 {
		fmt.Fprintf(&b, "  Incomplete: %d referenced fleet records unavailable.\n", missingFleets)
	}
	if missingMilitaryPower > 0 {
		fmt.Fprintf(&b, "  Incomplete: %d military-class fleet power values not recorded.\n", missingMilitaryPower)
	}

	// Comparison
	fmt.Fprintf(&b, "\nComparison with Known Empires:\n")
	hasNeighbors := false
	for _, rel := range c.RelationsManager.Relation {
		if rel.ContactRaw == nil || *rel.ContactRaw != "yes" {
			continue
		}
		hasNeighbors = true
		if rel.CountryID == nil {
			fmt.Fprintln(&b, "  Contact country reference not recorded")
		} else if neighbor, ok := gs.Country[*rel.CountryID]; ok {
			fmt.Fprintf(&b, "  %s: %s military power\n", name(neighbor.Adjective), armyFloat(neighbor.MilitaryPowerRaw))
		} else {
			fmt.Fprintf(&b, "  Contact country %d: referenced record unavailable\n", *rel.CountryID)
		}
	}
	if !hasNeighbors {
		fmt.Fprintln(&b, "  No saved contact=yes relations.")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func fleetDetail(gs *gamestate.GameState, fleetID int) (*mcp.CallToolResult, error) {
	fleet, ok := gs.Fleet[fleetID]
	if !ok {
		return toolError(fmt.Errorf("fleet %d not found", fleetID)), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Fleet %d: %s ===\n", fleetID, name(fleet.Name))
	fmt.Fprintf(&b, "Class: %s\n", l(fleet.ShipClass))
	fmt.Fprintf(&b, "Military Power: %s\n", armyFloat(fleet.MilitaryPowerRaw))
	fmt.Fprintf(&b, "Ships: %d\n", len(fleet.Ships))
	if fleet.Station {
		fmt.Fprintf(&b, "Station: yes\n")
	}
	if fleet.Civilian {
		fmt.Fprintf(&b, "Civilian: yes\n")
	}

	for _, sid := range fleet.Ships {
		ship, ok := gs.Ships[sid]
		if !ok {
			fmt.Fprintf(&b, "\n  Ship %d: referenced record unavailable\n", sid)
			continue
		}
		// Resolve design
		designName := "unknown"
		shipSize := "unknown"
		var design *gamestate.ShipDesign
		if id := ship.ShipDesignImplementation.Design; id != nil {
			if d, ok := gs.ShipDesign[*id]; ok {
				design = &d
				designName = name(d.Name)
				if len(d.GrowthStages) == 1 {
					shipSize = d.GrowthStages[0].ShipSize
				}
			}
		}

		fmt.Fprintf(&b, "\n  --- Ship %d: %s ---\n", sid, name(ship.Name))
		if id := ship.ShipDesignImplementation.Design; id != nil {
			fmt.Fprintf(&b, "  Design reference: %d\n", *id)
		} else {
			fmt.Fprintf(&b, "  Design reference: not recorded\n")
		}
		if stage := ship.ShipDesignImplementation.GrowthStage; stage != nil {
			fmt.Fprintf(&b, "  Saved design implementation growth_stage: %d (index interpretation not established)\n", *stage)
		}
		if upgrade := ship.ShipDesignImplementation.Upgrade; upgrade != nil {
			fmt.Fprintf(&b, "  Saved design implementation upgrade: %d\n", *upgrade)
		}
		fmt.Fprintf(&b, "  Design: %s (%s)\n", designName, l(shipSize))
		fmt.Fprintf(&b, "  Hull:   %.0f / %.0f\n", ship.Hitpoints, ship.MaxHitpoints)
		fmt.Fprintf(&b, "  Shield: %.0f / %.0f\n", ship.ShieldHitpoints, ship.MaxShieldHitpoints)
		fmt.Fprintf(&b, "  Armor:  %.0f / %.0f\n", ship.ArmorHitpoints, ship.MaxArmorHitpoints)

		// Components from design (weapons, utilities, aux, required)
		if design != nil {
			if len(design.GrowthStages) == 0 {
				fmt.Fprintf(&b, "  Design growth stages: not recorded\n")
			}
			for index, stage := range design.GrowthStages {
				fmt.Fprintf(&b, "  Saved design growth stage %d (ship size: %s; actual stage not established):\n", index, l(stage.ShipSize))
				var weapons, utilities, aux []string
				for _, section := range stage.Sections {
					for _, comp := range section.Component {
						name := l(comp.Template)
						switch {
						case strings.Contains(comp.Slot, "GUN") || strings.Contains(comp.Slot, "TORPEDO") || strings.Contains(comp.Slot, "HANGAR") || strings.Contains(comp.Slot, "STRIKE"):
							weapons = append(weapons, name)
						case strings.Contains(comp.Slot, "AUX"):
							aux = append(aux, name)
						default:
							utilities = append(utilities, name)
						}
					}
				}
				var required []string
				for _, r := range stage.RequiredComponent {
					required = append(required, l(r))
				}

				if len(weapons) > 0 {
					fmt.Fprintf(&b, "  Weapons: %s\n", strings.Join(weapons, ", "))
				}
				if len(utilities) > 0 {
					fmt.Fprintf(&b, "  Utilities: %s\n", strings.Join(utilities, ", "))
				}
				if len(aux) > 0 {
					fmt.Fprintf(&b, "  Aux: %s\n", strings.Join(aux, ", "))
				}
				if len(required) > 0 {
					fmt.Fprintf(&b, "  Core: %s\n", strings.Join(required, ", "))
				}
			}
		}
	}

	return mcp.NewToolResultText(b.String()), nil
}
