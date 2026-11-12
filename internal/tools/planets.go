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

func registerPlanets(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_planets", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("List owned planets or surveyed habitable planets with available=true. Pass planet_id for saved districts, zones, buildings, deposits, holder IDs and terraforming details."),
			mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save")),
			mcp.WithNumber("planet_id", mcp.Description("Planet ID for detailed view (omit for summary list)"), mcp.DefaultNumber(-1)),
			mcp.WithBoolean("available", mcp.Description("Show uncolonized habitable planets instead of owned"), mcp.DefaultBool(false)),
		),
		handlePlanets,
	)
}

func handlePlanets(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	countryID, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}

	planetID := int(req.GetFloat("planet_id", -1))
	if planetID >= 0 {
		return planetDetail(gs, planetID)
	}
	if req.GetBool("available", false) {
		return planetAvailable(gs, countryID, c)
	}
	return planetSummary(gs, countryID, c)
}

func planetSummary(gs *gamestate.GameState, countryID int, c *gamestate.Country) (*mcp.CallToolResult, error) {
	type entry struct {
		id int
		p  gamestate.Planet
	}
	var owned []entry
	for id, p := range gs.Planets.Planet {
		if p.OwnerID != nil && *p.OwnerID == countryID && isColonized(p) {
			owned = append(owned, entry{id, p})
		}
	}
	sort.Slice(owned, func(i, j int) bool { return owned[i].id < owned[j].id })

	var b strings.Builder
	fmt.Fprintf(&b, "=== Planets (%s) — %d planets ===\n", name(c.Adjective), len(owned))
	fmt.Fprintf(&b, "Use get_planets with planet_id for details. Colony values take precedence when present; saved planet values are retained otherwise.\n\n")
	fmt.Fprintf(&b, "%-6s %-25s %-16s %4s %5s %5s %-12s %-20s %-30s %s\n", "ID", "Name", "Class", "Size", "Pop(raw)", "Stab", "Design.", "Districts", "Buildings", "Deficit")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 160))

	for _, e := range owned {
		fmt.Fprintf(&b, "%-6d %-25s %-16s %4d %5s %5s %-12s %-20s %-30s %s\n",
			e.id,
			truncate(name(e.p.Name), 25),
			l(e.p.PlanetClass),
			e.p.PlanetSize,
			savedInt(e.p.NumPopsRaw),
			savedFloat(e.p.StabilityRaw),
			planetDesignation(e.p),
			districtSummary(gs, e.p),
			buildingSummary(gs, e.p),
			deficitSummary(e.p),
		)
	}

	if len(owned) == 0 {
		fmt.Fprintf(&b, "No owned planets.\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func planetDetail(gs *gamestate.GameState, planetID int) (*mcp.CallToolResult, error) {
	p, ok := gs.Planets.Planet[planetID]
	if !ok {
		return toolError(fmt.Errorf("planet %d not found", planetID)), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Planet %d: %s ===\n", planetID, name(p.Name))
	var systems []string
	for id, system := range gs.Systems {
		for _, candidate := range system.Planets {
			if candidate == planetID {
				systems = append(systems, fmt.Sprintf("%s (ID %d)", name(system.Name), id))
				break
			}
		}
	}
	sort.Strings(systems)
	if len(systems) > 0 {
		fmt.Fprintf(&b, "System: %s\n", strings.Join(systems, ", "))
	}
	fmt.Fprintf(&b, "Class: %s, Size: %d\n", l(p.PlanetClass), p.PlanetSize)
	fmt.Fprintf(&b, "Designation: %s (Ascension Tier %s)\n", planetDesignation(p), savedInt(p.AscensionTierRaw))
	if p.OwnerID != nil {
		fmt.Fprintf(&b, "Owner: %d\n", *p.OwnerID)
	} else {
		fmt.Fprintf(&b, "Owner: not recorded\n")
	}
	if p.ControllerID != nil {
		fmt.Fprintf(&b, "Controller: %d\n", *p.ControllerID)
	} else {
		fmt.Fprintln(&b, "Controller: not recorded")
	}
	if p.ColonyID != nil {
		colony, ok := gs.Colonies[*p.ColonyID]
		if !ok || colony.Carrier.Type != "planet" || colony.Carrier.ReferenceID == nil || *colony.Carrier.ReferenceID != planetID {
			fmt.Fprintf(&b, "Colony %d: reciprocal planet link unavailable; colony values not joined\n", *p.ColonyID)
		} else {
			fmt.Fprintln(&b, "Colony values take precedence where recorded; otherwise saved planet values are retained.")
		}
	}
	fmt.Fprintf(&b, "\nPopulation: %s (raw save units)\n", savedInt(p.NumPopsRaw))
	fmt.Fprintf(&b, "Stability: %s\n", savedFloat(p.StabilityRaw))
	fmt.Fprintf(&b, "Crime: %s\n", savedFloat(p.CrimeRaw))
	fmt.Fprintf(&b, "Amenities: %s (used: %s, free: %s)\n", savedFloat(p.AmenitiesRaw), savedFloat(p.AmenitiesUsageRaw), savedFloat(p.FreeAmenitiesRaw))
	fmt.Fprintf(&b, "Housing: %s (used: %s, free: %s)\n", savedFloat(p.TotalHousingRaw), savedFloat(p.HousingUsageRaw), savedFloat(p.FreeHousingRaw))

	if len(p.PopGroups) > 0 {
		fmt.Fprintf(&b, "\nPOP groups (population and growth in raw save units; happiness/habitability as saved; group totals may differ from cached colony population):\n")
		for _, id := range p.PopGroups {
			group, ok := gs.PopGroups[id]
			if !ok {
				fmt.Fprintf(&b, "  Group %d: referenced record unavailable\n", id)
				continue
			}
			if group.RecordDuplicate {
				fmt.Fprintf(&b, "  Group %d: duplicate saved group records or tables; use raw save tools\n", id)
				continue
			}
			fmt.Fprintf(&b, "  Group %d: species=%s category=%s population=%s growth=%s happiness=%s habitability=%s\n", id, populationSpecies(gs, group.Key.Species), group.Key.Category,
				popGroupNumberText(group.SavedSize, false), popGroupNumberText(group.SavedGrowth, false), popGroupNumberText(group.SavedHappiness, false), popGroupNumberText(group.SavedHabitability, false))
			writePopDistributions(&b, gs, group)
		}
	}

	// Only jobs with reciprocal references are included in the positive workforce total.
	if p.ColonyID != nil {
		if colony, ok := gs.Colonies[*p.ColonyID]; ok && colony.Carrier.Type == "planet" && colony.Carrier.ReferenceID != nil && *colony.Carrier.ReferenceID == planetID {
			if colony.PopJobs == nil {
				fmt.Fprintln(&b, "\nJob references: not recorded")
			} else {
				seen := map[int]bool{}
				for _, id := range colony.PopJobs {
					if seen[id] {
						fmt.Fprintf(&b, "Job %d: duplicate reference, counted once\n", id)
						continue
					}
					seen[id] = true
					job, exists := gs.PopJobs[id]
					if !exists {
						fmt.Fprintf(&b, "Job %d: referenced record unavailable\n", id)
						continue
					}
					if job.PlanetID == nil {
						fmt.Fprintf(&b, "Job %d: colony back-reference not recorded\n", id)
						continue
					}
					if *job.PlanetID != *p.ColonyID {
						fmt.Fprintf(&b, "Job %d: colony back-reference mismatch (%d)\n", id, *job.PlanetID)
						continue
					}
					if job.WorkforceRaw == nil {
						fmt.Fprintf(&b, "Job %d: workforce not recorded\n", id)
					}
				}
			}
		}
	}
	jobCounts := jobBreakdown(gs, planetID)
	if len(jobCounts) > 0 {
		fmt.Fprintf(&b, "\nPositive job workforce (raw save units; zero and negative values excluded):\n")
		for _, jc := range jobCounts {
			fmt.Fprintf(&b, "  %s: %g\n", l(jc.jobType), jc.workforce)
		}
	}

	// District slot summary
	counts := countDistrictTypes(gs, p)
	totalUsed := 0
	for _, n := range counts {
		totalUsed += n
	}
	if p.Districts != nil {
		fmt.Fprintf(&b, "\nDistrict Slots: %d used (planet size %d)\n", totalUsed, p.PlanetSize)
	}
	for _, dtype := range federationSortedKeys(counts) {
		fmt.Fprintf(&b, "  %s: %d\n", l(dtype), counts[dtype])
	}
	writeSurfaceLayout(&b, gs, p.DistrictRefs, p.BuildingCacheRefs)
	for i, district := range p.LegacyDistricts {
		writeSavedValue(&b, fmt.Sprintf("  Legacy district[%d]", i), district, 0)
	}
	if p.LegacyBuildingRefs.Recorded {
		fmt.Fprintf(&b, "  Legacy planet.buildings references: %s\n", surfaceRefText(p.LegacyBuildingRefs))
		writeSurfaceBuildings(&b, gs, "    ", p.LegacyBuildingRefs.Values, nil)
	}
	writeExternalSurfaceBuildings(&b, gs, "planet.externally_owned_buildings", p.ExternalBuildings)
	if p.ColonyID != nil {
		if c, ok := gs.Colonies[*p.ColonyID]; ok && c.Carrier.Type == "planet" && c.Carrier.ReferenceID != nil && *c.Carrier.ReferenceID == planetID {
			writeExternalSurfaceBuildings(&b, gs, "colony.externally_owned_buildings", c.ExternalBuildings)
		}
	}
	writePlanetDeposits(&b, gs.Deposit, planetID, p.DepositRefs)
	var features, blockers []string
	for _, id := range p.DepositRefs.Values {
		if dep, ok := gs.Deposit.Items[id]; ok && dep.Type != "" {
			label := l(dep.Type)
			if isBlocker(dep.Type) {
				if dep.SwapType != "" {
					label += " -> " + l(dep.SwapType)
				}
				blockers = append(blockers, label)
			} else {
				features = append(features, label)
			}
		}
	}
	if len(features) > 0 {
		fmt.Fprintf(&b, "\nPlanetary Features (%d):\n", len(features))
		for _, label := range features {
			fmt.Fprintf(&b, "  - %s\n", label)
		}
	}
	if len(blockers) > 0 {
		fmt.Fprintf(&b, "\nBlockers (%d):\n", len(blockers))
		for _, label := range blockers {
			fmt.Fprintf(&b, "  - %s\n", label)
		}
	}
	if p.TerraformDuplicate {
		fmt.Fprintln(&b, "terraform_process: duplicate saved fields")
	} else if p.TerraformProcess != nil {
		writeSavedValue(&b, "terraform_process (saved values; progress units unverified)", *p.TerraformProcess, 0)
	}

	// Timed modifiers
	if len(p.TimedModifier.Items) > 0 {
		fmt.Fprintf(&b, "\nModifiers:\n")
		for _, m := range p.TimedModifier.Items {
			if m.Days < 0 {
				fmt.Fprintf(&b, "  - %s (permanent)\n", l(m.Modifier))
			} else {
				fmt.Fprintf(&b, "  - %s (%d days remaining)\n", l(m.Modifier), m.Days)
			}
		}
	}

	if len(p.Produces) > 0 {
		fmt.Fprintf(&b, "\nMonthly Production:\n  %s\n", formatResources(p.Produces))
	}
	if len(p.Upkeep) > 0 {
		fmt.Fprintf(&b, "Monthly Upkeep:\n  %s\n", formatResources(p.Upkeep))
	}
	if len(p.Profits) > 0 {
		fmt.Fprintf(&b, "Monthly Profit:\n  %s\n", formatResources(p.Profits))
	}

	return mcp.NewToolResultText(b.String()), nil
}

func countDistrictTypes(gs *gamestate.GameState, p gamestate.Planet) map[string]int {
	counts := make(map[string]int)
	seen := map[int]bool{}
	for _, did := range p.Districts {
		if seen[did] {
			continue
		}
		seen[did] = true
		if d, ok := gs.Districts.Items[did]; ok {
			level := d.Level
			if level <= 0 {
				level = 1
			}
			counts[d.Type] += level
		}
	}
	return counts
}

func districtSummary(gs *gamestate.GameState, p gamestate.Planet) string {
	if p.DistrictRefs.Duplicate || p.DistrictRefs.Unsupported {
		return surfaceRefText(p.DistrictRefs)
	}
	if p.Districts == nil {
		if len(p.LegacyDistricts) != 0 {
			return fmt.Sprintf("%d saved legacy district entries", len(p.LegacyDistricts))
		}
		return "not recorded"
	}
	counts := countDistrictTypes(gs, p)
	if len(counts) == 0 {
		return fmt.Sprintf("0/%d", p.PlanetSize)
	}
	totalUsed := 0
	for _, n := range counts {
		totalUsed += n
	}
	parts := make([]string, 0, len(counts))
	for dtype, n := range counts {
		parts = append(parts, fmt.Sprintf("%s:%d", l(dtype), n))
	}
	sort.Strings(parts)
	return fmt.Sprintf("%d [%s]", totalUsed, strings.Join(parts, " "))
}

type jobCount struct {
	jobType   string
	workforce float64
}

func jobBreakdown(gs *gamestate.GameState, planetID int) []jobCount {
	counts := make(map[string]float64)
	p, exists := gs.Planets.Planet[planetID]
	if !exists {
		return nil
	}
	if p.ColonyID != nil {
		colony, ok := gs.Colonies[*p.ColonyID]
		if !ok || colony.Carrier.Type != "planet" || colony.Carrier.ReferenceID == nil || *colony.Carrier.ReferenceID != planetID {
			return nil
		}
		seen := map[int]bool{}
		for _, id := range colony.PopJobs {
			if seen[id] {
				continue
			}
			seen[id] = true
			job, ok := gs.PopJobs[id]
			if ok && job.PlanetID != nil && *job.PlanetID == *p.ColonyID && job.WorkforceRaw != nil && *job.WorkforceRaw > 0 {
				counts[job.Type] += *job.WorkforceRaw
			}
		}
	} else {
		for _, job := range gs.PopJobs {
			if job.PlanetID != nil && *job.PlanetID == planetID && job.WorkforceRaw != nil && *job.WorkforceRaw > 0 {
				counts[job.Type] += *job.WorkforceRaw
			}
		}
	}
	result := make([]jobCount, 0, len(counts))
	for t, w := range counts {
		result = append(result, jobCount{t, w})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].workforce != result[j].workforce {
			return result[i].workforce > result[j].workforce
		}
		return result[i].jobType < result[j].jobType
	})
	return result
}

