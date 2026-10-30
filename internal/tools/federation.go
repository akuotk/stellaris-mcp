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

func registerFederations(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_federations", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("federation_id", mcp.Description("Optional federation ID; omit to show all recorded federations")), mcp.WithDescription("Show saved federation membership, leadership, progression, laws, perks and other recorded details")), handleFederations)
}

func handleFederations(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	if _, provided := req.GetArguments()["federation_id"]; provided {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			exactMax := int64(1<<53 - 1)
			maxID = int(exactMax)
		}
		id, err := rawInteger(req, "federation_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		f, ok := gs.Federation[id]
		if !ok {
			return toolError(fmt.Errorf("federation %d not found", id)), nil
		}
		filtered := *gs
		filtered.Federation = map[int]gamestate.Federation{id: f}
		gs = &filtered
	}
	var b strings.Builder
	writeFederations(&b, gs)
	return mcp.NewToolResultText(b.String()), nil
}

func writeFederations(b *strings.Builder, gs *gamestate.GameState) {
	fmt.Fprint(b, "\nFederations:\n")
	if len(gs.Federation) == 0 {
		fmt.Fprintln(b, "  No federation records.")
		return
	}
	ids := make([]int, 0, len(gs.Federation))
	for id := range gs.Federation {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		f := gs.Federation[id]
		p := f.FederationProgression
		fmt.Fprintf(b, "  - ID %d: %s, type=%s\n", id, name(f.Name), p.FederationType)
		if f.Leader == nil {
			fmt.Fprintln(b, "    Leader: not recorded")
		} else {
			fmt.Fprintf(b, "    Leader: %s (ID %d)\n", countryName(gs, *f.Leader), *f.Leader)
		}
		fmt.Fprintf(b, "    Members: %s\n    Associates: %s\n", federationCountries(gs, f.Members), federationCountries(gs, f.Associates))
		if f.StartDate != "" {
			fmt.Fprintf(b, "    Started: %s\n", f.StartDate)
		}
		fmt.Fprintf(b, "    Progress (raw save values): experience=%s base_cohesion=%s cohesion=%s\n", federationNumber(p.Experience), federationNumber(p.BaseCohesion), federationNumber(p.Cohesion))
		if p.Levels != nil {
			fmt.Fprintf(b, "    levels (as saved): %d\n", *p.Levels)
		}
		for _, entry := range []struct{ label, value string }{{"succession_type", p.SuccessionType}, {"succession_term", p.SuccessionTerm}, {"last_succession_date", p.LastSuccessionDate}, {"expired", p.Expired}} {
			if entry.value != "" {
				fmt.Fprintf(b, "    %s: %s\n", entry.label, entry.value)
			}
		}
		for _, entry := range []struct {
			label string
			value *bool
		}{{"only_leader_builds_fleets", p.OnlyLeaderBuildsFleets}, {"allow_subjects_to_join", p.AllowSubjectsToJoin}, {"equal_voting_power", p.EqualVotingPower}, {"free_migration", p.FreeMigration}, {"research_agreement", p.ResearchAgreement}, {"failed_vote_half_price", p.FailedVoteHalfPrice}, {"research_sharing", p.ResearchSharing}} {
			if entry.value != nil {
				fmt.Fprintf(b, "    %s: %t\n", entry.label, *entry.value)
			}
		}
		for _, key := range federationSortedKeys(p.Laws) {
			fmt.Fprintf(b, "    Law %s: %s\n", key, p.Laws[key])
		}
		for _, key := range federationSortedKeys(p.ActionSettings) {
			a := p.ActionSettings[key]
			fmt.Fprintf(b, "    Action %s: vote_type=%s acceptance_type=%s\n", key, a.VoteType, a.AcceptanceType)
		}
		if len(p.Envoy) > 0 {
			fmt.Fprintf(b, "    Envoy references (as saved): %v\n", p.Envoy)
		}
		for _, perk := range p.Perks {
			fmt.Fprintf(b, "    Perk: level=%s type=%s\n", federationInteger(perk.Level), perk.Type)
		}
		for _, modifier := range p.TimedModifier.Items {
			fmt.Fprintf(b, "    Modifier: %s days=%s multiplier=%s\n", modifier.Modifier, federationInteger(modifier.Days), federationNumber(modifier.Multiplier))
		}
		for _, key := range federationSortedKeys(p.Flags) {
			fmt.Fprintf(b, "    Flag %s: %s\n", key, p.Flags[key])
		}
		if len(f.ShipDesignCollection.ShipDesign) > 0 {
			fmt.Fprintf(b, "    Ship design references: %v\n", f.ShipDesignCollection.ShipDesign)
		}
		for _, cooldown := range f.ProposedLawVotesCooldowns {
			fmt.Fprintf(b, "    Law vote cooldown: token=%s target=%s start_date=%s\n", cooldown.Token, cooldown.Target, cooldown.StartDate)
		}
	}
}

func federationNumber(value *float64) string {
	if value == nil {
		return "not recorded"
	}
	return fmt.Sprintf("%g", *value)
}

func federationCountries(gs *gamestate.GameState, ids []int) string {
	if len(ids) == 0 {
		return "none recorded"
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, fmt.Sprintf("%s (ID %d)", countryName(gs, id), id))
	}
	return strings.Join(names, ", ")
}

func federationSortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func federationInteger(value *int) string {
	if value == nil {
		return "not recorded"
	}
	return fmt.Sprintf("%d", *value)
}
