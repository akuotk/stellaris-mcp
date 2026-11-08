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

func registerColonies(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_colonies", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Read saved colonies, species records and population/job back-references. Ownership follows country.owned_planets colony references. Planet carriers use reciprocal planet links. Non-planet carrier identity and population units remain unverified."), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")), mcp.WithNumber("colony_id", mcp.Description("Owned colony ID for full details; omit for summary"))), handleColonies)
}

func handleColonies(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	countryID, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["colony_id"]; ok {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			maxExact := int64(1<<53 - 1)
			maxID = int(maxExact)
		}
		id, err := rawInteger(req, "colony_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &id
	}
	return renderColonies(gs, countryID, country, selected), nil
}

func renderColonies(gs *gamestate.GameState, countryID int, country *gamestate.Country, selected *int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Colony records (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Ownership uses saved country colony references. Numbers retain raw save units. Non-planet carrier identity is not inferred; Arkship serialization is not real-save validated.")
	if len(gs.Colonies) == 0 {
		fmt.Fprintln(&b, "No colony-table records; legacy inline planet data remains available through get_planets.")
		return mcp.NewToolResultText(b.String())
	}
	owned := map[int]bool{}
	for _, id := range country.OwnedPlanets {
		owned[id] = true
	}
	ids := make([]int, 0, len(owned))
	for id := range owned {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if selected != nil {
		if !owned[*selected] {
			return toolError(fmt.Errorf("colony %d is not referenced by selected country", *selected))
		}
		ids = []int{*selected}
	}
	if len(ids) == 0 {
		fmt.Fprintln(&b, "No owned colony references recorded.")
	}
	for _, id := range ids {
		colony, ok := gs.Colonies[id]
		if !ok {
			fmt.Fprintf(&b, "Colony %d: referenced record unavailable\n", id)
			continue
		}
		population := "not recorded"
		if colony.NumPopsRaw != nil {
			population = fmt.Sprint(*colony.NumPopsRaw)
		}
		fmt.Fprintf(&b, "\nColony %d: %s; population=%s; designation=%s\n", id, colonyCarrier(gs, id, colony), population, colony.FinalDesignation)
		if selected == nil {
			continue
		}
		for _, field := range []struct {
			name  string
			value *float64
		}{{"stability", colony.StabilityRaw}, {"crime", colony.CrimeRaw}, {"amenities", colony.AmenitiesRaw}, {"amenities_usage", colony.AmenitiesUsageRaw}, {"free_amenities", colony.FreeAmenitiesRaw}, {"total_housing", colony.TotalHousingRaw}, {"housing_usage", colony.HousingUsageRaw}, {"free_housing", colony.FreeHousingRaw}} {
			if field.value == nil {
				fmt.Fprintf(&b, "  %s: not recorded\n", field.name)
			} else {
				fmt.Fprintf(&b, "  %s: %g\n", field.name, *field.value)
			}
		}
		if colony.AscensionTierRaw != nil {
			fmt.Fprintf(&b, "  ascension_tier: %d\n", *colony.AscensionTierRaw)
		} else {
			fmt.Fprintln(&b, "  ascension_tier: not recorded")
		}
		for _, field := range []struct {
			name   string
			values map[string]float64
		}{{"produces", colony.Produces}, {"upkeep", colony.Upkeep}, {"profits", colony.Profits}} {
			if field.values == nil {
				fmt.Fprintf(&b, "  %s: not recorded\n", field.name)
			} else {
				fmt.Fprintf(&b, "  %s: %s\n", field.name, colonyResources(field.values))
			}
		}
		writeSurfaceLayout(&b, gs, colony.DistrictRefs, colony.BuildingCacheRefs)
		writeExternalSurfaceBuildings(&b, gs, "colony.externally_owned_buildings", colony.ExternalBuildings)
		writeColonySpecies(&b, gs, colony)
		fmt.Fprintln(&b, "  POP groups (raw saved sizes; no carrier-to-planet projection):")
		if colony.PopGroups == nil {
			fmt.Fprintln(&b, "    references not recorded")
		}
		groupRefs := map[int]bool{}
		for _, groupID := range colony.PopGroups {
			if groupRefs[groupID] {
				fmt.Fprintf(&b, "    Duplicate POP group reference %d\n", groupID)
				continue
			}
			groupRefs[groupID] = true
			group, ok := gs.PopGroups[groupID]
			if !ok {
				fmt.Fprintf(&b, "    Group %d: unavailable\n", groupID)
				continue
			}
			size := "not recorded"
			if group.SizeRaw != nil {
				size = fmt.Sprint(*group.SizeRaw)
			}
			species := "not recorded"
			if group.Key.SpeciesID != nil {
				species = populationSpecies(gs, *group.Key.SpeciesID)
			}
			fmt.Fprintf(&b, "    Group %d: species=%s category=%s population=%s\n", groupID, species, group.Key.Category, size)
			writeColonyPopulationLink(&b, "pop_group", id, group.PlanetRef, group.PlanetRefDuplicate)
			growth := governanceValue(group.LastMonthGrowthRaw)
			if group.LastMonthGrowthDuplicate {
				growth = "duplicate saved fields"
			}
			fmt.Fprintf(&b, "      month_start_size=%s last_month_growth=%s (raw save values)\n", councilSavedString(group.MonthStartSize), growth)
			writePopDistributions(&b, gs, group)
		}
		fmt.Fprintln(&b, "  Job references (workforce as saved, including zero/negative values):")
		if colony.PopJobs == nil {
			fmt.Fprintln(&b, "    references not recorded")
		}
		jobRefs := map[int]bool{}
		for _, jobID := range colony.PopJobs {
			if jobRefs[jobID] {
				fmt.Fprintf(&b, "    Duplicate job reference %d\n", jobID)
				continue
			}
			jobRefs[jobID] = true
			job, ok := gs.PopJobs[jobID]
			if !ok {
				fmt.Fprintf(&b, "    Job %d: unavailable\n", jobID)
				continue
			}
			fmt.Fprintf(&b, "    Job %d: type=%s", jobID, job.Type)
			if job.WorkforceRaw != nil {
				fmt.Fprintf(&b, " workforce=%g", *job.WorkforceRaw)
			} else {
				fmt.Fprint(&b, " workforce=not recorded")
			}
			fmt.Fprintln(&b)
			writeColonyPopulationLink(&b, "pop_job", id, job.PlanetRef, job.PlanetRefDuplicate)
		}
	}
	return mcp.NewToolResultText(b.String())
}

func writeColonySpecies(b *strings.Builder, gs *gamestate.GameState, colony gamestate.Colony) {
	fmt.Fprintf(b, "  colonizing_species: %s\n", speciesRefText(gs, colony.ColonizingSpecies))
	writeSpeciesRefList(b, "  species_refs", colony.SpeciesRefs)
	writeSpeciesRefList(b, "  enslaved_species_refs", colony.EnslavedSpeciesRefs)
	info := colony.SpeciesInformation
	switch {
	case !info.Recorded:
		fmt.Fprintln(b, "  species_information: not recorded")
	case info.TableIssue:
		fmt.Fprintln(b, "  species_information: incomplete or duplicate table. Use raw tools.")
	case len(info.Items) == 0 && len(info.DecodeErrors) == 0:
		fmt.Fprintln(b, "  species_information: empty")
	}
	ids := make([]int, 0, len(info.Items)+len(info.DecodeErrors))
	for id := range info.Items {
		ids = append(ids, id)
	}
	for id := range info.DecodeErrors {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		if issue := info.DecodeErrors[id]; issue != "" {
			fmt.Fprintf(b, "    species_information[%d]: %s\n", id, diplomaticRecordIssue(issue))
			continue
		}
		row := info.Items[id]
		membership := armyMembership(colony.SpeciesRefs.Values, id)
		switch {
		case colony.SpeciesRefs.Duplicate:
			membership = "duplicate saved fields"
		case colony.SpeciesRefs.Unsupported:
			membership = "unsupported saved shape"
		}
		fmt.Fprintf(b, "    species_information[%d]: species=%s num_pops=%s num_enslaved=%s (raw save values), species_refs membership=%s\n", id, populationSpecies(gs, id), governanceValue(row.NumPops), governanceValue(row.NumEnslaved), membership)
	}
}

func writeColonyPopulationLink(b *strings.Builder, label string, colonyID int, ref gamestate.OptionalIntRef, duplicate bool) {
	switch {
	case duplicate:
		fmt.Fprintf(b, "      %s.planet: duplicate saved reference fields\n", label)
	case ref.Unsupported:
		fmt.Fprintf(b, "      %s.planet: unsupported saved reference shape\n", label)
	case ref.Value == nil:
		fmt.Fprintf(b, "      %s.planet: not recorded\n", label)
	case *ref.Value == colonyID:
		fmt.Fprintf(b, "      %s.planet=%d matches colony ID\n", label, *ref.Value)
	default:
		fmt.Fprintf(b, "      %s.planet=%d differs from colony ID %d\n", label, *ref.Value, colonyID)
	}
}

func colonyCarrier(gs *gamestate.GameState, colonyID int, colony gamestate.Colony) string {
	if colony.Carrier.Unsupported {
		return "carrier unsupported or duplicate (identity unvalidated; use raw tools)"
	}
	carrierType := colony.Carrier.Type
	if carrierType == "" {
		carrierType = "not recorded"
	}
	reference := "not recorded"
	if colony.Carrier.ReferenceID != nil {
		reference = fmt.Sprint(*colony.Carrier.ReferenceID)
	}
	label := fmt.Sprintf("carrier type=%s reference=%s", carrierType, reference)
	if colony.Carrier.Type == "planet" && colony.Carrier.ReferenceID != nil {
		if p, ok := gs.Planets.Planet[*colony.Carrier.ReferenceID]; ok && p.ColonyID != nil && *p.ColonyID == colonyID {
			return label + " (" + name(p.Name) + ", reciprocal planet link validated)"
		}
	}
	return label + " (carrier identity unvalidated)"
}

func colonyReferenceList(ids []int) string {
	if ids == nil {
		return "not recorded"
	}
	return fmt.Sprint(ids)
}

func colonyResources(values map[string]float64) string {
	parts := make([]string, 0, len(values))
	for _, key := range federationSortedKeys(values) {
		parts = append(parts, fmt.Sprintf("%s=%g", key, values[key]))
	}
	return strings.Join(parts, ", ")
}
