package tools

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerEmpireAssets(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_empire_assets", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("List this country's saved agreements, subject links, situations and megastructures. Cross-checks saved country and system references. Shows situation stages, targets and flags, plus megastructure upgrades, dismantling, planet and bypass references. Uninterpreted values are labeled raw."), mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save"))), handleEmpireAssets)
}

func handleEmpireAssets(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderEmpireAssets(gs, id, c), nil
}

func renderEmpireAssets(gs *gamestate.GameState, id int, c *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Agreements, situations and megastructures (%s) ===\n", name(c.Adjective))
	ids := make([]int, 0, len(gs.Agreements.Items))
	for k := range gs.Agreements.Items {
		ids = append(ids, k)
	}
	sort.Ints(ids)
	fmt.Fprintln(&b, "\nAgreements (owner/target as recorded; roles not inferred):")
	incomplete := gs.Agreements.TableIssue || len(gs.Agreements.DecodeErrors) != 0
	if incomplete {
		fmt.Fprintln(&b, "  Agreement table has unsupported or duplicate data; use raw tree/byte tools for omitted entries.")
	} else if !gs.Agreements.Recorded {
		fmt.Fprintln(&b, "  Agreement table not recorded.")
	}
	count := 0
	for _, k := range ids {
		a := gs.Agreements.Items[k]
		if (a.Owner == nil || *a.Owner != id) && (a.Target == nil || *a.Target != id) {
			continue
		}
		count++
		fmt.Fprintf(&b, "  ID %d: owner %s, target %s; status=%s, preset=%s, added=%s\n", k, assetCountryReference(gs, a.Owner), assetCountryReference(gs, a.Target), a.Status, a.Terms.Preset, a.DateAdded)
		fmt.Fprintf(&b, "    Saved cross-check: owner country.subjects=%s; target country.overlord=%s\n", agreementSubjectMembership(gs, a), agreementOverlordMatch(gs, a))
		if a.DateChanged != "" {
			fmt.Fprintf(&b, "    date_changed=%s\n", a.DateChanged)
		}
		for _, field := range []struct {
			key   string
			value *string
		}{
			{"can_subject_be_integrated", a.Terms.CanIntegrate}, {"can_subject_do_diplomacy", a.Terms.CanDiplomacy}, {"can_subject_vote", a.Terms.CanVote}, {"has_access", a.Terms.HasAccess}, {"has_sensors", a.Terms.HasSensors}, {"joins_overlord_wars", a.Terms.JoinsOverlordWars}, {"calls_overlord_to_war", a.Terms.CallsOverlordToWar}, {"subject_expansion_type", a.Terms.Expansion},
			{"specialization_level", a.Specialization.Level}, {"specialization_experience", a.Specialization.Experience}, {"conversion_progress", a.Specialization.Conversion.Progress}, {"conversion_in_progress", a.Specialization.Conversion.InProgress}, {"conversion_done", a.Specialization.Conversion.Done},
		} {
			if field.value != nil {
				fmt.Fprintf(&b, "    %s=%s\n", field.key, *field.value)
			}
		}
		for _, term := range a.Terms.Discrete {
			fmt.Fprintf(&b, "    discrete_term %s=%s\n", term.Key, term.Value)
		}
		for _, term := range a.Terms.Resources {
			fmt.Fprintf(&b, "    resource_term %s=%s (raw save value)\n", term.Key, term.Value)
		}
		for _, field := range []struct {
			key   string
			value *string
		}{
			{"forced_initial_loyalty", a.Terms.ForcedInitialLoyalty},
			{"has_cooldown_on_first_renegotiation", a.Terms.FirstRenegotiationHasCooldown},
			{"subject_conversion_process.ignore", a.Specialization.Conversion.Ignore},
		} {
			if field.value != nil {
				fmt.Fprintf(&b, "    %s=%s (raw save value)\n", field.key, *field.value)
			}
		}
	}
	if count == 0 && gs.Agreements.Recorded && !incomplete {
		fmt.Fprintln(&b, "  No matching records.")
	}
	writeCountrySubjectLinks(&b, gs, c)
	writeSituations(&b, gs, id, c)
	writeMegastructures(&b, gs, id, c.OwnedMegastructureRefs)
	return mcp.NewToolResultText(b.String())
}

