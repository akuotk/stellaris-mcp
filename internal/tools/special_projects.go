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
		mcp.WithDescription("Read repeated special_project records from the selected country's saved events and ID candidates in its research queues. Shows raw status, days_left and other saved fields. Availability, completion, timing units and scope or location namespaces remain unverified. Deeper and mixed payloads are available through raw tree/byte tools."),
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
	fmt.Fprintln(&b, "Scope follows the selected country's saved events and research queues. Current availability, completion, timing units and location namespaces remain unverified.")
	fmt.Fprintf(&b, "last_completed_special_project=%s (saved field)\n", councilSavedString(country.LastCompletedSpecialProject))
	projects := country.SpecialProjects
	switch {
	case projects.Unsupported:
		fmt.Fprintln(&b, "country.events unsupported or duplicate. Use raw tree/byte tools.")
	case !projects.Recorded:
		fmt.Fprintln(&b, "country.events not recorded")
	case len(projects.Items) == 0:
		fmt.Fprintln(&b, "No saved country.events.special_project records")
	}
	counts := map[int]int{}
	for _, project := range projects.Items {
		if project.ID.Value != nil {
			counts[*project.ID.Value]++
		}
	}
	for index, project := range projects.Items {
		label := fmt.Sprintf("\ncountry.events.special_project[%d] (saved order)", index)
		writeSavedValue(&b, label, project.Value, 0)
		if project.Unsupported {
			fmt.Fprintln(&b, "  Unsupported project record shape. Use raw tree/byte tools.")
			continue
		}
		switch {
		case project.ID.Unsupported:
			fmt.Fprintln(&b, "  Project ID unsupported or duplicate. Research candidate unresolved.")
		case project.ID.Value == nil:
			fmt.Fprintln(&b, "  Project ID not recorded")
		case counts[*project.ID.Value] > 1:
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
			if ref.Value == nil {
				continue
			}
			switch count := counts[*ref.Value]; count {
			case 0:
				fmt.Fprintln(&b, "    No decoded same-country project ID candidate")
			case 1:
				fmt.Fprintln(&b, "    One decoded same-country project ID candidate")
			default:
				fmt.Fprintf(&b, "    Ambiguous ID with %d decoded same-country project candidates\n", count)
			}
		}
	}
	if !seen {
		fmt.Fprintln(&b, "  No saved research queue special_project references")
	}
	return mcp.NewToolResultText(b.String())
}
