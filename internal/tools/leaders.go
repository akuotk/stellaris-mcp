package tools

import (
	"context"
	"fmt"
	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"slices"
	"sort"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLeaders(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_leaders", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("List saved country leaders, ruler and heir references. Pass leader_id for country, species, planet, council and army ID checks, raw experience, dates and available traits."),
			mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save")),
			mcp.WithNumber("leader_id", mcp.Description("Leader ID for detailed view (omit for summary)"), mcp.DefaultNumber(-1)),
		),
		handleLeaders,
	)
}

func handleLeaders(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}

	leaderID, err := rawInteger(req, "leader_id", -1, -1, maxRecordID)
	if err != nil {
		return toolError(err), nil
	}
	if leaderID >= 0 {
		return leaderDetail(gs, leaderID)
	}

	countryID, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return leaderSummary(gs, countryID, c)
}

func leaderSummary(gs *gamestate.GameState, countryID int, c *gamestate.Country) (*mcp.CallToolResult, error) {
	type entry struct {
		id int
		l  gamestate.Leader
	}
	owned := make(map[int]int, len(c.OwnedLeaders))
	for _, id := range c.OwnedLeaders {
		owned[id]++
	}
	var leaders []entry
	for id, ldr := range gs.Leaders {
		if owned[id] > 0 {
			leaders = append(leaders, entry{id, ldr})
		}
	}
	sort.Slice(leaders, func(i, j int) bool {
		if leaders[i].l.Class != leaders[j].l.Class {
			return leaders[i].l.Class < leaders[j].l.Class
		}
		return leaders[i].id < leaders[j].id
	})

	var b strings.Builder
	fmt.Fprintf(&b, "=== Leaders (%s) — %d leaders ===\n", name(c.Adjective), len(leaders))
	writeCountryLeadershipRefs(&b, gs, countryID, c)
	if c.OwnedLeaderRefs.Unsupported {
		fmt.Fprintln(&b, "country.owned_leaders unsupported or duplicate. Use raw tree/byte tools.")
	}
	if gs.LeaderRecords.TableIssue || len(gs.LeaderRecords.DecodeErrors) > 0 {
		fmt.Fprintln(&b, "Leader table decoding incomplete. Use raw tree/byte tools for unresolved references.")
	}
	fmt.Fprintf(&b, "Use get_leaders with leader_id for full detail.\n\n")

	fmt.Fprintf(&b, "%-12s %-25s %-12s %3s %3s %-20s %s\n",
		"ID", "Name", "Class", "Lvl", "Age", "Assignment", "Traits")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 110))

	for _, e := range leaders {
		assignment := resolveAssignment(gs, e.id, e.l)
		traits := strings.Join(ll(e.l.Traits), ", ")
		if traits == "" {
			traits = "-"
		}
		fmt.Fprintf(&b, "%-12d %-25s %-12s %3d %3d %-20s %s\n",
			e.id,
			truncate(name(e.l.Name.FullNames), 25),
			e.l.Class,
			e.l.Level,
			e.l.Age,
			truncate(assignment, 20),
			traits,
		)
		if owned[e.id] > 1 {
			fmt.Fprintf(&b, "  Leader %d has duplicate saved owned_leaders references\n", e.id)
		}
		if e.l.CountryID != nil && *e.l.CountryID != countryID {
			fmt.Fprintf(&b, "  Leader %d saved country ID differs from the selected country ID\n", e.id)
		}
	}

	missing := make([]int, 0)
	for id := range owned {
		if _, ok := gs.Leaders[id]; !ok {
			missing = append(missing, id)
		}
	}
	sort.Ints(missing)
	for _, id := range missing {
		fmt.Fprintf(&b, "Leader %d: %s\n", id, diplomaticRecordIssue(gs.LeaderRecords.DecodeErrors[id]))
	}
	if len(leaders) == 0 && len(missing) == 0 && !c.OwnedLeaderRefs.Unsupported {
		fmt.Fprintf(&b, "No owned leader references recorded.\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func leaderDetail(gs *gamestate.GameState, leaderID int) (*mcp.CallToolResult, error) {
	ldr, ok := gs.Leaders[leaderID]
	if !ok {
		if gs.LeaderRecords.TableIssue && len(gs.LeaderRecords.Items) == 0 {
			return toolError(fmt.Errorf("leader %d unavailable because the saved leaders table is unsupported or duplicate", leaderID)), nil
		}
		return toolError(fmt.Errorf("leader %d: %s", leaderID, diplomaticRecordIssue(gs.LeaderRecords.DecodeErrors[leaderID]))), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Leader %d: %s ===\n", leaderID, name(ldr.Name.FullNames))
	fmt.Fprintf(&b, "Class: %s\n", ldr.Class)
	fmt.Fprintf(&b, "Level: %d (bonus: %d)\n", ldr.Level, ldr.BonusSkillLevel)
	fmt.Fprintf(&b, "Experience: %s (saved value)\n", councilSavedString(ldr.SavedExperience))
	fmt.Fprintf(&b, "Age: %d\n", ldr.Age)
	fmt.Fprintf(&b, "Gender: %s\n", ldr.Gender)
	fmt.Fprintf(&b, "Ethic: %s\n", l(ldr.Ethic))
	fmt.Fprintf(&b, "Job: %s\n", l(ldr.Job))
	fmt.Fprintf(&b, "Recruited: %s\n", ldr.RecruitmentDate)

	writeLeaderSavedLinks(&b, gs, leaderID, ldr)
	if len(ldr.SavedDetails) > 0 {
		writeSavedValue(&b, "Saved lifecycle and trait fields", clausewitz.Value{Object: &clausewitz.Object{Pairs: ldr.SavedDetails}}, 0)
	}
	fmt.Fprintf(&b, "\nAssignment: %s\n", resolveAssignment(gs, leaderID, ldr))

	if ldr.Location.Unsupported {
		fmt.Fprintln(&b, "Location: unsupported or duplicate. Use raw tree/byte tools.")
	} else if ldr.Location.Type != "" {
		if ldr.Location.IDRaw == nil {
			fmt.Fprintf(&b, "Location: %s (id not recorded)\n", ldr.Location.Type)
		} else {
			fmt.Fprintf(&b, "Location: %s (id=%d)\n", ldr.Location.Type, *ldr.Location.IDRaw)
		}
	}
	if ldr.CouncilLocation.Unsupported {
		fmt.Fprintln(&b, "Council: unsupported or duplicate. Use raw tree/byte tools.")
	} else if ldr.CouncilLocation.Type != "" {
		id := "not recorded"
		if ldr.CouncilLocation.IDRaw != nil {
			id = fmt.Sprint(*ldr.CouncilLocation.IDRaw)
		}
		position := "not recorded"
		if ldr.CouncilLocation.PositionRaw != nil {
			position = fmt.Sprint(*ldr.CouncilLocation.PositionRaw)
		}
		fmt.Fprintf(&b, "Council: %s (id=%s, position=%s)\n",
			ldr.CouncilLocation.Type, id, position)
	}

	fmt.Fprintf(&b, "\nTraits:\n")
	if len(ldr.Traits) == 0 {
		fmt.Fprintf(&b, "  (none)\n")
	}
	for _, t := range ldr.Traits {
		fmt.Fprintf(&b, "  - %s\n", l(t))
	}

	return mcp.NewToolResultText(b.String()), nil
}

func resolveAssignment(gs *gamestate.GameState, leaderID int, ldr gamestate.Leader) string {
	if ldr.CouncilLocation.Type == "council_position" {
		if ldr.CouncilLocation.IDRaw == nil {
			return "Council position reference not recorded"
		}
		posID := *ldr.CouncilLocation.IDRaw
		if gs.CouncilPositions.TableIssue {
			return fmt.Sprintf("Council #%d (saved position table incomplete)", posID)
		}
		position, found := gs.CouncilPositions.Items[posID]
		if !found {
			return fmt.Sprintf("Council #%d (%s)", posID, diplomaticRecordIssue(gs.CouncilPositions.DecodeErrors[posID]))
		}
		switch {
		case position.Leader == nil:
			return fmt.Sprintf("Council #%d (position.leader not recorded)", posID)
		case *position.Leader != leaderID:
			return fmt.Sprintf("Council #%d (saved position.leader=%d differs)", posID, *position.Leader)
		case ldr.CountryRef.Unsupported || (position.Country != nil && ldr.CountryID != nil && *position.Country != *ldr.CountryID):
			return fmt.Sprintf("Council #%d (saved country references unresolved or differ)", posID)
		case position.Country == nil || ldr.CountryID == nil:
			return fmt.Sprintf("Council #%d (reciprocal saved IDs agree, country reference incomplete)", posID)
		case position.Type != nil:
			return fmt.Sprintf("%s (Council #%d, reciprocal saved IDs agree)", l(*position.Type), posID)
		default:
			return fmt.Sprintf("Council #%d (reciprocal saved IDs agree, type not recorded)", posID)
		}
	}
	if ldr.Location.Unsupported || ldr.LocationAmbiguous {
		return "Location unsupported or duplicate"
	}
	if ldr.Location.Type == "army" {
		return resolveLeaderArmy(gs, leaderID, ldr.Location.IDRaw)
	}
	// A ship ID can collide with an unrelated fleet ID, so follow the ship reference first.
	if ldr.Location.Type == "ship" {
		if ldr.Location.IDRaw == nil {
			return "Ship reference not recorded"
		}
		shipID := *ldr.Location.IDRaw
		ship, ok := gs.Ships[shipID]
		if !ok {
			return fmt.Sprintf("Ship %d (record unavailable)", shipID)
		}
		if ship.FleetID != nil {
			if fleet, ok := gs.Fleet[*ship.FleetID]; ok {
				count := 0
				for _, id := range fleet.Ships {
					if id == shipID {
						count++
					}
				}
				if count == 1 {
					return fmt.Sprintf("Ship %d, fleet %s", shipID, name(fleet.Name))
				}
				if count > 1 {
					return fmt.Sprintf("Ship %d, fleet %d (duplicate saved membership)", shipID, *ship.FleetID)
				}
				return fmt.Sprintf("Ship %d, fleet %d (membership unverified)", shipID, *ship.FleetID)
			}
			return fmt.Sprintf("Ship %d, fleet %d (record unavailable)", shipID, *ship.FleetID)
		}
		return fmt.Sprintf("Ship %d (fleet reference not recorded)", shipID)
	}
	// Governor
	if ldr.Location.Type == "planet" {
		if ldr.Location.IDRaw == nil {
			return "Planet reference not recorded"
		}
		if p, ok := gs.Planets.Planet[*ldr.Location.IDRaw]; ok {
			return fmt.Sprintf("Planet: %s", name(p.Name))
		}
		return fmt.Sprintf("Planet %d (record unavailable)", *ldr.Location.IDRaw)
	}
	if ldr.Job != "" {
		return l(ldr.Job)
	}
	if ldr.CouncilLocation.Unsupported {
		return "Council location unsupported or duplicate"
	}
	return "Unassigned"
}

func writeCountryLeadershipRefs(b *strings.Builder, gs *gamestate.GameState, countryID int, country *gamestate.Country) {
	fmt.Fprintln(b, "Saved ruler and heir references")
	if country.RulerTitle.Recorded {
		fmt.Fprintf(b, "  country.ruler_title=%s\n", councilSavedString(country.RulerTitle))
	}
	for _, role := range []struct {
		key string
		ref gamestate.OptionalIntRef
	}{{"country.ruler", country.RulerRef}, {"government.heir", country.CouncilGovernment.Heir}} {
		if role.key == "government.heir" && country.CouncilGovernment.Unsupported {
			fmt.Fprintln(b, "  government.heir unavailable because the saved government is unsupported or duplicate")
			continue
		}
		fmt.Fprintf(b, "  %s=%s (saved reference)\n", role.key, optionalIntRefText(role.ref))
		if role.ref.Value == nil {
			continue
		}
		id := *role.ref.Value
		fmt.Fprintf(b, "    country.owned_leaders membership=%s\n", leaderMembership(country, id))
		leader, found := gs.Leaders[id]
		if !found {
			fmt.Fprintf(b, "    Leader ID candidate %d (%s)\n", id, diplomaticRecordIssue(gs.LeaderRecords.DecodeErrors[id]))
			continue
		}
		fmt.Fprintf(b, "    Leader ID candidate %d (%s) leader.country=%s\n", id, name(leader.Name.FullNames), optionalIntRefText(leader.CountryRef))
		if leader.CountryID != nil {
			fmt.Fprintf(b, "    leader.country %s the saved country ID\n", compareSavedIDs(*leader.CountryID, countryID))
		}
	}
}

func leaderMembership(country *gamestate.Country, id int) string {
	if country.OwnedLeaderRefs.Unsupported {
		return "unsupported or duplicate"
	}
	return armyMembership(country.OwnedLeaders, id)
}

func writeLeaderSavedLinks(b *strings.Builder, gs *gamestate.GameState, id int, leader gamestate.Leader) {
	fmt.Fprintf(b, "\nSaved references: country=%s species=%s planet=%s\n", optionalIntRefText(leader.CountryRef), optionalIntRefText(leader.SpeciesRef), optionalIntRefText(leader.PlanetRef))
	if ref := leader.CountryRef.Value; ref != nil {
		if _, found := gs.Country[*ref]; found {
			fmt.Fprintf(b, "  Country ID candidate %d (%s)\n", *ref, countryName(gs, *ref))
		} else {
			fmt.Fprintf(b, "  Country ID candidate %d (record unavailable)\n", *ref)
		}
	}
	if ref := leader.SpeciesRef.Value; ref != nil {
		if species, found := gs.Species.Items[*ref]; found {
			fmt.Fprintf(b, "  Species ID candidate %d (%s)\n", *ref, name(species.Name))
		} else {
			fmt.Fprintf(b, "  Species ID candidate %d (%s)\n", *ref, diplomaticRecordIssue(gs.Species.DecodeErrors[*ref]))
		}
	}
	if ref := leader.PlanetRef.Value; ref != nil {
		if planet, found := gs.Planets.Planet[*ref]; found {
			fmt.Fprintf(b, "  Saved leader.planet ID candidate %d (%s)\n", *ref, name(planet.Name))
		} else {
			fmt.Fprintf(b, "  Saved leader.planet ID candidate %d (record unavailable)\n", *ref)
		}
	}
	var countryIDs []int
	for countryID, country := range gs.Country {
		if sameLeaderRef(country.RulerRef, id) || sameLeaderRef(country.CouncilGovernment.Heir, id) || slices.Contains(country.OwnedLeaders, id) {
			countryIDs = append(countryIDs, countryID)
		}
	}
	sort.Ints(countryIDs)
	for _, countryID := range countryIDs {
		country := gs.Country[countryID]
		var roles []string
		if sameLeaderRef(country.RulerRef, id) {
			roles = append(roles, "country.ruler")
		}
		if sameLeaderRef(country.CouncilGovernment.Heir, id) {
			roles = append(roles, "government.heir")
		}
		if slices.Contains(country.OwnedLeaders, id) {
			roles = append(roles, "owned_leaders="+leaderMembership(&country, id))
		}
		fmt.Fprintf(b, "  Country %d saved references: %s\n", countryID, strings.Join(roles, ", "))
		if leader.CountryID != nil && *leader.CountryID != countryID {
			fmt.Fprintln(b, "    Referencing country ID differs from saved leader.country")
		}
	}
}

func sameLeaderRef(ref gamestate.OptionalIntRef, id int) bool {
	return !ref.Unsupported && ref.Value != nil && *ref.Value == id
}

func resolveLeaderArmy(gs *gamestate.GameState, leaderID int, ref *int) string {
	if ref == nil {
		return "Army reference not recorded"
	}
	id := *ref
	if gs.Armies.TableIssue {
		return fmt.Sprintf("Army %d (saved army table incomplete)", id)
	}
	army, found := gs.Armies.Items[id]
	if issue := gs.Armies.DecodeErrors[id]; !found || issue != "" {
		return fmt.Sprintf("Army %d (%s)", id, diplomaticRecordIssue(issue))
	}
	switch {
	case army.LeaderAmbiguous:
		return fmt.Sprintf("Army %d (duplicate army.leader field)", id)
	case army.Leader == nil:
		return fmt.Sprintf("Army %d (army.leader not recorded)", id)
	case *army.Leader != leaderID:
		return fmt.Sprintf("Army %d (saved army.leader=%d differs)", id, *army.Leader)
	default:
		return fmt.Sprintf("Army %d (%s, reciprocal saved leader IDs agree)", id, name(army.Name))
	}
}
