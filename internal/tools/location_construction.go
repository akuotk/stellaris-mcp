package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLocationConstruction(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_location_construction", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Follow a saved planet's or colony's queue fields (build_queue, army_build_queue, externally_owned_build_queues and other *queue* fields) into construction queues and items in saved order. Supply exactly one of planet_id or colony_id; the linked colony or carrier planet is included. Checks queue location IDs and item.queue back-references, and lists other queues with the same location ID and type, and items whose buildable planet value names the linked colony (saves with a colony table) or the planet. Location type codes, ownership, progress units and buildability are not inferred."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("planet_id", mcp.Description("ID in planets.planet")),
		mcp.WithNumber("colony_id", mcp.Description("ID in the colony table (newer saves)"))), handleLocationConstruction)
}

func handleLocationConstruction(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	_, planetProvided := args["planet_id"]
	_, colonyProvided := args["colony_id"]
	if planetProvided == colonyProvided {
		return toolError(fmt.Errorf("supply exactly one of planet_id or colony_id")), nil
	}
	if err := requireStringArgs(req, "save_id"); err != nil {
		return toolError(err), nil
	}
	key := "planet_id"
	if colonyProvided {
		key = "colony_id"
	}
	id, err := rawInteger(req, key, 0, 0, maxRecordID)
	if err != nil {
		return toolError(err), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	var planetID, colonyID *int
	if planetProvided {
		planetID = &id
	} else {
		colonyID = &id
	}
	saved, err := gamestate.LoadLocationConstructionFromArchive(archive.zip, planetID, colonyID)
	if err != nil {
		return toolError(err), nil
	}
	return renderLocationConstruction(saved), nil
}

func renderLocationConstruction(saved *gamestate.LocationConstruction) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintln(&b, "=== Saved location construction references ===")
	fmt.Fprintln(&b, "Planet, colony, queue and item IDs refer to separate tables. Matches are numeric; location type codes, ownership, progress units, completion and buildability are not inferred.")
	if saved.PlanetID != nil {
		fmt.Fprintf(&b, "\nPlanet %d (planets.planet): %s\n", *saved.PlanetID, locationRecordStatus(saved.Planet))
		if saved.Planet.Issue == "" {
			fmt.Fprintf(&b, "  planet.colony=%s (saved reference)\n", optionalIntRefText(saved.PlanetColony))
		}
	} else {
		fmt.Fprintln(&b, "\nPlanet not resolved from the colony (colony record, carrier type or carrier reference unavailable; see colony status below)")
	}
	if saved.ColonyID != nil {
		fmt.Fprintf(&b, "Colony %d (colony table): %s\n", *saved.ColonyID, locationRecordStatus(saved.Colony))
		if saved.Colony.Issue == "" {
			fmt.Fprintf(&b, "  colony.carrier.type=%s colony.carrier.reference=%s (saved values)\n", optionalStringText(saved.CarrierType), optionalIntRefText(saved.CarrierRef))
			if saved.PlanetID != nil && saved.Planet.Issue == "" {
				fmt.Fprintf(&b, "  Reciprocal planet.colony and colony.carrier.reference: %s\n", locationReciprocity(saved))
			}
		}
	}
	if saved.QueueTableIssue != "" {
		fmt.Fprintf(&b, "construction.queue_mgr.queues: %s\n", saved.QueueTableIssue)
	}
	if saved.ItemTableIssue != "" {
		fmt.Fprintf(&b, "construction.item_mgr.items: %s\n", saved.ItemTableIssue)
	}
	if saved.ColonyID != nil && saved.ColonyTableIssue != "" {
		fmt.Fprintf(&b, "colony table: %s\n", saved.ColonyTableIssue)
	}
	ctx := locationItemContext{table: saved.ItemLocationTable, id: saved.ItemLocationID}
	switch {
	case saved.ItemLocationTable == "colony" && saved.ItemLocationID != nil:
		fmt.Fprintf(&b, "Item buildable_*.planet values are compared with colony %d: this save has a colony table, and in such saves those fields were observed to hold colony IDs.\n", *saved.ItemLocationID)
	case saved.ItemLocationTable == "colony":
		fmt.Fprintln(&b, "Item buildable_*.planet values are not compared: this save has a colony table (those fields were observed to hold colony IDs) but no colony record is linked here.")
	case saved.ItemLocationID != nil:
		fmt.Fprintf(&b, "Item buildable_*.planet values are compared with planet %d (no colony table in this save).\n", *saved.ItemLocationID)
	}
	fmt.Fprintln(&b, "\nQueue reference fields (saved order)")
	if len(saved.Refs) == 0 {
		fmt.Fprintln(&b, "  None recorded")
	}
	for _, ref := range saved.Refs {
		key := ""
		if ref.Key != nil {
			key = fmt.Sprintf(" key=%s", optionalIntRefText(*ref.Key))
		}
		fmt.Fprintf(&b, "  %s=%s%s\n", ref.Source, optionalIntRefText(ref.Ref), key)
		if ref.Queue != nil {
			writeLocationQueue(&b, *ref.Queue, saved.PlanetID, ctx, "    ")
		}
	}
	if saved.PlanetID != nil && saved.QueueTableIssue == "" {
		scope := "any location.type (no queue reached through saved fields matched this planet)"
		if len(saved.ReferencedLocationTypes) > 0 {
			types := make([]string, len(saved.ReferencedLocationTypes))
			for i, typ := range saved.ReferencedLocationTypes {
				types[i] = fmt.Sprint(typ)
			}
			scope = "location.type " + strings.Join(types, " or ") + " as used by the queues above"
		}
		fmt.Fprintf(&b, "\nOther queues whose saved location.id equals planet %d with %s (numeric candidates): %d\n", *saved.PlanetID, scope, saved.UnreferencedQueueCount)
		for _, queue := range saved.UnreferencedQueues {
			writeLocationQueue(&b, queue, saved.PlanetID, ctx, "  ")
		}
		if saved.UnreferencedQueueCount > len(saved.UnreferencedQueues) {
			fmt.Fprintf(&b, "  first %d shown\n", len(saved.UnreferencedQueues))
		}
		if saved.OtherTypeQueueCount > 0 {
			fmt.Fprintf(&b, "  Queues with the same numeric location.id but another location.type (likely another ID namespace; not listed): %d\n", saved.OtherTypeQueueCount)
		}
	}
	if saved.ItemLocationID != nil && saved.ItemTableIssue == "" && saved.QueueTableIssue == "" {
		fmt.Fprintf(&b, "\nItems outside the queues above whose buildable_*.planet equals %s %d: %d\n", saved.ItemLocationTable, *saved.ItemLocationID, saved.UnreferencedItemCount)
		for _, item := range saved.UnreferencedItems {
			writeLocationItem(&b, item, nil, ctx, "  ")
		}
		if saved.UnreferencedItemCount > len(saved.UnreferencedItems) {
			fmt.Fprintf(&b, "  first %d shown\n", len(saved.UnreferencedItems))
		}
	}
	return mcp.NewToolResultText(b.String())
}

