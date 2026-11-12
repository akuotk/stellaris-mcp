package tools

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSpecies(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_species", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save")), mcp.WithNumber("species_id", mcp.Description("Optional species database ID for a specific record")), mcp.WithDescription("Show saved species, country founder/built/owned/enslaved references, base species and home planet fields. Population groups on owned planets use raw save units.")), handleSpecies)
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
		v, err := rawInteger(req, "species_id", 0, 0, maxRecordID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &v
	}
	return renderSpecies(gs, id, c, selected), nil
}

func renderSpecies(gs *gamestate.GameState, countryID int, c *gamestate.Country, selected *int) *mcp.CallToolResult {
	type summary struct {
		groups, missingSize, duplicateSize, unsupportedSize, ambiguousGroups int
		rawPopulation                                                        big.Int
		planets                                                              map[int]bool
	}
	all := map[int]*summary{}
	groupPlanets := map[int]map[int]bool{}
	missingGroups, missingSpecies, duplicateRefs, ambiguousGroups, duplicateGroups := 0, 0, 0, 0, 0
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
		if g.RecordDuplicate {
			duplicateGroups++
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
		switch {
		case len(g.SavedSize.Values) == 0:
			item.missingSize++
		case len(g.SavedSize.Values) > 1:
			item.duplicateSize++
		case !g.SavedSize.Valid:
			item.unsupportedSize++
		default:
			var size big.Int
			item.rawPopulation.Add(&item.rawPopulation, size.SetInt64(g.SavedSize.Integer))
		}
	}
	var ids []int
	if selected != nil {
		ids = []int{*selected}
	} else {
		scoped := map[int]bool{}
		for id := range all {
			scoped[id] = true
		}
		for _, ref := range []gamestate.SpeciesRef{c.FounderSpeciesRef, c.BuiltSpeciesRef} {
			if ref.Value != nil {
				scoped[*ref.Value] = true
			}
		}
		for _, list := range []gamestate.SpeciesRefList{c.OwnedSpeciesRefs, c.EnslavedSpeciesRefs} {
			for _, id := range list.Values {
				scoped[id] = true
			}
		}
		for id := range scoped {
			ids = append(ids, id)
		}
		sort.Ints(ids)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Species records (%s) ===\n", name(c.Adjective))
	fmt.Fprintf(&b, "country.founder_species_ref: %s\ncountry.built_species_ref: %s\n", speciesRefText(gs, c.FounderSpeciesRef), speciesRefText(gs, c.BuiltSpeciesRef))
	writeSpeciesRefList(&b, "country.owned_species_refs", c.OwnedSpeciesRefs)
	writeSpeciesRefList(&b, "country.enslaved_species_refs", c.EnslavedSpeciesRefs)
	if gs.Species.TableIssue || len(gs.Species.DecodeErrors) != 0 {
		fmt.Fprintln(&b, "Species table has incomplete decoding. Use raw tree/byte tools for omitted records.")
	} else if !gs.Species.Recorded {
		fmt.Fprintln(&b, "Species table not recorded.")
	}
	if selected == nil && len(ids) == 0 {
		fmt.Fprintln(&b, "No species references found in the country or owned planet population groups.")
	}
	for _, id := range ids {
		s, ok := gs.Species.Items[id]
		if !ok {
			if issue := gs.Species.DecodeErrors[id]; issue != "" {
				fmt.Fprintf(&b, "\nSpecies ID %d: %s\n", id, diplomaticRecordIssue(issue))
			} else {
				fmt.Fprintf(&b, "\nSpecies ID %d: record unavailable\n", id)
			}
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
		writeSpeciesDetails(&b, gs, id, s)
		if v := all[id]; v != nil {
			fmt.Fprintf(&b, "  Owned planets referencing groups: %d; groups: %d; population sum (raw save units): %s", len(v.planets), v.groups, v.rawPopulation.String())
			if v.missingSize > 0 {
				fmt.Fprintf(&b, " (incomplete: %d group sizes missing)", v.missingSize)
			}
			if v.duplicateSize > 0 {
				fmt.Fprintf(&b, " (incomplete: %d groups with duplicate sizes)", v.duplicateSize)
			}
			if v.unsupportedSize > 0 {
				fmt.Fprintf(&b, " (incomplete: %d groups with unsupported sizes)", v.unsupportedSize)
			}
			if v.ambiguousGroups > 0 {
				fmt.Fprintf(&b, " (location ambiguous for %d groups)", v.ambiguousGroups)
			}
			fmt.Fprintln(&b)
		} else {
			fmt.Fprintln(&b, "  No matching population groups found on selected country's planets.")
		}
	}
	if missingGroups > 0 || missingSpecies > 0 || duplicateRefs > 0 || ambiguousGroups > 0 {
		fmt.Fprintf(&b, "\nIncomplete references: missing groups=%d, groups without species ID=%d, duplicate group references=%d, groups referenced by multiple owned planets=%d.\n", missingGroups, missingSpecies, duplicateRefs, ambiguousGroups)
	}
	if duplicateGroups > 0 {
		fmt.Fprintf(&b, "\nIncomplete totals: %d referenced groups have duplicate saved records or tables and were excluded.\n", duplicateGroups)
	}
	fmt.Fprintln(&b, "Population values are saved raw group sizes, not validated game UI POP counts. Each group is counted once; multiple planet references do not establish a location. Non-planet carriers are not included.")
	return mcp.NewToolResultText(b.String())
}

func speciesRefText(gs *gamestate.GameState, ref gamestate.SpeciesRef) string {
	switch {
	case ref.Duplicate:
		return "duplicate saved reference fields"
	case ref.Unsupported:
		return "unsupported saved reference shape"
	case ref.Value == nil:
		return "not recorded"
	default:
		return populationSpecies(gs, *ref.Value)
	}
}

func writeSpeciesRefList(b *strings.Builder, label string, refs gamestate.SpeciesRefList) {
	switch {
	case refs.Duplicate:
		fmt.Fprintf(b, "%s: duplicate saved reference fields\n", label)
	case refs.Unsupported:
		fmt.Fprintf(b, "%s: unsupported saved reference shape\n", label)
	case !refs.Recorded:
		fmt.Fprintf(b, "%s: not recorded\n", label)
	default:
		fmt.Fprintf(b, "%s (saved order): %v\n", label, refs.Values)
		seen := map[int]bool{}
		for _, id := range refs.Values {
			if seen[id] {
				fmt.Fprintf(b, "  Duplicate species reference %d in %s\n", id, strings.TrimSpace(label))
			}
			seen[id] = true
		}
	}
}

func writeSpeciesDetails(b *strings.Builder, gs *gamestate.GameState, id int, s gamestate.Species) {
	fmt.Fprintf(b, "  base_ref: %s\n", speciesRefText(gs, s.BaseRef))
	if s.BaseRef.Value != nil && *s.BaseRef.Value == id {
		fmt.Fprintln(b, "  base_ref points to this same saved record.")
	}
	switch h := s.Home; {
	case h.Duplicate:
		fmt.Fprintln(b, "  home_planet: duplicate saved fields. Use raw tools.")
	case !h.Recorded:
		fmt.Fprintln(b, "  home_planet: not recorded")
	default:
		writeSavedValue(b, "  home_planet", h.Saved, 0)
		switch {
		case h.Unsupported || h.Reference.Unsupported:
			fmt.Fprintln(b, "  home_planet: unsupported saved shape. Use raw tools.")
		case h.Reference.Value != nil && (h.Saved.IsScalar() || h.CarrierType != nil && *h.CarrierType == "planet"):
			label := "Saved typed home planet"
			if h.Saved.IsScalar() {
				label = "Legacy scalar home planet ID candidate"
			}
			id := *h.Reference.Value
			if planet, ok := gs.Planets.Planet[id]; ok {
				fmt.Fprintf(b, "  %s %d: %s\n", label, id, name(planet.Name))
			} else {
				fmt.Fprintf(b, "  %s %d: planet record unavailable\n", label, id)
			}
		}
	}
	seen := map[string]bool{}
	for _, field := range s.SavedDetails {
		label := "  " + field.Key
		if seen[field.Key] {
			label += " [duplicate saved key]"
		}
		seen[field.Key] = true
		writeSavedValue(b, label, field.Value, 0)
	}
}
