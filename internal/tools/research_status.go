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

func registerResearchStatus(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_research_status", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Get current research, available options, and completed technologies"),
			mcp.WithNumber("country_id", mcp.Description("Country ID. If omitted, all saved player records must refer to one country.")),
		),
		handleResearchStatus,
	)
}

func handleResearchStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	_, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}

	ts := c.TechStatus
	var b strings.Builder
	fmt.Fprintf(&b, "=== Research Status (%s) ===\n", name(c.Adjective))

	fmt.Fprintf(&b, "\nCurrent Research:\n")
	writeQueue(&b, "Physics", ts.PhysicsQueue)
	writeQueue(&b, "Society", ts.SocietyQueue)
	writeQueue(&b, "Engineering", ts.EngineeringQueue)

	fmt.Fprintf(&b, "\nAvailable Research Options:\n")
	writeAlternatives(&b, "Physics", ts.Alternatives.Physics)
	writeAlternatives(&b, "Society", ts.Alternatives.Society)
	writeAlternatives(&b, "Engineering", ts.Alternatives.Engineering)

	fmt.Fprintf(&b, "\nCompleted Technologies (%d):\n", len(ts.Technology))
	for i, t := range ts.Technology {
		level := "level not recorded or not paired"
		if i < len(ts.CompletedLevels) && ts.CompletedLevels[i] != nil {
			level = fmt.Sprintf("level %d", *ts.CompletedLevels[i])
		}
		fmt.Fprintf(&b, "  - %s [%s] (%s)\n", l(t), t, level)
	}
	if len(ts.StoredPoints) > 0 {
		fmt.Fprintf(&b, "\nStored research points (raw saved sequence): %v\n", ts.StoredPoints)
	}
	if len(ts.StoredForTech) > 0 {
		fmt.Fprintln(&b, "\nStored points for individual technologies (raw save values):")
		keys := make([]string, 0, len(ts.StoredForTech))
		for k := range ts.StoredForTech {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "  %s [%s]: %g\n", l(key), key, ts.StoredForTech[key])
		}
	}
	if len(ts.AlwaysAvailable) > 0 {
		fmt.Fprintf(&b, "\nAlways-available technology keys (as saved): %s\n", strings.Join(ts.AlwaysAvailable, ", "))
	}
	if ts.LastIncreased != "" {
		fmt.Fprintf(&b, "Last increased technology: %s [%s]\n", l(ts.LastIncreased), ts.LastIncreased)
	}
	for _, entry := range []struct {
		label string
		value *string
	}{{"physics", ts.AutoPhysics}, {"society", ts.AutoSociety}, {"engineering", ts.AutoEngineering}} {
		if entry.value != nil {
			fmt.Fprintf(&b, "Auto research %s (as saved): %s\n", entry.label, *entry.value)
		}
	}
	if ts.PotentialBlocks > 0 {
		fmt.Fprintf(&b, "Research potential records: %d; use get_research_potential for saved keys and raw values.\n", len(ts.Potential))
	}

	return mcp.NewToolResultText(b.String()), nil
}

func writeQueue(b *strings.Builder, label string, queue []gamestate.ResearchItem) {
	if queue == nil {
		fmt.Fprintf(b, "  %s: not recorded\n", label)
		return
	}
	if len(queue) == 0 {
		fmt.Fprintf(b, "  %s: empty\n", label)
		return
	}
	for _, r := range queue {
		technology := "not recorded"
		if r.TechnologyRaw != nil {
			technology = fmt.Sprintf("%s [%s]", l(*r.TechnologyRaw), *r.TechnologyRaw)
		}
		date := "not recorded"
		if r.DateRaw != nil {
			date = strconv.Quote(*r.DateRaw)
		}
		fmt.Fprintf(b, "  %s: %s (saved progress: %s, saved date: %s)\n", label, technology, armyFloat(r.ProgressRaw), date)
		if r.SpecialProjectRef.Unsupported {
			fmt.Fprintln(b, "    special_project reference unsupported or duplicate; use raw tools")
		} else if r.SpecialProjectRef.Recorded {
			fmt.Fprintf(b, "    special_project reference=%s (use get_special_projects for matching records)\n", armyInt(r.SpecialProjectRef.Value))
		}
	}
}

func writeAlternatives(b *strings.Builder, label string, alts []string) {
	if alts == nil {
		fmt.Fprintf(b, "  %s: not recorded\n", label)
		return
	}
	if len(alts) == 0 {
		fmt.Fprintf(b, "  %s: empty\n", label)
		return
	}
	options := make([]string, 0, len(alts))
	for _, key := range alts {
		options = append(options, fmt.Sprintf("%s [%s]", l(key), key))
	}
	fmt.Fprintf(b, "  %s: %s\n", label, strings.Join(options, ", "))
}
