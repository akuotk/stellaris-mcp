package tools

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerShipLoadout(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_ship_loadout", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read a saved ship's actual sections and weapons, or read a saved design directly. Supply exactly one of ship_id or design_id. Checks ship/fleet membership and compares saved section and component slots with legacy and growth-stage designs. Runtime weapon values and growth_stage codes remain raw. Explicit leader, army, kill_target, owner, upgrade design, species, aura source and weapon/targeting target IDs are listed with numeric table candidates, plus ship records naming the selected ship."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("ship_id", mcp.Description("ID in the ships table; use get_fleet_power for member IDs")),
		mcp.WithNumber("design_id", mcp.Description("ID in the ship_design table"))), handleShipLoadout)
}

func handleShipLoadout(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	_, shipProvided := args["ship_id"]
	_, designProvided := args["design_id"]
	if shipProvided == designProvided {
		return toolError(fmt.Errorf("supply exactly one of ship_id or design_id")), nil
	}
	if value, ok := args["save_id"]; ok {
		if _, ok := value.(string); !ok {
			return toolError(fmt.Errorf("save_id must be a string")), nil
		}
	}
	key := "ship_id"
	if designProvided {
		key = "design_id"
	}
	maxID := int(^uint(0) >> 1)
	if strconv.IntSize == 64 {
		maxExact := int64(1<<53 - 1)
		maxID = int(maxExact)
	}
	id, err := rawInteger(req, key, 0, -maxID, maxID)
	if err != nil {
		return toolError(err), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	var shipID, designID *int
	if shipProvided {
		shipID = &id
	} else {
		designID = &id
	}
	loadout, err := gamestate.LoadShipLoadoutFromArchive(archive.zip, shipID, designID)
	if err != nil {
		return toolError(err), nil
	}
	return renderShipLoadout(loadout), nil
}

func renderShipLoadout(loadout *gamestate.ShipLoadout) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintln(&b, "=== Saved ship loadout and design ===")
	fmt.Fprintln(&b, "Ship, fleet and design IDs refer to separate tables. Slot comparisons use saved keys and templates. Weapon values, growth_stage codes and upgrade meaning remain unverified.")
	if shipID := loadout.ShipID; shipID != nil {
		fmt.Fprintf(&b, "\nShip %d (ships table)\n", *shipID)
		if loadout.Ship.Issue != "" {
			fmt.Fprintf(&b, "  %s. Use raw tree/byte tools for unavailable detail.\n", loadout.Ship.Issue)
			return mcp.NewToolResultText(b.String())
		}
		if loadout.SameNumericFleetID {
			fmt.Fprintln(&b, "  A fleet record also uses this numeric ID. Membership follows the saved ship.fleet reference.")
		}
		writeLoadoutFleetMembership(&b, loadout)
		fmt.Fprintf(&b, "  ship_design=%s ship_design_implementation.design=%s growth_stage=%s upgrade=%s (saved references)\n", loadoutRefText(loadout.LegacyDesign), loadoutRefText(loadout.ImplDesign), loadoutRefText(loadout.GrowthStage), loadoutRefText(loadout.Upgrade))
		writeLoadoutFields(&b, "Saved ship fields", loadout.Ship.Value, "section")
		writeLoadoutSections(&b, loadout.ShipLayout, "weapon")
		writeShipReferences(&b, loadout)
	}
	if loadout.DesignID == nil {
		if loadout.LegacyDesign.Recorded || loadout.ImplDesign.Recorded {
			fmt.Fprintln(&b, "\nDesign selection unresolved because saved references are unsupported or disagree.")
		} else {
			fmt.Fprintln(&b, "\nDesign reference not recorded")
		}
		return mcp.NewToolResultText(b.String())
	}
	fmt.Fprintf(&b, "\nDesign %d (ship_design table)\n", *loadout.DesignID)
	if loadout.Design.Issue != "" {
		fmt.Fprintf(&b, "  %s. Use raw tree/byte tools for unavailable detail.\n", loadout.Design.Issue)
		return mcp.NewToolResultText(b.String())
	}
	writeLoadoutFields(&b, "Saved design fields", loadout.Design.Value, "section", "growth_stages")
	if loadout.GrowthStagesUnsupported {
		fmt.Fprintln(&b, "growth_stages unsupported or duplicate. Use raw tree/byte tools.")
	}
	if len(loadout.DesignLayouts) == 0 {
		fmt.Fprintln(&b, "No decoded design layouts")
	}
	for _, layout := range loadout.DesignLayouts {
		if layout.Stage == nil {
			fmt.Fprintln(&b, "\nSaved legacy design layout")
		} else {
			fmt.Fprintf(&b, "\nSaved design growth-stage position %d\n", *layout.Stage)
			writeLoadoutFields(&b, "Saved growth-stage fields", layout.Value, "section")
		}
		writeLoadoutSections(&b, layout, "component")
		if loadout.ShipID != nil {
			comparison := gamestate.CompareShipLoadout(loadout.ShipLayout, layout)
			fmt.Fprintf(&b, "  Section comparison: %s\n", comparison.SectionStatus)
			fmt.Fprintf(&b, "  Saved weapon slot/template comparison: matches=%d differs=%d unresolved=%d\n", comparison.Matched, comparison.Different, comparison.Unresolved)
			fmt.Fprintf(&b, "  Design component entries without a matching saved weapon slot=%d (component roles unverified)\n", comparison.DesignOnly)
		}
	}
	return mcp.NewToolResultText(b.String())
}

