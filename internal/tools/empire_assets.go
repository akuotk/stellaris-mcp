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

func registerEmpireAssets(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_empire_assets", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("List this country's saved agreements, subject links, situations and megastructures. Checks agreement owner/target against the saved country.subjects and country.overlord references. Uninterpreted values are labeled raw."), mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save"))), handleEmpireAssets)
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
	ids = ids[:0]
	for k := range gs.Situations.Items {
		ids = append(ids, k)
	}
	sort.Ints(ids)
	count = 0
	fmt.Fprintln(&b, "\nSituations (progress in raw save units):")
	for _, k := range ids {
		v := gs.Situations.Items[k]
		if v.Country == nil || *v.Country != id {
			continue
		}
		count++
		fmt.Fprintf(&b, "  ID %d: %s [%s]; progress=%s, last month=%s, approach=%s\n", k, l(v.Type), v.Type, armyFloat(v.ProgressRaw), armyFloat(v.LastMonthProgressRaw), v.Approach)
	}
	if count == 0 {
		fmt.Fprintln(&b, "  No matching records.")
	}
	ids = ids[:0]
	for k := range gs.Megastructures {
		ids = append(ids, k)
	}
	sort.Ints(ids)
	count = 0
	fmt.Fprintln(&b, "\nMegastructures:")
	for _, k := range ids {
		v := gs.Megastructures[k]
		if v.Owner == nil || *v.Owner != id {
			continue
		}
		count++
		system := "system unavailable"
		if v.Coordinate.Origin != nil {
			system = fmt.Sprintf("system ID %d", *v.Coordinate.Origin)
			if s, ok := gs.Systems[*v.Coordinate.Origin]; ok {
				system = fmt.Sprintf("possible system ID match %s (system ID %d; reverse link unverified)", name(s.Name), *v.Coordinate.Origin)
			}
		}
		fmt.Fprintf(&b, "  ID %d: %s [%s], %s\n", k, l(v.Type), v.Type, system)
	}
	if count == 0 {
		fmt.Fprintln(&b, "  No matching records.")
	}
	return mcp.NewToolResultText(b.String())
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
