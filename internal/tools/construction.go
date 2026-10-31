package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerConstruction(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_construction", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Read saved construction queues/items matching queue owner or paying_country. Preserves queue order, shows orphan/back-reference mismatches and raw progress/resources. Numeric location types, progress units and completion dates are not inferred."), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player"))), handleConstruction)
}

func handleConstruction(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderConstruction(gs, id, country), nil
}

func renderConstruction(gs *gamestate.GameState, countryID int, country *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Construction (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope matches saved queue owner or item paying_country. Progress/resources and numeric location types are raw; ETA and completion percentages are not inferred.")
	if gs.Construction.DecodeError != "" {
		fmt.Fprintln(&b, "Construction decoding incomplete: unsupported saved shape; decoded records below may be partial. Use raw tree/byte tools for complete construction data.")
	}
	queues, items := gs.Construction.QueueManager.Queues, gs.Construction.ItemManager.Items
	selected := map[int]bool{}
	for id, q := range queues {
		if q.Owner != nil && *q.Owner == countryID {
			selected[id] = true
		}
	}
	for _, item := range items {
		if item.PayingCountry != nil && *item.PayingCountry == countryID && item.Queue != nil {
			if _, ok := queues[*item.Queue]; ok {
				selected[*item.Queue] = true
			}
		}
	}
	queueIDs := make([]int, 0, len(selected))
	for id := range selected {
		queueIDs = append(queueIDs, id)
	}
	sort.Ints(queueIDs)
	shown := map[int]bool{}
	for _, queueID := range queueIDs {
		q := queues[queueID]
		fmt.Fprintf(&b, "\nQueue %d: type=%s owner=%s simultaneous=%s", queueID, q.Type, constructionInt(q.Owner), constructionFloat(q.Simultaneous))
		if q.Disabled == nil {
			fmt.Fprint(&b, " disabled=not recorded")
		} else {
			fmt.Fprintf(&b, " disabled=%t", *q.Disabled)
		}
		fmt.Fprintf(&b, "\n  Location (raw): type=%s id=%s\n", constructionInt(q.Location.Type), constructionInt(q.Location.ID))
		if q.Items == nil {
			fmt.Fprintln(&b, "  Item order: not recorded")
		} else if len(q.Items) == 0 {
			fmt.Fprintln(&b, "  Item order: empty")
		}
		seen := map[int]bool{}
		for position, itemID := range q.Items {
			fmt.Fprintf(&b, "  Position %d, item %d", position, itemID)
			if seen[itemID] {
				fmt.Fprint(&b, " [duplicate queue reference]")
			}
			seen[itemID] = true
			fmt.Fprintln(&b)
			item, ok := items[itemID]
			if !ok {
				fmt.Fprintln(&b, "    referenced item unavailable")
				continue
			}
			shown[itemID] = true
			if item.Queue == nil {
				fmt.Fprintln(&b, "    back-reference: queue not recorded")
			} else if *item.Queue != queueID {
				fmt.Fprintf(&b, "    back-reference mismatch: item.queue=%d\n", *item.Queue)
			} else {
				fmt.Fprintln(&b, "    queue/item back-reference validated")
			}
			writeConstructionItem(&b, gs, item)
		}
		// A back-reference without membership must remain visible instead of disappearing.
		backRefs := []int{}
		for id, item := range items {
			if item.Queue != nil && *item.Queue == queueID && !seen[id] {
				backRefs = append(backRefs, id)
			}
		}
		sort.Ints(backRefs)
		for _, id := range backRefs {
			shown[id] = true
			fmt.Fprintf(&b, "  Item %d: item.queue points here but queue.items does not contain it\n", id)
			writeConstructionItem(&b, gs, items[id])
		}
	}
	orphanIDs := []int{}
	for id, item := range items {
		if item.PayingCountry != nil && *item.PayingCountry == countryID && !shown[id] {
			orphanIDs = append(orphanIDs, id)
		}
	}
	sort.Ints(orphanIDs)
	for _, id := range orphanIDs {
		item := items[id]
		fmt.Fprintf(&b, "\nUnresolved item %d: queue=%s", id, constructionInt(item.Queue))
		if item.Queue == nil {
			fmt.Fprintln(&b, " (queue reference not recorded)")
		} else {
			fmt.Fprintln(&b, " (referenced queue unavailable)")
		}
		writeConstructionItem(&b, gs, item)
	}
	if len(queueIDs) == 0 && len(orphanIDs) == 0 {
		fmt.Fprintln(&b, "No matching decoded construction records.")
	}
	return mcp.NewToolResultText(b.String())
}

func writeConstructionItem(b *strings.Builder, gs *gamestate.GameState, item gamestate.ConstructionItem) {
	fmt.Fprintf(b, "    paying_country=%s progress=%s progress_needed=%s\n", constructionInt(item.PayingCountry), constructionFloat(item.Progress), constructionFloat(item.ProgressNeeded))
	if item.Resources == nil {
		fmt.Fprintln(b, "    resources: not recorded")
	} else {
		fmt.Fprintf(b, "    resources (as saved): %s\n", colonyResources(item.Resources))
	}
	buildables := []struct {
		name  string
		value *gamestate.ConstructionBuildable
	}{{"district", item.District}, {"planet_building", item.Building}, {"planet_upgrade_building", item.UpgradeBuilding}, {"planet_replace_building", item.ReplaceBuilding}, {"clear_deposit_blocker", item.ClearBlocker}, {"army", item.Army}, {"ship", item.Ship}, {"colony_ship", item.ColonyShip}, {"starbase_module", item.StarbaseModule}, {"starbase_building", item.StarbaseBuilding}, {"starbase_upgrade", item.StarbaseUpgrade}}
	count := 0
	for _, entry := range buildables {
		v := entry.value
		if v == nil {
			continue
		}
		count++
		fmt.Fprintf(b, "    Buildable: %s\n", entry.name)
		for _, field := range []struct{ name, value string }{{"district", v.District}, {"building", v.Building}, {"upgrade_building", v.UpgradeBuilding}, {"replace_building", v.ReplaceBuilding}, {"army_type", v.ArmyType}, {"starbase_module", v.StarbaseModule}, {"starbase_building", v.StarbaseBuilding}, {"starbase_upgrade", v.StarbaseUpgrade}} {
			if field.value != "" {
				fmt.Fprintf(b, "      %s=%s\n", field.name, field.value)
			}
		}
		for _, field := range []struct {
			name  string
			value *int
		}{{"planet (saved reference)", v.Planet}, {"zone", v.Zone}, {"deposit", v.Deposit}, {"species", v.Species}, {"starbase", v.Starbase}, {"slot", v.Slot}, {"orbitable.starbase", v.Orbitable.Starbase}, {"colonization_data.species", v.ColonizationData.Species}, {"ship_design.design", v.ShipDesign.Design}, {"ship_design.upgrade", v.ShipDesign.Upgrade}, {"ship_design.growth_stage", v.ShipDesign.GrowthStage}} {
			if field.value != nil {
				fmt.Fprintf(b, "      %s=%d\n", field.name, *field.value)
			}
		}
		if v.Planet != nil {
			if colony, ok := gs.Colonies[*v.Planet]; ok {
				fmt.Fprintf(b, "      Possible colony ID match (not proven by this item alone): %d; %s\n", *v.Planet, colonyCarrier(gs, *v.Planet, colony))
			} else {
				fmt.Fprintln(b, "      Colony identity unavailable; planet field remains raw (legacy semantics may differ)")
			}
		}
		if v.ShipDesign.Design != nil {
			if design, ok := gs.ShipDesign[*v.ShipDesign.Design]; ok {
				fmt.Fprintf(b, "      Ship design: %s\n", name(design.Name))
			} else {
				fmt.Fprintln(b, "      Ship design record unavailable")
			}
		}
	}
	if count == 0 {
		fmt.Fprintln(b, "    No recognized buildable payload; use raw tree tools for other saved fields")
	}
	if count > 1 {
		fmt.Fprintln(b, "    Multiple buildable payloads recorded; interpretation not inferred")
	}
}

func constructionInt(value *int) string {
	if value == nil {
		return "not recorded"
	}
	return fmt.Sprint(*value)
}
func constructionFloat(value *float64) string {
	if value == nil {
		return "not recorded"
	}
	return fmt.Sprintf("%g", *value)
}
