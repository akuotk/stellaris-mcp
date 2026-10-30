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
	s.AddTool(mcp.NewTool("get_colonies", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Read colony-table records directly, including non-planet carriers. Ownership follows this country's saved owned_planets colony references; carrier types/references remain raw unless a reciprocal planet link validates identity. Arkship carrier serialization has not been validated with a real Arkship save. Values are raw save units."), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")), mcp.WithNumber("colony_id", mcp.Description("Owned colony ID for full details; omit for summary"))), handleColonies)
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
		fmt.Fprintf(&b, "  District references: %s\n  Building cache references: %s\n", colonyReferenceList(colony.Districts), colonyReferenceList(colony.BuildingsCache))
		for _, districtID := range colony.Districts {
			if district, ok := gs.Districts[districtID]; ok {
				fmt.Fprintf(&b, "    District %d: type=%s zones=%s\n", districtID, district.Type, colonyReferenceList(district.Zones))
			} else {
				fmt.Fprintf(&b, "    District %d: unavailable\n", districtID)
			}
		}
		for _, buildingID := range colony.BuildingsCache {
			if building, ok := gs.Buildings[buildingID]; ok {
				fmt.Fprintf(&b, "    Building %d: type=%s\n", buildingID, building.Type)
			} else {
				fmt.Fprintf(&b, "    Building %d: unavailable\n", buildingID)
			}
		}
		fmt.Fprintln(&b, "  POP groups (raw saved sizes; no carrier-to-planet projection):")
		if colony.PopGroups == nil {
			fmt.Fprintln(&b, "    references not recorded")
		}
		for _, groupID := range colony.PopGroups {
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
			writePopDistributions(&b, gs, group)
		}
		fmt.Fprintln(&b, "  Job references (workforce as saved, including zero/negative values):")
		if colony.PopJobs == nil {
			fmt.Fprintln(&b, "    references not recorded")
		}
		for _, jobID := range colony.PopJobs {
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
		}
	}
	return mcp.NewToolResultText(b.String())
}

func colonyCarrier(gs *gamestate.GameState, colonyID int, colony gamestate.Colony) string {
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
