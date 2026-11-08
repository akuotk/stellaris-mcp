package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSpecialProjects(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_special_projects", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read repeated special_project records from the selected country's saved events and ID candidates in its research queues and fleet current orders. Shows raw status, days_left and other saved fields, including debris ID candidates. Availability, completion, timing units and scope or location namespaces remain unverified. Deeper and mixed payloads are available through raw tree/byte tools."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player"))), handleSpecialProjects)
}

func handleSpecialProjects(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderSpecialProjects(gs, id, country), nil
}

func renderSpecialProjects(gs *gamestate.GameState, countryID int, country *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved special projects (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope follows the selected country's saved events, research queues and owned_fleets. Current availability, completion, timing units and location namespaces remain unverified.")
	fmt.Fprintf(&b, "last_completed_special_project=%s (saved field)\n", councilSavedString(country.LastCompletedSpecialProject))
	rootCounts := indexRootEventTargets(&b, gs.RootEventTargets)
	projects := country.SpecialProjects
	switch {
	case projects.Unsupported:
		fmt.Fprintln(&b, "country.events unsupported or duplicate. Use raw tree/byte tools.")
	case !projects.Recorded:
		fmt.Fprintln(&b, "country.events not recorded")
	case len(projects.Items) == 0:
		fmt.Fprintln(&b, "No saved country.events.special_project records")
	}
	positions := map[int][]int{}
	for index, project := range projects.Items {
		if project.ID.Value != nil {
			id := *project.ID.Value
			positions[id] = append(positions[id], index)
		}
	}
	for index, project := range projects.Items {
		label := fmt.Sprintf("\ncountry.events.special_project[%d] (saved order)", index)
		writeSavedValue(&b, label, project.Value, 0)
		writeNestedEventTargets(&b, project.Value, rootCounts)
		if project.Unsupported {
			fmt.Fprintln(&b, "  Unsupported project record shape. Use raw tree/byte tools.")
			continue
		}
		switch {
		case project.ID.Unsupported:
			fmt.Fprintln(&b, "  Project ID unsupported or duplicate. Research candidate unresolved.")
		case project.ID.Value == nil:
			fmt.Fprintln(&b, "  Project ID not recorded")
		case len(positions[*project.ID.Value]) > 1:
			fmt.Fprintln(&b, "  Multiple saved project records share this ID")
		}
		if project.Planet.Unsupported {
			fmt.Fprintln(&b, "  Planet field unsupported or duplicate. Candidate unresolved.")
		} else if project.Planet.Value != nil {
			id := *project.Planet.Value
			if planet, ok := gs.Planets.Planet[id]; ok {
				fmt.Fprintf(&b, "  Saved planet ID candidate %d %s\n", id, name(planet.Name))
			} else {
				fmt.Fprintf(&b, "  Saved planet ID candidate %d unavailable\n", id)
			}
		}
		writeProjectDebrisCandidate(&b, gs.Debris, project.Debris, countryID)
	}
	fmt.Fprintln(&b, "\nResearch queue special_project references (saved positions)")
	seen := false
	for _, queue := range []struct {
		name  string
		items []gamestate.ResearchItem
	}{
		{"physics_queue", country.TechStatus.PhysicsQueue},
		{"society_queue", country.TechStatus.SocietyQueue},
		{"engineering_queue", country.TechStatus.EngineeringQueue},
	} {
		for index, item := range queue.items {
			ref := item.SpecialProjectRef
			if !ref.Recorded {
				continue
			}
			seen = true
			if ref.Unsupported {
				fmt.Fprintf(&b, "  %s[%d].special_project unsupported or duplicate. Use raw tools.\n", queue.name, index)
				continue
			}
			fmt.Fprintf(&b, "  %s[%d] special_project=%s progress=%s date=%s (saved values)\n", queue.name, index, armyInt(ref.Value), armyFloat(item.ProgressRaw), governanceValue(item.DateRaw))
			if ref.Value != nil {
				writeSpecialProjectCandidates(&b, positions[*ref.Value])
			}
		}
	}
	if !seen {
		fmt.Fprintln(&b, "  No saved research queue special_project references")
	}
	writeFleetSpecialProjectRefs(&b, gs, country, positions)
	return mcp.NewToolResultText(b.String())
}

func writeProjectDebrisCandidate(b *strings.Builder, table gamestate.DebrisTable, ref gamestate.OptionalIntRef, countryID int) {
	switch {
	case ref.Unsupported:
		fmt.Fprintln(b, "  Debris field unsupported or duplicate. Candidate unresolved. Use raw tree/byte tools.")
	case ref.Value == nil:
		fmt.Fprintln(b, "  Debris reference not recorded")
	default:
		id := *ref.Value
		record, issue := debrisCandidate(table, id)
		if issue != "" {
			fmt.Fprintf(b, "  Saved debris reference %d unresolved (%s)\n", id, issue)
			return
		}
		fmt.Fprintf(b, "  Saved debris ID candidate %d debris.country=%s (use get_debris for saved fields)\n", id, debrisRefText(record.Country))
		if record.Country.Value != nil {
			fmt.Fprintf(b, "    debris.country numeric ID %s the project country ID\n", debrisCountryComparison(*record.Country.Value, countryID))
		}
	}
}

func writeSpecialProjectCandidates(b *strings.Builder, positions []int) {
	switch len(positions) {
	case 0:
		fmt.Fprintln(b, "    No decoded same-country project ID candidate")
	case 1:
		fmt.Fprintf(b, "    One decoded same-country project ID candidate at country.events.special_project[%d]\n", positions[0])
	default:
		fmt.Fprintf(b, "    Ambiguous ID with %d decoded same-country project candidates at saved positions %v\n", len(positions), positions)
	}
}

func writeFleetSpecialProjectRefs(b *strings.Builder, gs *gamestate.GameState, country *gamestate.Country, positions map[int][]int) {
	fmt.Fprintln(b, "\nFleet current_order special_project references (saved owned_fleets order)")
	manager := country.FleetsManager
	if manager.OwnershipAmbiguous {
		fmt.Fprintln(b, "  Country owned_fleets entries or container are duplicate. Candidate scope unresolved. Use raw tree/byte tools.")
		return
	}
	memberships := map[int]int{}
	for _, c := range gs.Country {
		for _, entry := range c.FleetsManager.OwnedFleets {
			if entry.FleetID != nil {
				memberships[*entry.FleetID]++
			}
		}
	}
	seen := map[int]bool{}
	var missing, unresolved int
	wrote := false
	for _, entry := range manager.OwnedFleets {
		if entry.FleetID == nil {
			missing++
			continue
		}
		id := *entry.FleetID
		if seen[id] {
			continue
		}
		seen[id] = true
		fleet, ok := gs.Fleet[id]
		if !ok {
			unresolved++
			continue
		}
		orders := fleet.CurrentOrderRefs
		if orders.Unsupported {
			fmt.Fprintf(b, "  Fleet %d current_order unsupported or duplicate. Use raw tree/byte tools.\n", id)
			wrote = true
			continue
		}
		for _, order := range []struct {
			path string
			ref  gamestate.OptionalIntRef
		}{
			{"current_order.collect_data_fleet_order.special_project", orders.SpecialProject},
			{"current_order.automate_fleet_order.sub_order.collect_data_fleet_order.special_project", orders.AutomatedSpecialProject},
		} {
			if !order.ref.Recorded {
				continue
			}
			wrote = true
			if order.ref.Unsupported || order.ref.Value == nil {
				fmt.Fprintf(b, "  Fleet %d %s unsupported or duplicate. Use raw tree/byte tools.\n", id, order.path)
				continue
			}
			fmt.Fprintf(b, "  Fleet %d (%s) %s=%d (saved reference)\n", id, name(fleet.Name), order.path, *order.ref.Value)
			if memberships[id] > 1 {
				fmt.Fprintf(b, "    Ambiguous fleet scope with %d saved owned_fleets references. Project candidate unresolved.\n", memberships[id])
				continue
			}
			writeSpecialProjectCandidates(b, positions[*order.ref.Value])
		}
	}
	if !wrote {
		fmt.Fprintln(b, "  No decoded fleet current_order special_project references in the selected scope")
	}
	if missing > 0 || unresolved > 0 {
		fmt.Fprintf(b, "  Owned fleet entries without IDs=%d; referenced fleet records unavailable=%d\n", missing, unresolved)
	}
}