func writeShipReferences(b *strings.Builder, loadout *gamestate.ShipLoadout) {
	fmt.Fprintln(b, "\nExplicit saved reference fields (saved order; numeric table candidates only. Type codes, target namespaces, activity and ownership are unverified)")
	if len(loadout.References) == 0 {
		fmt.Fprintln(b, "  None recorded")
	}
	for _, ref := range loadout.References {
		fmt.Fprintf(b, "  %s=%s%s -> %s: %s\n", ref.Path, shipRefIDText(ref.ID), shipRefTypeText(ref.Type), ref.Table, ref.Status)
	}
	if loadout.InboundUnscanned < 0 {
		fmt.Fprintln(b, "Inbound ship references unavailable: ships table not recorded, unsupported or duplicate")
		return
	}
	fmt.Fprintf(b, "Ship records whose explicit ship-table reference fields name ID %d: references=%d records=%d (numeric candidates; includes this record)\n", *loadout.ShipID, loadout.InboundCount, loadout.InboundShips)
	if loadout.InboundUnscanned > 0 {
		fmt.Fprintf(b, "  %d ships entries were not scanned (non-numeric key or unsupported shape)\n", loadout.InboundUnscanned)
	}
	for _, ref := range loadout.Inbound {
		self := ""
		if ref.ShipID == *loadout.ShipID {
			self = " (this record)"
		}
		fmt.Fprintf(b, "  ship %d%s %s%s\n", ref.ShipID, self, ref.Path, shipRefTypeText(ref.Type))
	}
	if loadout.InboundCount > len(loadout.Inbound) {
		fmt.Fprintf(b, "  first %d shown; use search_save_tree or raw tools for the rest\n", len(loadout.Inbound))
	}
}

func shipRefIDText(id *int) string {
	if id == nil {
		return "(no usable integer)"
	}
	return fmt.Sprint(*id)
}

func shipRefTypeText(typ *gamestate.OptionalString) string {
	switch {
	case typ == nil:
		return ""
	case typ.Unsupported:
		return " type=unsupported or duplicate"
	default:
		return fmt.Sprintf(" type=%q", typ.Value)
	}
}

func writeLoadoutFleetMembership(b *strings.Builder, loadout *gamestate.ShipLoadout) {
	fmt.Fprintf(b, "  ship.fleet=%s (saved reference)\n", loadoutRefText(loadout.FleetRef))
	if loadout.FleetRef.Value == nil {
		return
	}
	if loadout.Fleet.Issue != "" {
		fmt.Fprintf(b, "  Fleet %d: %s\n", *loadout.FleetRef.Value, loadout.Fleet.Issue)
		return
	}
	if loadout.FleetShips.Unsupported || !loadout.FleetShips.Recorded {
		fmt.Fprintln(b, "  Fleet ships membership unresolved. Saved ships list missing, unsupported or duplicate.")
		return
	}
	matches := 0
	for _, id := range loadout.FleetShips.Values {
		if id == *loadout.ShipID {
			matches++
		}
	}
	switch matches {
	case 0:
		fmt.Fprintln(b, "  Fleet ships list has no matching saved ship ID")
	case 1:
		fmt.Fprintln(b, "  Reciprocal saved ship.fleet and fleet.ships IDs agree")
	default:
		fmt.Fprintf(b, "  Ambiguous fleet ships membership (%d duplicate saved references)\n", matches)
	}
}

func writeLoadoutSections(b *strings.Builder, layout gamestate.LoadoutLayout, componentKey string) {
	if len(layout.Sections) == 0 && layout.Value.IsObject() {
		fmt.Fprintln(b, "  Sections not recorded")
	}
	for position, section := range layout.Sections {
		writeLoadoutFields(b, fmt.Sprintf("  Section[%d] (saved order)", position), section.Value, componentKey)
		for index, component := range section.Components {
			writeSavedValue(b, fmt.Sprintf("    %s[%d] (saved order)", componentKey, index), component.Value, 0)
		}
	}
}

func writeLoadoutFields(b *strings.Builder, label string, value clausewitz.Value, except ...string) {
	if value.IsObject() {
		fields := &clausewitz.Object{}
		for _, pair := range value.Object.Pairs {
			if !slices.Contains(except, pair.Key) {
				fields.Pairs = append(fields.Pairs, pair)
			}
		}
		value = clausewitz.Value{Object: fields}
	}
	writeSavedValue(b, label, value, 0)
}

func loadoutRefText(ref gamestate.OptionalIntRef) string {
	if ref.Unsupported {
		return "unsupported or duplicate"
	}
	return armyInt(ref.Value)
}