func writeLocationQueue(b *strings.Builder, queue gamestate.LocationQueue, planetID *int, ctx locationItemContext, indent string) {
	fmt.Fprintf(b, "%sQueue %d: %s\n", indent, queue.ID, locationRecordStatus(queue.Record))
	if queue.Record.Issue != "" {
		return
	}
	fmt.Fprintf(b, "%s  type=%s owner=%s location.type=%s location.id=%s simultaneous=%s disabled=%s\n", indent,
		optionalStringText(queue.Type), optionalIntRefText(queue.Owner), optionalIntRefText(queue.LocType), optionalIntRefText(queue.LocID),
		optionalStringText(queue.Simultaneous), optionalStringText(queue.Disabled))
	if planetID != nil {
		if queue.LocID.Value != nil && *queue.LocID.Value == *planetID {
			fmt.Fprintf(b, "%s  location.id equals planet %d\n", indent, *planetID)
		} else {
			fmt.Fprintf(b, "%s  location.id does not equal planet %d\n", indent, *planetID)
		}
	}
	switch {
	case !queue.ItemsRecorded:
		fmt.Fprintf(b, "%s  items not recorded\n", indent)
	case queue.ItemsUnsupported:
		fmt.Fprintf(b, "%s  items unsupported or duplicate; use raw tree/byte tools\n", indent)
	case len(queue.Items) == 0:
		fmt.Fprintf(b, "%s  items: empty\n", indent)
	}
	for position, item := range queue.Items {
		writeLocationItem(b, item, &queue.ID, ctx, fmt.Sprintf("%s  items[%d] ", indent, position))
	}
}

type locationItemContext struct {
	table string
	id    *int
}