func depositSummary(gs *gamestate.GameState, p gamestate.Planet) string {
	if p.DepositRefs.Duplicate || p.DepositRefs.Unsupported {
		return surfaceRefText(p.DepositRefs)
	}
	if len(p.Deposits) == 0 {
		return "-"
	}
	var names []string
	for _, did := range p.Deposits {
		dep, ok := gs.Deposit.Items[did]
		if !ok || dep.Type == "" {
			continue
		}
		names = append(names, l(dep.Type))
	}
	if len(names) == 0 {
		return "-"
	}
	return strings.Join(names, ", ")
}

var habitableClasses = map[string]bool{
	"pc_continental": true, "pc_tropical": true, "pc_arid": true,
	"pc_desert": true, "pc_ocean": true, "pc_arctic": true,
	"pc_alpine": true, "pc_savannah": true, "pc_tundra": true,
	"pc_gaia": true, "pc_tomb": true, "pc_relic": true,
}

func planetDesignation(p gamestate.Planet) string {
	if p.FinalDesignationRaw == nil {
		return "not recorded"
	}
	return l(*p.FinalDesignationRaw)
}

func isColonized(p gamestate.Planet) bool {
	return p.ColonyID != nil || p.NumPops > 0 || p.FinalDesignation != ""
}

func buildingSummary(gs *gamestate.GameState, p gamestate.Planet) string {
	if p.Districts == nil && !p.DistrictRefs.Recorded && !p.BuildingCacheRefs.Recorded && !p.LegacyBuildingRefs.Recorded {
		return "not recorded"
	}
	counts := make(map[string]int)
	seen := map[int]bool{}
	add := func(ids []int) {
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			if building, ok := gs.Buildings.Items[id]; ok {
				counts[building.Type]++
			}
		}
	}
	add(p.LegacyBuildingRefs.Values)
	add(p.BuildingsCache)
	for _, did := range p.Districts {
		d, ok := gs.Districts.Items[did]
		if !ok {
			continue
		}
		for _, zid := range d.ZoneRefs.Values {
			z, ok := gs.Zones.Items[zid]
			if !ok {
				continue
			}
			add(z.BuildingRefs.Values)
		}
	}
	if len(counts) == 0 {
		return "-"
	}
	type bldgCount struct {
		name  string
		count int
	}
	entries := make([]bldgCount, 0, len(counts))
	for t, n := range counts {
		entries = append(entries, bldgCount{l(t), n})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].name < entries[j].name
	})
	top := 3
	if len(entries) < top {
		top = len(entries)
	}
	parts := make([]string, top)
	for i := 0; i < top; i++ {
		parts[i] = fmt.Sprintf("%s x%d", entries[i].name, entries[i].count)
	}
	result := strings.Join(parts, ", ")
	if len(entries) > top {
		result += fmt.Sprintf(" +%d more", len(entries)-top)
	}
	return result
}

