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

func registerEspionage(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_espionage", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Read saved spy networks, country espionage_manager references, reciprocal leader location fields, timed modifiers and raw power. Infiltration conversion and modifier duration units remain unverified."), mcp.WithNumber("country_id", mcp.Description("Country ID; omit to select the sole player in a single-player save"))), handleEspionage)
}

func handleEspionage(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderEspionage(gs, id, c), nil
}

func renderEspionage(gs *gamestate.GameState, countryID int, c *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Espionage (%s) ===\n", name(c.Adjective))
	const path = "country.espionage_manager.spy_networks"
	refs := c.SpyNetworkRefs.CountryDiplomaticRefs
	writeCountryDiplomaticRefs(&b, path, refs)
	table := gs.SpyNetworks
	if table.TableIssue || len(table.DecodeErrors) != 0 {
		fmt.Fprintln(&b, "Spy network decoding incomplete. Use raw tree/byte tools for omitted data.")
	} else if !table.Recorded {
		fmt.Fprintln(&b, "Spy network table not recorded.")
	}
	scoped := map[int]bool{}
	for _, id := range refs.Values {
		scoped[id] = true
	}
	for id, n := range table.Items {
		if (n.Owner != nil && *n.Owner == countryID) || (n.Target != nil && *n.Target == countryID) {
			scoped[id] = true
		}
	}
	ids := make([]int, 0, len(scoped))
	for id := range scoped {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		n, ok := table.Items[id]
		if !ok {
			fmt.Fprintf(&b, "\nNetwork ID %d: %s. Selected %s membership: %s\n", id, diplomaticRecordIssue(table.DecodeErrors[id]), path, diplomaticMembership(refs, id))
			continue
		}
		party := func(value *int) string {
			if value == nil {
				return "not recorded"
			}
			return fmt.Sprintf("%s (ID %d)", countryName(gs, *value), *value)
		}
		fmt.Fprintf(&b, "\nNetwork ID %d: owner %s; target %s\n", id, party(n.Owner), party(n.Target))
		fmt.Fprintf(&b, "  Selected %s membership: %s\n", path, diplomaticMembership(refs, id))
		if n.Owner != nil && *n.Owner != countryID {
			if owner, ok := gs.Country[*n.Owner]; ok {
				fmt.Fprintf(&b, "  Saved owner %s membership: %s\n", path, diplomaticMembership(owner.SpyNetworkRefs.CountryDiplomaticRefs, id))
			} else {
				fmt.Fprintln(&b, "  Saved owner country record unavailable.")
			}
			if slices.Contains(refs.Values, id) {
				fmt.Fprintln(&b, "  Selected country's saved reference disagrees with network.owner.")
			}
		}
		if n.Power == nil {
			fmt.Fprintln(&b, "  Network power: not recorded")
		} else {
			fmt.Fprintf(&b, "  Network power (raw save value; infiltration conversion unknown): %s\n", *n.Power)
		}
		if n.Leader != nil {
			if leader, ok := gs.Leaders[*n.Leader]; ok {
				fmt.Fprintf(&b, "  Leader: %s (ID %d)\n", name(leader.Name.FullNames), *n.Leader)
				loc := leader.Location
				switch {
				case leader.LocationAmbiguous:
					fmt.Fprintln(&b, "    Duplicate leader.location fields; reverse reference unresolved.")
				case loc.Type == "spy_network" && loc.IDRaw != nil && *loc.IDRaw == id:
					fmt.Fprintln(&b, "    Reciprocal saved leader.location type and network ID agree.")
				default:
					fmt.Fprintf(&b, "    Saved leader.location type=%s assignment=%s id=%s. Reciprocal network reference unavailable.\n", loc.Type, loc.Assignment, savedInt(loc.IDRaw))
				}
			} else {
				fmt.Fprintf(&b, "  Leader ID: %d (identity unavailable)\n", *n.Leader)
			}
		}
		if n.Formed != "" {
			fmt.Fprintf(&b, "  Formed: %s\n", n.Formed)
		}
		if len(n.ActiveOperations) > 0 {
			fmt.Fprintf(&b, "  Recorded operation IDs: %v\n", n.ActiveOperations)
		}
		writeSpyNetworkTimedModifiers(&b, n.TimedModifiers)
	}
	if len(ids) == 0 {
		fmt.Fprintln(&b, "No matching network records.")
	}
	return mcp.NewToolResultText(b.String())
}

func writeSpyNetworkTimedModifiers(b *strings.Builder, modifiers gamestate.SpyNetworkTimedModifiers) {
	switch {
	case modifiers.Duplicate:
		fmt.Fprintln(b, "  Duplicate timed_modifiers or items fields; use raw tools.")
	case modifiers.Unsupported:
		fmt.Fprintln(b, "  Unsupported timed_modifiers shape; use raw tools.")
	case modifiers.Recorded:
		fmt.Fprintf(b, "  Timed modifiers (saved order): %d\n", len(modifiers.Items))
	}
	for i, modifier := range modifiers.Items {
		switch {
		case modifier.Duplicate:
			fmt.Fprintf(b, "    Item %d: duplicate modifier or days field; use raw tools.\n", i)
		case modifier.Unsupported:
			fmt.Fprintf(b, "    Item %d: unsupported saved shape; use raw tools.\n", i)
		default:
			fmt.Fprintf(b, "    Item %d: modifier=%s days=%s (raw save values)\n", i, governanceValue(modifier.Modifier), governanceValue(modifier.Days))
		}
	}
}

func writeCountryDiplomaticRefs(b *strings.Builder, path string, refs gamestate.CountryDiplomaticRefs) {
	switch {
	case refs.Duplicate:
		fmt.Fprintf(b, "%s has duplicate saved fields. Use raw tools.\n", path)
	case refs.Unsupported:
		fmt.Fprintf(b, "%s has an unsupported saved shape. Use raw tools.\n", path)
	case !refs.Recorded:
		fmt.Fprintf(b, "%s not recorded.\n", path)
	default:
		fmt.Fprintf(b, "%s (saved order): %v\n", path, refs.Values)
	}
}

func diplomaticMembership(refs gamestate.CountryDiplomaticRefs, id int) string {
	switch {
	case refs.Duplicate:
		return "duplicate saved fields"
	case refs.Unsupported:
		return "unsupported saved shape"
	default:
		return armyMembership(refs.Values, id)
	}
}

func diplomaticRecordIssue(issue string) string {
	switch {
	case issue == "":
		return "referenced record unavailable"
	case strings.HasPrefix(issue, "duplicate "):
		return "duplicate saved record ID or fields"
	default:
		return "referenced record has an unsupported saved shape"
	}
}
