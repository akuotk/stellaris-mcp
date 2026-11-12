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

func registerFleetTemplates(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_fleet_templates", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read fleet_template records referenced by this country's fleet_template_manager. Checks reciprocal fleet references. template_id adds ordered design/count records, home_base and all_queued references. Preserves legacy design and ship_design_implementation forms. Counts, fleet_size and reinforcement meanings, queue-array alignment and completion remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")),
		mcp.WithNumber("template_id", mcp.Description("Referenced template ID for details; omit for summary"))), handleFleetTemplates)
}

func handleFleetTemplates(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["template_id"]; ok {
		templateID, err := rawInteger(req, "template_id", 0, 0, maxRecordID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &templateID
	}
	return renderFleetTemplates(gs, id, country, selected), nil
}

func renderFleetTemplates(gs *gamestate.GameState, countryID int, country *gamestate.Country, selected *int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved fleet templates (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope follows country.fleet_template_manager.fleet_template references. Count, fleet_size and reinforcement meanings remain unverified. Missing counts are not filled in. Use template_id for saved design/queue details.")
	if !gs.FleetTemplates.Recorded {
		fmt.Fprintln(&b, "fleet_template table not recorded")
	}
	if gs.FleetTemplates.TableIssue || len(gs.FleetTemplates.DecodeErrors) > 0 {
		fmt.Fprintln(&b, "Template decoding incomplete; unsupported/duplicate records require raw tree/byte tools")
	}
	manager := country.FleetTemplateRefs
	switch {
	case manager.Unsupported || manager.Templates.Unsupported:
		fmt.Fprintln(&b, "fleet_template_manager references unsupported or duplicate; use raw tools")
		return mcp.NewToolResultText(b.String())
	case !manager.Recorded:
		fmt.Fprintln(&b, "fleet_template_manager not recorded")
	case !manager.Templates.Recorded:
		fmt.Fprintln(&b, "fleet_template_manager.fleet_template not recorded")
	case len(manager.Templates.Values) == 0:
		fmt.Fprintln(&b, "fleet_template_manager.fleet_template empty")
	default:
		fmt.Fprintf(&b, "Country template references (saved order)=%v\n", manager.Templates.Values)
	}
	scoped := map[int]bool{}
	for _, id := range manager.Templates.Values {
		scoped[id] = true
	}
	ids := make([]int, 0, len(scoped))
	for id := range scoped {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if selected != nil {
		if !scoped[*selected] {
			return toolError(fmt.Errorf("template %d is not referenced by selected country's manager", *selected))
		}
		ids = []int{*selected}
	}
	var ownedFleetIDs []int
	if country.FleetsManager.OwnedFleets != nil {
		ownedFleetIDs = []int{}
	}
	missingFleetRefs := 0
	for _, ref := range country.FleetsManager.OwnedFleets {
		if ref.FleetID == nil {
			missingFleetRefs++
		} else {
			ownedFleetIDs = append(ownedFleetIDs, *ref.FleetID)
		}
	}
	if missingFleetRefs > 0 {
		fmt.Fprintf(&b, "Country fleet entries without fleet ID=%d; these are not treated as ID zero\n", missingFleetRefs)
	}
	for _, id := range ids {
		template, ok := gs.FleetTemplates.Items[id]
		if !ok {
			if _, bad := gs.FleetTemplates.DecodeErrors[id]; bad {
				fmt.Fprintf(&b, "\nTemplate %d: referenced record unsupported or duplicate; use raw tools\n", id)
			} else {
				fmt.Fprintf(&b, "\nTemplate %d: referenced record unavailable\n", id)
			}
			continue
		}
		fmt.Fprintf(&b, "\nTemplate %d: country manager membership=%s\n", id, armyMembership(manager.Templates.Values, id))
		fmt.Fprintf(&b, "  fleet=%s fleet_size=%s reinforcement=%s is_edited_by_human=%s (saved values)\n", savedInt(template.Fleet), governanceValue(template.FleetSize), governanceValue(template.Reinforcement), governanceValue(template.EditedByHuman))
		if template.Designs == nil {
			fmt.Fprintln(&b, "  fleet_template_design not recorded")
		} else {
			fmt.Fprintf(&b, "  fleet_template_design entries=%d\n", len(template.Designs))
		}
		if template.Fleet != nil {
			fleetID := *template.Fleet
			fmt.Fprintf(&b, "  Referenced fleet ID=%d; country owned_fleets membership=%s\n", fleetID, armyMembership(ownedFleetIDs, fleetID))
			if fleet, ok := gs.Fleet[fleetID]; ok {
				fmt.Fprintf(&b, "    Fleet record: %s; fleet.fleet_template=", name(fleet.Name))
				switch {
				case fleet.TemplateRef.Unsupported:
					fmt.Fprintln(&b, "unsupported or duplicate; use raw tools")
				case fleet.TemplateRef.Value == nil:
					fmt.Fprintln(&b, "not recorded")
				case *fleet.TemplateRef.Value != id:
					fmt.Fprintf(&b, "%d; template/fleet reference mismatch\n", *fleet.TemplateRef.Value)
				default:
					fmt.Fprintf(&b, "%d; reciprocal template/fleet IDs agree\n", *fleet.TemplateRef.Value)
				}
			} else {
				fmt.Fprintln(&b, "    Referenced fleet record unavailable")
			}
		}
		if selected == nil {
			continue
		}
		writeFleetTemplateDetails(&b, gs, template)
	}
	return mcp.NewToolResultText(b.String())
}

func writeFleetTemplateDetails(b *strings.Builder, gs *gamestate.GameState, template gamestate.FleetTemplate) {
	for index, entry := range template.Designs {
		fmt.Fprintf(b, "  Design entry %d (saved order): design=%s count=%s\n", index, savedInt(entry.Design), governanceValue(entry.Count))
		writeTemplateDesignCandidate(b, gs, entry.Design)
		if entry.ShipDesign == nil {
			fmt.Fprintln(b, "    ship_design_implementation not recorded")
		} else {
			fmt.Fprintf(b, "    ship_design_implementation: design=%s upgrade=%s growth_stage=%s\n", savedInt(entry.ShipDesign.Design), savedInt(entry.ShipDesign.Upgrade), savedInt(entry.ShipDesign.GrowthStage))
			writeTemplateDesignCandidate(b, gs, entry.ShipDesign.Design)
			if entry.Design != nil && entry.ShipDesign.Design != nil && *entry.Design != *entry.ShipDesign.Design {
				fmt.Fprintln(b, "    Legacy and implementation design IDs differ; relationship unverified")
			}
		}
		writeTemplateConstructionItems(b, gs, "design.items", entry.Items)
	}
	if template.HomeBase == nil {
		fmt.Fprintln(b, "  home_base not recorded")
	} else if template.HomeBase.Orbitable == nil {
		fmt.Fprintln(b, "  home_base.orbitable not recorded")
	} else {
		starbaseID := template.HomeBase.Orbitable.Starbase
		fmt.Fprintf(b, "  home_base.orbitable.starbase=%s (saved reference; role unverified)\n", savedInt(starbaseID))
		if starbaseID != nil {
			_, bad := gs.Starbases.DecodeErrors[*starbaseID]
			if starbase, ok := gs.Starbases.Items[*starbaseID]; ok && !bad {
				fmt.Fprintf(b, "    Starbase ID candidate: level=%s type=%s\n", governanceValue(starbase.Level), governanceValue(starbase.Type))
			} else {
				fmt.Fprintln(b, "    Starbase ID candidate unavailable or unsupported")
			}
		}
	}
	if template.AllQueued == nil {
		fmt.Fprintln(b, "  all_queued not recorded")
		return
	}
	fmt.Fprintln(b, "  all_queued (saved values; parallel-list alignment, progress and completion unverified)")
	queued := template.AllQueued
	writeTemplateConstructionItems(b, gs, "all_queued.construction_queue_items", queued.ConstructionItems)
	writeTemplateReferenceList(b, "all_queued.upgrade_designs", queued.UpgradeDesigns)
	for _, id := range queued.UpgradeDesigns.Values {
		id := id
		writeTemplateDesignCandidate(b, gs, &id)
	}
	if queued.Implementations == nil {
		fmt.Fprintln(b, "    all_queued.upgrade_design_implementations not recorded")
	} else if len(queued.Implementations) == 0 {
		fmt.Fprintln(b, "    all_queued.upgrade_design_implementations empty")
	}
	for index, impl := range queued.Implementations {
		fmt.Fprintf(b, "    Upgrade implementation %d (saved order): design=%s upgrade=%s growth_stage=%s\n", index, savedInt(impl.Design), savedInt(impl.Upgrade), savedInt(impl.GrowthStage))
		writeTemplateDesignCandidate(b, gs, impl.Design)
	}
}

func writeTemplateReferenceList(b *strings.Builder, label string, refs gamestate.OptionalIntList) {
	switch {
	case refs.Unsupported:
		fmt.Fprintf(b, "    %s unsupported; use raw tools\n", label)
	case !refs.Recorded:
		fmt.Fprintf(b, "    %s not recorded\n", label)
	case len(refs.Values) == 0:
		fmt.Fprintf(b, "    %s empty\n", label)
	default:
		fmt.Fprintf(b, "    %s (saved order)=%v\n", label, refs.Values)
	}
}

func writeTemplateConstructionItems(b *strings.Builder, gs *gamestate.GameState, label string, refs gamestate.OptionalIntList) {
	writeTemplateReferenceList(b, label, refs)
	if gs.Construction.DecodeError != "" {
		if len(refs.Values) > 0 {
			fmt.Fprintln(b, "    Construction decoding incomplete; item candidates unavailable, use raw tools")
		}
		return
	}
	for index, id := range refs.Values {
		if item, ok := gs.Construction.ItemManager.Items[id]; ok {
			fmt.Fprintf(b, "    %s position %d item ID candidate %d: item.queue=%s paying_country=%s\n", label, index, id, savedInt(item.Queue), savedInt(item.PayingCountry))
		} else {
			fmt.Fprintf(b, "    %s position %d item ID candidate %d unavailable\n", label, index, id)
		}
	}
}

func writeTemplateDesignCandidate(b *strings.Builder, gs *gamestate.GameState, id *int) {
	if id == nil {
		return
	}
	if design, ok := gs.ShipDesign[*id]; ok {
		fmt.Fprintf(b, "    Design ID candidate %d: %s\n", *id, name(design.Name))
	} else {
		fmt.Fprintf(b, "    Design ID candidate %d unavailable\n", *id)
	}
}