func deficitSummary(p gamestate.Planet) string {
	if p.Profits == nil {
		return "not recorded"
	}
	var deficits []string
	for res, val := range p.Profits {
		if val < -0.1 {
			deficits = append(deficits, l(res))
		}
	}
	if len(deficits) == 0 {
		return "-"
	}
	sort.Strings(deficits)
	return strings.Join(deficits, ", ")
}

func planetAvailable(gs *gamestate.GameState, countryID int, c *gamestate.Country) (*mcp.CallToolResult, error) {
	type entry struct {
		id       int
		p        gamestate.Planet
		surveyed bool
	}
	var planets []entry
	for id, p := range gs.Planets.Planet {
		if !habitableClasses[p.PlanetClass] || isColonized(p) {
			continue
		}
		// Skip planets owned by another empire
		if p.OwnerID != nil {
			continue
		}
		if p.SurveyedByID == nil || *p.SurveyedByID != countryID {
			continue
		}
		planets = append(planets, entry{id, p, true})
	}
	// Sort by size descending
	sort.Slice(planets, func(i, j int) bool {
		return planets[i].p.PlanetSize > planets[j].p.PlanetSize
	})

	var b strings.Builder
	fmt.Fprintf(&b, "=== Available Habitable Planets (%s) — %d planets ===\n", name(c.Adjective), len(planets))
	fmt.Fprintf(&b, "Use get_planets with planet_id for details.\n\n")

	fmt.Fprintf(&b, "%-6s %-25s %-18s %4s %s\n", "ID", "Name", "Class", "Size", "Features")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 90))

	for _, e := range planets {
		fmt.Fprintf(&b, "%-6d %-25s %-18s %4d %s\n",
			e.id,
			truncate(name(e.p.Name), 25),
			l(e.p.PlanetClass),
			e.p.PlanetSize,
			depositSummary(gs, e.p),
		)
	}

	if len(planets) == 0 {
		fmt.Fprintf(&b, "No available habitable planets found.\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func isBlocker(depositType string) bool {
	blockers := []string{
		"_blocker",
		"d_failing_infrastructure",
		"d_decrepit_dwellings",
		"d_active_volcano",
		"d_deep_sinkhole",
		"d_dense_jungle",
		"d_noxious_swamp",
		"d_quicksand_basin",
		"d_radioactive_wasteland",
		"d_toxic_kelp",
		"d_machine_empire_ruins",
	}
	for _, b := range blockers {
		if strings.Contains(depositType, b) || depositType == b {
			return true
		}
	}
	return false
}
