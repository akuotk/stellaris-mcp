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

func registerDebris(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_debris", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read save-wide debris records and country.events.special_project.debris ID references. debris_id shows ordered raw fields, including coordinates, components, ship sizes, resources and flags. Numeric country equality and project references describe saved values. Ownership, visibility, availability and location semantics remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("debris_id", mcp.Description("Saved debris ID for raw details; omit for summary"))), handleDebris)
}

func handleDebris(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var selected *int
	if _, ok := req.GetArguments()["debris_id"]; ok {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			maxExact := int64(1<<53 - 1)
			maxID = int(maxExact)
		}
		id, err := rawInteger(req, "debris_id", 0, -maxID, maxID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &id
	}
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	return renderDebris(gs, selected), nil
}

type debrisProjectReference struct {
	countryID int
	position  int
	projectID gamestate.OptionalIntRef
}

func renderDebris(gs *gamestate.GameState, selected *int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintln(&b, "=== Saved debris and project references (save-wide) ===")
	fmt.Fprintln(&b, "Project references compare saved debris IDs. The meanings of country, from_country, coordinates and flags, and current ownership or availability remain unverified.")
	table := gs.Debris
	switch {
	case table.Unsupported:
		fmt.Fprintln(&b, "debris table unsupported or duplicate. Use raw tree/byte tools.")
	case !table.Recorded:
		fmt.Fprintln(&b, "debris table not recorded")
	case table.InvalidKeys > 0:
		fmt.Fprintf(&b, "debris table contains %d nonnumeric keys. Use raw tree/byte tools.\n", table.InvalidKeys)
	}
	refs, unresolved := debrisProjectReferences(gs)
	if unresolved > 0 {
		fmt.Fprintf(&b, "Project reference coverage incomplete for %d unsupported or duplicate containers or records. Use raw tree/byte tools.\n", unresolved)
	}
	ids := make([]int, 0, len(table.Items)+len(refs))
	if selected != nil {
		ids = append(ids, *selected)
	} else {
		for id, entries := range table.Items {
			if len(entries) != 1 || !entries[0].IsSavedNone() || len(refs[id]) > 0 {
				ids = append(ids, id)
			}
		}
		for id := range refs {
			if _, ok := table.Items[id]; !ok {
				ids = append(ids, id)
			}
		}
		sort.Ints(ids)
	}
	if len(ids) == 0 {
		fmt.Fprintln(&b, "No saved debris records or decoded project debris references")
	}
	for _, id := range ids {
		record, issue := debrisCandidate(table, id)
		if issue != "" {
			fmt.Fprintf(&b, "\nDebris %d %s\n", id, issue)
		} else {
			fmt.Fprintf(&b, "\nDebris %d debris.country=%s (saved field)\n", id, debrisRefText(record.Country))
		}
		if len(refs[id]) == 0 {
			fmt.Fprintln(&b, "  No decoded saved project debris references")
		}
		for _, ref := range refs[id] {
			fmt.Fprintf(&b, "  Country %d country.events.special_project[%d] id=%s debris=%d (saved reference)\n", ref.countryID, ref.position, debrisRefText(ref.projectID), id)
			if issue == "" && record.Country.Value != nil {
				fmt.Fprintf(&b, "    debris.country numeric ID %s the project country ID\n", debrisCountryComparison(*record.Country.Value, ref.countryID))
			}
		}
		if selected != nil && !table.Unsupported {
			for index, entry := range table.Items[id] {
				writeSavedValue(&b, fmt.Sprintf("Saved debris entry %d", index), entry.Value, 0)
			}
		}
	}
	return mcp.NewToolResultText(b.String())
}

func debrisProjectReferences(gs *gamestate.GameState) (map[int][]debrisProjectReference, int) {
	refs := map[int][]debrisProjectReference{}
	var unresolved int
	countryIDs := make([]int, 0, len(gs.Country))
	for id := range gs.Country {
		countryIDs = append(countryIDs, id)
	}
	sort.Ints(countryIDs)
	for _, countryID := range countryIDs {
		projects := gs.Country[countryID].SpecialProjects
		if projects.Unsupported {
			unresolved++
		}
		for position, project := range projects.Items {
			if project.Unsupported || project.Debris.Unsupported {
				unresolved++
			} else if project.Debris.Value != nil {
				id := *project.Debris.Value
				refs[id] = append(refs[id], debrisProjectReference{countryID, position, project.ID})
			}
		}
	}
	return refs, unresolved
}

func debrisCandidate(table gamestate.DebrisTable, id int) (gamestate.SavedDebris, string) {
	entries := table.Items[id]
	switch {
	case table.Unsupported:
		return gamestate.SavedDebris{}, "table unsupported or duplicate"
	case !table.Recorded:
		return gamestate.SavedDebris{}, "table not recorded"
	case len(entries) == 0:
		return gamestate.SavedDebris{}, "record unavailable"
	case len(entries) > 1:
		return gamestate.SavedDebris{}, fmt.Sprintf("has a duplicate saved ID (%d entries)", len(entries))
	case entries[0].IsSavedNone():
		return gamestate.SavedDebris{}, `saved entry is "none"`
	case !entries[0].Value.IsObject():
		return gamestate.SavedDebris{}, "record shape unsupported. Use raw tree/byte tools."
	default:
		return entries[0], ""
	}
}

func debrisRefText(ref gamestate.OptionalIntRef) string {
	if ref.Unsupported {
		return "unsupported or duplicate"
	}
	return armyInt(ref.Value)
}

func debrisCountryComparison(country, projectCountry int) string {
	if country == projectCountry {
		return "agrees with"
	}
	return "differs from"
}
