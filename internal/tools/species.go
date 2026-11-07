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

func registerSpecies(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_species", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save")), mcp.WithNumber("species_id", mcp.Description("Optional species database ID for a specific record")), mcp.WithDescription("Show saved species names, classes, traits and referenced population groups on the selected country's planets; population is in raw save units")), handleSpecies)
}

func handleSpecies(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["species_id"]; ok {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			exactMax := int64(1<<53 - 1)
			maxID = int(exactMax)
		}
		v, err := rawInteger(req, "species_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &v
	}
	return renderSpecies(gs, id, c, selected), nil
}

func renderSpecies(gs *gamestate.GameState, countryID int, c *gamestate.Country, selected *int) *mcp.CallToolResult {
	type summary struct {
		groups, missingSize, rawPopulation, ambiguousGroups int
		planets                                             map[int]bool
	}
	all := map[int]*summary{}
	groupPlanets := map[int]map[int]bool{}
	missingGroups, missingSpecies, duplicateRefs, ambiguousGroups := 0, 0, 0, 0
	for planetID, p := range gs.Planets.Planet {
		if p.OwnerID == nil || *p.OwnerID != countryID {
			continue
		}
		for _, groupID := range p.PopGroups {
			if groupPlanets[groupID] == nil {
				groupPlanets[groupID] = map[int]bool{}
			} else {
				duplicateRefs++
			}
			groupPlanets[groupID][planetID] = true
		}
	}
	for groupID, planets := range groupPlanets {
		g, ok := gs.PopGroups[groupID]
		if !ok {
			missingGroups++
			continue
		}
		if len(planets) > 1 {
			ambiguousGroups++
		}
		if g.Key.SpeciesID == nil {
			missingSpecies++
			continue
		}
		sid := *g.Key.SpeciesID
		item := all[sid]
		if item == nil {
			item = &summary{planets: map[int]bool{}}
			all[sid] = item
		}
		item.groups++
		for planetID := range planets {
			item.planets[planetID] = true
		}
		if len(planets) > 1 {
			item.ambiguousGroups++
		}
		if g.SizeRaw == nil {
			item.missingSize++
		} else {
			item.rawPopulation += *g.SizeRaw
		}
	}
	var ids []int
	if selected != nil {
		ids = []int{*selected}
	} else {
		for id := range all {
			ids = append(ids, id)
		}
		sort.Ints(ids)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Species records (%s) ===\n", name(c.Adjective))
	if selected == nil && len(ids) == 0 {
		fmt.Fprintln(&b, "No species references found in owned planet population groups.")
	}
	for _, id := range ids {
		s, ok := gs.Species[id]
		if !ok {
			fmt.Fprintf(&b, "\nSpecies ID %d: record unavailable\n", id)
			continue
		}
		fmt.Fprintf(&b, "\nSpecies ID %d: %s\n  Class: %s; portrait: %s\n", id, name(s.Name), s.Class, s.Portrait)
		if len(s.Traits.Trait) > 0 {
			for _, trait := range s.Traits.Trait {
				fmt.Fprintf(&b, "  Trait: %s [%s]\n", l(trait), trait)
			}
		} else {
			fmt.Fprintln(&b, "  Traits: none recorded")
		}
		if v := all[id]; v != nil {
			fmt.Fprintf(&b, "  Owned planets referencing groups: %d; groups: %d; population sum (raw save units): %d", len(v.planets), v.groups, v.rawPopulation)
			if v.missingSize > 0 {
				fmt.Fprintf(&b, " (incomplete: %d group sizes missing)", v.missingSize)
			}
			if v.ambiguousGroups > 0 {
				fmt.Fprintf(&b, " (location ambiguous for %d groups)", v.ambiguousGroups)
			}
			fmt.Fprintln(&b)
		} else if selected != nil {
			fmt.Fprintln(&b, "  No matching population groups found on selected country's planets.")
		}
	}
	if missingGroups > 0 || missingSpecies > 0 || duplicateRefs > 0 || ambiguousGroups > 0 {
		fmt.Fprintf(&b, "\nIncomplete references: missing groups=%d, groups without species ID=%d, duplicate group references=%d, groups referenced by multiple owned planets=%d.\n", missingGroups, missingSpecies, duplicateRefs, ambiguousGroups)
	}
	fmt.Fprintln(&b, "Population values are saved raw group sizes, not validated game UI POP counts. Each group is counted once; multiple planet references do not establish a location. Non-planet carriers are not included.")
	return mcp.NewToolResultText(b.String())
}