func writeLocationItem(b *strings.Builder, item gamestate.LocationItem, queueID *int, ctx locationItemContext, prefix string) {
	fmt.Fprintf(b, "%sItem %d: %s\n", prefix, item.ID, locationRecordStatus(item.Record))
	if item.Record.Issue != "" {
		return
	}
	indent := strings.Repeat(" ", len(prefix)-len(strings.TrimLeft(prefix, " "))+4)
	back := ""
	if queueID != nil {
		switch {
		case item.Queue.Value != nil && *item.Queue.Value == *queueID:
			back = " (item.queue back-reference agrees)"
		default:
			back = " (item.queue back-reference differs or is unavailable)"
		}
	}
	fmt.Fprintf(b, "%sitem.queue=%s%s paying_country=%s progress=%s progress_needed=%s (saved values)\n", indent,
		optionalIntRefText(item.Queue), back, optionalIntRefText(item.PayingCountry), optionalStringText(item.Progress), optionalStringText(item.Needed))
	if len(item.Buildables) == 0 {
		fmt.Fprintf(b, "%sNo buildable_* fields\n", indent)
	}
	for _, pair := range item.Buildables {
		fmt.Fprintf(b, "%s%s:%s", indent, pair.Key, inlineSavedFields(pair.Value))
		if planet, recorded, unsupported := savedChild(pair.Value, "planet"); recorded && ctx.id != nil {
			switch {
			case unsupported:
				fmt.Fprint(b, " (planet field unsupported or duplicate)")
			case planet == fmt.Sprint(*ctx.id):
				fmt.Fprintf(b, " (planet field equals %s %d)", ctx.table, *ctx.id)
			default:
				fmt.Fprintf(b, " (planet field does not equal %s %d)", ctx.table, *ctx.id)
			}
		}
		fmt.Fprintln(b)
	}
}

// inlineSavedFields renders one level of a saved block on a single line.
func inlineSavedFields(value clausewitz.Value) string {
	switch {
	case value.Mixed:
		return " mixed saved block; use raw tools"
	case value.IsScalar():
		return fmt.Sprintf(" %q (saved scalar)", value.Scalar)
	case value.IsList():
		return fmt.Sprintf(" list with %d saved entries; use raw tools", len(value.List))
	case len(value.Object.Pairs) == 0:
		return " empty block"
	}
	var parts []string
	seen := map[string]bool{}
	for _, pair := range value.Object.Pairs {
		key := pair.Key
		if seen[key] {
			key += "[duplicate]"
		}
		seen[pair.Key] = true
		switch {
		case pair.Value.Mixed:
			parts = append(parts, key+"=mixed")
		case pair.Value.IsScalar():
			parts = append(parts, fmt.Sprintf("%s=%q", key, pair.Value.Scalar))
		case pair.Value.IsObject():
			parts = append(parts, fmt.Sprintf("%s=block(%d fields)", key, len(pair.Value.Object.Pairs)))
		default:
			parts = append(parts, fmt.Sprintf("%s=list(%d)", key, len(pair.Value.List)))
		}
	}
	return " " + strings.Join(parts, " ")
}

// savedChild returns a unique scalar child; a repeated or non-scalar child is unsupported.
func savedChild(value clausewitz.Value, key string) (string, bool, bool) {
	if !value.IsObject() {
		return "", false, false
	}
	found, text, scalar := 0, "", true
	for _, pair := range value.Object.Pairs {
		if pair.Key == key {
			found++
			text, scalar = pair.Value.Scalar, pair.Value.IsScalar()
		}
	}
	return text, found > 0, found > 1 || !scalar
}

func locationRecordStatus(record gamestate.LoadoutRecord) string {
	if record.Issue != "" {
		return record.Issue
	}
	return "unique saved record"
}

func locationReciprocity(saved *gamestate.LocationConstruction) string {
	planetSide := saved.PlanetColony.Value != nil && saved.ColonyID != nil && *saved.PlanetColony.Value == *saved.ColonyID
	colonySide := saved.CarrierRef.Value != nil && *saved.CarrierRef.Value == *saved.PlanetID && saved.CarrierType.Value == "planet" && !saved.CarrierType.Unsupported
	switch {
	case planetSide && colonySide:
		return "both saved sides agree"
	case planetSide:
		return "only planet.colony names this colony"
	case colonySide:
		return "only colony.carrier names this planet"
	default:
		return "neither side names the other"
	}
}

func optionalStringText(value gamestate.OptionalString) string {
	switch {
	case !value.Recorded:
		return "not recorded"
	case value.Unsupported:
		return "unsupported or duplicate"
	default:
		return fmt.Sprintf("%q", value.Value)
	}
}
