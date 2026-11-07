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

func registerCouncil(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_council", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read the selected country's saved council positions, leader references, agenda, raw progress, unlocked slots and repeated cooldown records. Shows reciprocal position/leader ID checks and missing or conflicting references. Current assignments, agenda progress units, cooldown effects and positional type alignment remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player"))), handleCouncil)
}

func handleCouncil(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderCouncil(gs, id, country), nil
}

func renderCouncil(gs *gamestate.GameState, countryID int, country *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved council and agenda (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope uses saved position.country or government.council_positions. Current assignments, progress units, cooldown effects and type-list alignment remain unverified.")
	gov := country.CouncilGovernment
	var positionRefs []int
	var pickedTypes []string
	switch {
	case gov.Unsupported:
		fmt.Fprintln(&b, "Government shape unsupported or duplicate. Government council references are unavailable. Use raw tree/byte tools.")
	case !gov.Recorded:
		fmt.Fprintln(&b, "country.government not recorded")
	default:
		positionRefs = country.Government.CouncilPositions
		pickedTypes = country.Government.PickedCouncilTypes
		fmt.Fprintf(&b, "council_agenda=%s council_agenda_progress=%s unlocked_civic_council_slots=%s (saved values)\n", councilSavedString(gov.Agenda), councilSavedString(gov.Progress), councilSavedString(gov.UnlockedSlots))
		if gov.Cooldowns == nil {
			fmt.Fprintln(&b, "council_agenda_cooldowns not recorded")
		}
		for index, cooldown := range gov.Cooldowns {
			if cooldown.Unsupported {
				fmt.Fprintf(&b, "  Cooldown record %d unsupported or duplicate. Use raw tree/byte tools.\n", index)
				continue
			}
			fmt.Fprintf(&b, "  Cooldown record %d (saved order) council_agenda=%s start_date=%s\n", index, councilSavedString(cooldown.Agenda), councilSavedString(cooldown.StartDate))
		}
		fmt.Fprintf(&b, "government.council_positions (saved order)=%s\n", colonyReferenceList(positionRefs))
		if pickedTypes == nil {
			fmt.Fprintln(&b, "government.picked_council_types not recorded")
		} else {
			fmt.Fprintf(&b, "government.picked_council_types (saved order)=%q\n", pickedTypes)
		}
		if positionRefs != nil && pickedTypes != nil && len(positionRefs) != len(pickedTypes) {
			fmt.Fprintln(&b, "Saved council position and type lists have different lengths. Same-index candidates are unavailable.")
		}
	}
	if !gs.CouncilPositions.Recorded {
		fmt.Fprintln(&b, "Council position table not recorded")
	}
	if gs.CouncilPositions.TableIssue || len(gs.CouncilPositions.DecodeErrors) > 0 {
		fmt.Fprintln(&b, "Council position decoding incomplete. Unsupported or duplicate records require raw tree/byte tools.")
	}
	positions := map[int]bool{}
	for _, id := range positionRefs {
		positions[id] = true
	}
	for id, position := range gs.CouncilPositions.Items {
		if position.Country != nil && *position.Country == countryID {
			positions[id] = true
		}
	}
	ids := make([]int, 0, len(positions))
	for id := range positions {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if len(ids) == 0 {
		fmt.Fprintln(&b, "No matching decoded council positions or country references")
	}
	for _, id := range ids {
		position, ok := gs.CouncilPositions.Items[id]
		if !ok {
			if _, bad := gs.CouncilPositions.DecodeErrors[id]; bad {
				fmt.Fprintf(&b, "\nPosition %d referenced record unsupported or duplicate. Use raw tools.\n", id)
			} else {
				fmt.Fprintf(&b, "\nPosition %d referenced record unavailable\n", id)
			}
			continue
		}
		membership := armyMembership(positionRefs, id)
		if gov.Unsupported {
			membership = "unsupported (use raw tools)"
		}
		fmt.Fprintf(&b, "\nPosition %d country=%s type=%s leader=%s government membership=%s\n", id, armyInt(position.Country), governanceValue(position.Type), armyInt(position.Leader), membership)
		if position.Country != nil && *position.Country != countryID {
			fmt.Fprintln(&b, "  Saved position.country differs from selected country")
		}
		if len(positionRefs) == len(pickedTypes) {
			for index, ref := range positionRefs {
				if ref != id {
					continue
				}
				fmt.Fprintf(&b, "  Same-index type candidate picked_council_types[%d]=%q", index, pickedTypes[index])
				if position.Type != nil && pickedTypes[index] != *position.Type {
					fmt.Fprint(&b, " differs from saved position.type")
				}
				fmt.Fprintln(&b)
			}
		}
		writeCouncilLeaderReferences(&b, gs, country, id, position)
	}
	return mcp.NewToolResultText(b.String())
}

func writeCouncilLeaderReferences(b *strings.Builder, gs *gamestate.GameState, country *gamestate.Country, positionID int, position gamestate.CouncilPosition) {
	if position.Leader != nil {
		leaderID := *position.Leader
		fmt.Fprintf(b, "  Leader ID %d candidate. country.owned_leaders membership=%s\n", leaderID, armyMembership(country.OwnedLeaders, leaderID))
		if leader, ok := gs.Leaders[leaderID]; ok {
			fmt.Fprintf(b, "    %s country=%s council_location.type=%q id=%s position=%s\n", name(leader.Name.FullNames), armyInt(leader.CountryID), leader.CouncilLocation.Type, armyInt(leader.CouncilLocation.IDRaw), armyInt(leader.CouncilLocation.PositionRaw))
			if position.Country != nil && leader.CountryID != nil && *position.Country != *leader.CountryID {
				fmt.Fprintln(b, "    Saved leader.country differs from position.country")
			}
			switch {
			case leader.CouncilLocation.Type == "" || leader.CouncilLocation.IDRaw == nil:
				fmt.Fprintln(b, "    Leader council reverse reference incomplete")
			case leader.CouncilLocation.Type != "council_position" || *leader.CouncilLocation.IDRaw != positionID:
				fmt.Fprintln(b, "    Position and leader council references disagree")
			default:
				fmt.Fprintln(b, "    Reciprocal saved position/leader IDs agree")
			}
		} else {
			fmt.Fprintln(b, "    Referenced leader record unavailable")
		}
	}
	reverse := []int{}
	for id, leader := range gs.Leaders {
		location := leader.CouncilLocation
		if location.Type == "council_position" && location.IDRaw != nil && *location.IDRaw == positionID && (position.Leader == nil || *position.Leader != id) {
			reverse = append(reverse, id)
		}
	}
	sort.Ints(reverse)
	for _, id := range reverse {
		fmt.Fprintf(b, "  Leader %d council_location points here. Saved position.leader=%s\n", id, armyInt(position.Leader))
	}
}

func councilSavedString(field gamestate.OptionalString) string {
	if field.Unsupported {
		return "unsupported or duplicate (use raw tools)"
	}
	if !field.Recorded {
		return "not recorded"
	}
	return strconv.Quote(field.Value)
}