func writeSituations(b *strings.Builder, gs *gamestate.GameState, countryID int, country *gamestate.Country) {
	fmt.Fprintln(b, "\nSituations (progress in raw save units):")
	fmt.Fprintln(b, "  Scope follows situation.country and country.events.situations. Target links require saved type=planet. Stage duration units and opener_id meaning remain unverified.")
	fmt.Fprintf(b, "  country.track_all_situations=%s (saved field)\n", councilSavedString(country.TrackAllSituations))
	table := gs.Situations
	switch {
	case table.TableIssue || len(table.DecodeErrors) != 0:
		fmt.Fprintln(b, "  Situation table has unsupported or duplicate data; use raw tree/byte tools for omitted entries.")
	case !table.Recorded:
		fmt.Fprintln(b, "  Situation table not recorded.")
	}
	events := country.SpecialProjects
	refs := events.SituationRefs
	switch {
	case events.Unsupported || refs.Unsupported:
		fmt.Fprintln(b, "  country.events.situations unsupported or duplicate; use raw tree/byte tools.")
	case !refs.Recorded:
		fmt.Fprintln(b, "  country.events.situations not recorded.")
	default:
		fmt.Fprintf(b, "  country.events.situations (saved order)=%v\n", refs.Values)
	}
	scoped := map[int]bool{}
	for _, id := range refs.Values {
		scoped[id] = true
	}
	for id, situation := range table.Items {
		if situation.Country != nil && *situation.Country == countryID {
			scoped[id] = true
		}
	}
	if len(scoped) == 0 {
		fmt.Fprintln(b, "  No matching decoded records or country references.")
		return
	}
	ids := make([]int, 0, len(scoped))
	for id := range scoped {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	foreignRefs := map[int][]int{}
	for id, c := range gs.Country {
		if id == countryID {
			continue
		}
		for _, ref := range c.SpecialProjects.SituationRefs.Values {
			if scoped[ref] {
				foreignRefs[ref] = append(foreignRefs[ref], id)
			}
		}
	}
	for _, id := range ids {
		situation, ok := table.Items[id]
		membership := armyMembership(refs.Values, id)
		if events.Unsupported || refs.Unsupported {
			membership = "unsupported or duplicate"
		}
		if !ok {
			status := "referenced record unavailable"
			if _, bad := table.DecodeErrors[id]; bad {
				status = "referenced record unsupported or duplicate; use raw tools"
			}
			fmt.Fprintf(b, "  ID %d: %s; country.events.situations membership=%s\n", id, status, membership)
			continue
		}
		fmt.Fprintf(b, "  ID %d: %s [%s]; progress=%s, last month=%s, approach=%s\n", id, l(situation.Type), situation.Type, operationFlagText(situation.ProgressRaw), operationFlagText(situation.LastMonthProgressRaw), operationFlagText(situation.Approach))
		fmt.Fprintf(b, "    situation.country=%s; country.events.situations membership=%s\n", assetCountryReference(gs, situation.Country), membership)
		if situation.Country != nil && *situation.Country != countryID {
			fmt.Fprintln(b, "    Saved situation.country differs from selected country.")
		}
		if others := foreignRefs[id]; len(others) != 0 {
			sort.Ints(others)
			fmt.Fprintf(b, "    Other countries' saved situation references (country IDs, including repeated entries)=%v\n", others)
		}
		writeSituationDetails(b, gs, situation)
	}
}

func writeSituationDetails(b *strings.Builder, gs *gamestate.GameState, situation gamestate.Situation) {
	var details strings.Builder
	seen := map[string]bool{}
	for _, pair := range situation.SavedDetails {
		label := pair.Key
		if seen[label] {
			label += " [duplicate saved key]"
		}
		seen[pair.Key] = true
		writeSavedValue(&details, label, pair.Value, 0)
	}
	for line := range strings.SplitSeq(details.String(), "\n") {
		if line != "" {
			fmt.Fprintf(b, "    %s\n", line)
		}
	}
	target := situation.PlanetTarget
	switch {
	case target.Unsupported:
		fmt.Fprintln(b, "    Situation target unsupported or duplicate; use raw tree/byte tools.")
	case target.Recorded && target.Value == nil:
		fmt.Fprintln(b, "    Planet target ID not recorded.")
	case target.Value != nil:
		id := *target.Value
		if planet, ok := gs.Planets.Planet[id]; ok {
			fmt.Fprintf(b, "    Saved planet target %d: %s\n", id, name(planet.Name))
		} else {
			fmt.Fprintf(b, "    Saved planet target %d: planet record unavailable.\n", id)
		}
	}
}

func writeMegastructures(b *strings.Builder, gs *gamestate.GameState, countryID int, countryRefs gamestate.OptionalIntList) {
	fmt.Fprintln(b, "\nMegastructures (saved owner or country.owned_megastructures):")
	table := gs.Megastructures
	switch {
	case table.TableIssue || len(table.DecodeErrors) != 0:
		fmt.Fprintln(b, "  Megastructure table has unsupported or duplicate data; use raw tree/byte tools for omitted entries.")
	case !table.Recorded:
		fmt.Fprintln(b, "  Megastructure table not recorded.")
	}
	if countryRefs.Unsupported {
		fmt.Fprintln(b, "  country.owned_megastructures unsupported or duplicate; use raw tools.")
	}
	scoped := map[int]bool{}
	for _, id := range countryRefs.Values {
		scoped[id] = true
	}
	for id, m := range table.Items {
		if m.Owner != nil && *m.Owner == countryID {
			scoped[id] = true
		}
	}
	if len(scoped) == 0 {
		fmt.Fprintln(b, "  No matching decoded records or country references.")
		return
	}
	ids := make([]int, 0, len(scoped))
	for id := range scoped {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	systemRefs := map[int][]int{}
	unsupportedSystems := 0
	for systemID, system := range gs.Systems {
		if system.MegastructureRefs.Unsupported {
			unsupportedSystems++
		}
		for _, id := range system.MegastructureRefs.Values {
			if scoped[id] {
				systemRefs[id] = append(systemRefs[id], systemID)
			}
		}
	}
	if unsupportedSystems != 0 {
		fmt.Fprintf(b, "  System reverse-reference survey incomplete: unsupported galactic_object.megastructures fields=%d.\n", unsupportedSystems)
	}
	for _, id := range ids {
		m, ok := table.Items[id]
		membership := armyMembership(countryRefs.Values, id)
		if countryRefs.Unsupported {
			membership = "unsupported or duplicate"
		}
		if !ok {
			status := "referenced record unavailable"
			if _, bad := table.DecodeErrors[id]; bad {
				status = "referenced record unsupported or duplicate; use raw tools"
			}
			fmt.Fprintf(b, "  ID %d: %s; country.owned_megastructures membership=%s\n", id, status, membership)
			continue
		}
		origin := m.Coordinate.Origin
		system, systemOK := gamestate.GalacticSystem{}, false
		if origin != nil {
			system, systemOK = gs.Systems[*origin]
		}
		refs := systemRefs[id]
		sort.Ints(refs)
		fmt.Fprintf(b, "  ID %d: %s [%s], %s\n", id, l(m.Type), m.Type, megastructureSystemLink(origin, system, systemOK, id))
		fmt.Fprintf(b, "    owner=%s; country.owned_megastructures membership=%s\n", assetCountryReference(gs, m.Owner), membership)
		if m.Owner != nil && *m.Owner != countryID {
			fmt.Fprintln(b, "    Saved owner differs from selected country.")
		}
		if len(refs) != 0 {
			fmt.Fprintf(b, "    galactic_object.megastructures reverse system IDs=%v\n", refs)
			uniqueRefs := slices.Compact(refs)
			if len(uniqueRefs) < len(refs) {
				fmt.Fprintln(b, "    Duplicate system membership entries recorded.")
			}
			if len(uniqueRefs) > 1 || (origin != nil && uniqueRefs[0] != *origin) {
				fmt.Fprintln(b, "    Saved system references disagree or span multiple systems.")
			}
		}
		writeMegastructureDetails(b, gs, id, m, system, systemOK)
	}
}

func megastructureSystemLink(origin *int, system gamestate.GalacticSystem, systemOK bool, id int) string {
	if origin == nil {
		return "coordinate.origin not recorded"
	}
	if !systemOK {
		return fmt.Sprintf("coordinate.origin=%d (system record unavailable)", *origin)
	}
	if !system.MegastructureRefs.Recorded {
		return fmt.Sprintf("possible system ID match %s (system ID %d; reverse link unverified)", name(system.Name), *origin)
	}
	membership := armyMembership(system.MegastructureRefs.Values, id)
	if system.MegastructureRefs.Unsupported {
		membership = "unsupported or duplicate"
	}
	return fmt.Sprintf("%s (system ID %d; coordinate.origin and galactic_object.megastructures membership=%s)", name(system.Name), *origin, membership)
}

func writeMegastructureDetails(b *strings.Builder, gs *gamestate.GameState, id int, m gamestate.Megastructure, system gamestate.GalacticSystem, systemOK bool) {
	c := m.Coordinate
	fmt.Fprintf(b, "    coordinate.x=%s coordinate.y=%s coordinate.randomized=%s coordinate.visual_height=%s (raw save values)\n", governanceValue(c.X), governanceValue(c.Y), governanceValue(c.Randomized), governanceValue(c.VisualHeight))
	fmt.Fprintf(b, "    dismantle_progress=%s dismantle_finish_date=%s graphical_culture=%s (raw save values)\n", governanceValue(m.DismantleProgress), governanceValue(m.DismantleFinishDate), governanceValue(m.GraphicalCulture))
	if u := m.Upgrade; u != nil {
		fmt.Fprintf(b, "    upgrade.upgrade_to=%s upgrade.progress=%s upgrade.halted=%s upgrade.indefinitely_halted=%s (raw save values)\n", governanceValue(u.To), governanceValue(u.Progress), governanceValue(u.Halted), governanceValue(u.IndefinitelyHalted))
	}
	if m.Planet != nil {
		planet, found := gs.Planets.Planet[*m.Planet]
		fmt.Fprintf(b, "    planet=%d", *m.Planet)
		if found {
			fmt.Fprintf(b, " (planet record %s)", name(planet.Name))
		} else {
			fmt.Fprint(b, " (planet record unavailable; raw value)")
		}
		if systemOK {
			fmt.Fprintf(b, "; coordinate.origin system planet membership=%s", armyMembership(system.Planets, *m.Planet))
		}
		fmt.Fprintln(b)
	}
	if m.Bypass != nil {
		fmt.Fprintf(b, "    bypass=%d", *m.Bypass)
		if bypass, found := gs.Bypasses.Items[*m.Bypass]; found {
			fmt.Fprintf(b, "; bypass.type=%s", governanceValue(bypass.Type))
			if owner := bypass.Owner; owner != nil {
				fmt.Fprintf(b, "; bypass.owner.type=%s bypass.owner.id=%s (owner namespace unverified)", governanceValue(owner.Type), savedInt(owner.ID))
				if owner.ID != nil {
					if *owner.ID == id {
						fmt.Fprint(b, "; saved owner ID equals megastructure ID")
					} else {
						fmt.Fprint(b, "; saved owner ID differs from megastructure ID")
					}
				}
			} else {
				fmt.Fprint(b, "; bypass.owner not recorded")
			}
		} else {
			fmt.Fprint(b, " (bypass record unavailable)")
		}
		fmt.Fprintln(b)
	}
	if m.BuildQueue != nil {
		fmt.Fprintf(b, "    build_queue=%s (raw save value; queue reference unverified)\n", *m.BuildQueue)
	}
}

func writeCountrySubjectLinks(b *strings.Builder, gs *gamestate.GameState, c *gamestate.Country) {
	fmt.Fprintln(b, "\nCountry subject links (direct saved fields):")
	overlord := assetCountryReference(gs, c.OverlordRef.Value)
	if c.OverlordRef.Unsupported {
		overlord = "unsupported or duplicate (use raw tools)"
	}
	fmt.Fprintf(b, "  country.overlord=%s\n", overlord)
	fmt.Fprintf(b, "  country.subject_date=%s\n", councilSavedString(c.SubjectDate))
	switch {
	case c.SubjectRefs.Unsupported:
		fmt.Fprintln(b, "  country.subjects=unsupported or duplicate (use raw tools)")
	case !c.SubjectRefs.Recorded:
		fmt.Fprintln(b, "  country.subjects=not recorded")
	case len(c.SubjectRefs.Values) == 0:
		fmt.Fprintln(b, "  country.subjects=empty")
	default:
		fmt.Fprintln(b, "  country.subjects (saved order):")
		for _, id := range c.SubjectRefs.Values {
			fmt.Fprintf(b, "    %s\n", assetCountryReference(gs, &id))
		}
	}
}

func agreementSubjectMembership(gs *gamestate.GameState, a gamestate.Agreement) string {
	if a.Owner == nil || a.Target == nil {
		return "agreement owner or target not recorded"
	}
	owner, ok := gs.Country[*a.Owner]
	if !ok {
		return "owner country record unavailable"
	}
	if owner.SubjectRefs.Unsupported {
		return "unsupported or duplicate (use raw tools)"
	}
	return armyMembership(owner.SubjectRefs.Values, *a.Target)
}

func agreementOverlordMatch(gs *gamestate.GameState, a gamestate.Agreement) string {
	if a.Owner == nil || a.Target == nil {
		return "agreement owner or target not recorded"
	}
	target, ok := gs.Country[*a.Target]
	if !ok {
		return "target country record unavailable"
	}
	switch ref := target.OverlordRef; {
	case ref.Unsupported:
		return "unsupported or duplicate (use raw tools)"
	case ref.Value == nil:
		return "not recorded"
	case *ref.Value == *a.Owner:
		return fmt.Sprintf("matches owner (%d)", *ref.Value)
	default:
		return fmt.Sprintf("saved %d differs from owner", *ref.Value)
	}
}

func assetCountryReference(gs *gamestate.GameState, id *int) string {
	if id == nil {
		return "not recorded"
	}
	if _, ok := gs.Country[*id]; !ok {
		return fmt.Sprintf("%d (country record unavailable)", *id)
	}
	return fmt.Sprintf("%s (%d)", countryName(gs, *id), *id)
}
