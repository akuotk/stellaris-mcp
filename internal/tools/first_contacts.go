package tools

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerFirstContacts(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_first_contacts", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Read saved first contact records by owner, country or country.first_contact.contacts. Cross-checks the country list with saved owner fields. contact_id selects details and history. Leader/location matches are candidates. Current state, progress units and ETA remain unverified."), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")), mcp.WithNumber("contact_id", mcp.Description("Matching first-contact ID for full details; omit for summary"))), handleFirstContacts)
}

func handleFirstContacts(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["contact_id"]; ok {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			maxExact := int64(1<<53 - 1)
			maxID = int(maxExact)
		}
		contactID, err := rawInteger(req, "contact_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &contactID
	}
	return renderFirstContacts(gs, id, country, selected), nil
}

func renderFirstContacts(gs *gamestate.GameState, countryID int, country *gamestate.Country, selected *int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== First contact records (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope follows saved owner/country IDs and country.first_contact.contacts. Stage, status, timing and progress are saved values. Leader and location namespaces remain unverified.")
	const path = "country.first_contact.contacts"
	refs := country.FirstContactRefs.CountryDiplomaticRefs
	writeCountryDiplomaticRefs(&b, path, refs)
	if gs.FirstContacts.TableIssue || len(gs.FirstContacts.DecodeErrors) > 0 {
		fmt.Fprintln(&b, "First contact decoding incomplete for some records; use raw tree/byte tools for complete data.")
	} else if !gs.FirstContacts.Recorded {
		fmt.Fprintln(&b, "First contact table not recorded.")
	}
	scoped := map[int]bool{}
	for _, id := range refs.Values {
		scoped[id] = true
	}
	for id, c := range gs.FirstContacts.Items {
		if c.Owner != nil && *c.Owner == countryID || c.Country != nil && *c.Country == countryID {
			scoped[id] = true
		}
	}
	ids := make([]int, 0, len(scoped))
	for id := range scoped {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if selected != nil {
		if !scoped[*selected] {
			return toolError(fmt.Errorf("first contact %d does not match selected country references", *selected))
		}
		ids = []int{*selected}
	}
	if len(ids) == 0 {
		fmt.Fprintln(&b, "No matching decoded first contact records.")
	}
	for _, id := range ids {
		c, ok := gs.FirstContacts.Items[id]
		if !ok {
			fmt.Fprintf(&b, "\nFirst contact %d: %s. Selected %s membership: %s\n", id, diplomaticRecordIssue(gs.FirstContacts.DecodeErrors[id]), path, diplomaticMembership(refs, id))
			continue
		}
		fmt.Fprintf(&b, "\nFirst contact %d: %s; owner=%s country=%s stage=%s status=%s\n", id, name(c.Name), armyInt(c.Owner), armyInt(c.Country), operationFlagText(c.Stage), operationFlagText(c.Status))
		fmt.Fprintf(&b, "  Selected %s membership: %s\n", path, diplomaticMembership(refs, id))
		if c.Owner != nil && *c.Owner != countryID {
			if owner, ok := gs.Country[*c.Owner]; ok {
				fmt.Fprintf(&b, "  Saved owner %s membership: %s\n", path, diplomaticMembership(owner.FirstContactRefs.CountryDiplomaticRefs, id))
			}
			if slices.Contains(refs.Values, id) {
				fmt.Fprintln(&b, "  Selected country's saved reference disagrees with first_contact.owner.")
			}
		}
		for _, field := range []struct {
			name string
			id   *int
		}{{"owner", c.Owner}, {"country", c.Country}} {
			if field.id == nil {
				continue
			}
			if candidate, ok := gs.Country[*field.id]; ok {
				fmt.Fprintf(&b, "  %s possible country ID match: %s\n", field.name, name(candidate.Adjective))
			} else {
				fmt.Fprintf(&b, "  %s country reference unresolved (sentinel/deleted-state meaning not inferred)\n", field.name)
			}
		}
		fmt.Fprintf(&b, "  location=%s leader=%s date=%s\n", armyInt(c.Location), armyInt(c.Leader), operationFlagText(c.Date))
		if c.Location != nil {
			if system, ok := gs.Systems[*c.Location]; ok {
				fmt.Fprintf(&b, "    Possible system ID match: %s; location namespace/reverse link unverified\n", name(system.Name))
			} else {
				fmt.Fprintln(&b, "    Location reference unresolved; namespace unverified")
			}
		}
		if c.Leader != nil {
			if leader, ok := gs.Leaders[*c.Leader]; ok {
				fmt.Fprintf(&b, "    Possible leader ID match: %s\n", name(leader.Name.FullNames))
				if leader.LocationAmbiguous {
					fmt.Fprintln(&b, "    Duplicate leader.location fields; use raw tools.")
				} else {
					fmt.Fprintf(&b, "    leader.location (raw): type=%s assignment=%s id=%s\n", leader.Location.Type, leader.Location.Assignment, armyInt(leader.Location.IDRaw))
					fmt.Fprintln(&b, "    No reverse first_contact leader assignment validated; namespace/timing unverified")
					if leader.Location.IDRaw != nil && *leader.Location.IDRaw != id {
						fmt.Fprintln(&b, "    Raw leader.location ID differs from contact ID; namespace/timing unverified")
					}
				}
			} else {
				fmt.Fprintln(&b, "    Leader reference unresolved (sentinel/deleted-state meaning not inferred)")
			}
		}
		if selected == nil {
			continue
		}
		fmt.Fprintf(&b, "  last_roll=%s days_left=%s difficulty=%s clues=%s (raw values)\n", armyFloat(c.LastRoll), armyFloat(c.DaysLeft), armyFloat(c.Difficulty), armyFloat(c.Clues))
		if c.Event == nil {
			fmt.Fprintln(&b, "  event: not recorded")
		} else {
			writeFirstContactEvent(&b, "event", *c.Event)
		}
		if c.Events == nil {
			fmt.Fprintln(&b, "  events: not recorded")
		} else if len(c.Events) == 0 {
			fmt.Fprintln(&b, "  events: empty")
		}
		for index, event := range c.Events {
			writeFirstContactEvent(&b, fmt.Sprintf("events[%d] (saved order)", index), event)
		}
		if c.Completed == nil {
			fmt.Fprintln(&b, "  completed history: not recorded")
		} else if len(c.Completed) == 0 {
			fmt.Fprintln(&b, "  completed history: empty")
		}
		for index, entry := range c.Completed {
			fmt.Fprintf(&b, "  completed[%d] (saved history): date=%s stage=%s\n", index, operationFlagText(entry.Date), operationFlagText(entry.Stage))
		}
		for _, key := range federationSortedKeys(c.Flags) {
			flag := c.Flags[key]
			if flag.Scalar != nil {
				fmt.Fprintf(&b, "  Flag %s=%s\n", key, *flag.Scalar)
			} else {
				fmt.Fprintf(&b, "  Flag %s: flag_date=%s flag_days=%s (as saved)\n", key, operationFlagText(flag.FlagDate), operationFlagText(flag.FlagDays))
			}
		}
	}
	return mcp.NewToolResultText(b.String())
}

func writeFirstContactEvent(b *strings.Builder, label string, event gamestate.ArchaeologyEvent) {
	fmt.Fprintf(b, "  %s: event_id=%s expired=%s picture=%s index=%s player_event=%s\n", label, operationFlagText(event.EventID), operationFlagText(event.Expired), operationFlagText(event.Picture), armyInt(event.Index), operationFlagText(event.PlayerEvent))
	fmt.Fprintf(b, "    owner=%s fleet=%s effect=%s (as saved)\n", armyInt(event.Owner), armyInt(event.Fleet), operationFlagText(event.Effect))
	if event.Scope == nil {
		fmt.Fprintln(b, "    scope: not recorded")
	} else {
		fmt.Fprintf(b, "    scope: preserved opaque object (%d ordered fields); use raw tree tools for complete scope data/meaning\n", len(event.Scope.Value.Object.Pairs))
	}
}
