package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGalacticCommunity(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_galactic_community", mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read saved Galactic Community members, emissaries, resolution lists, last/voting references and timer fields. Pass resolution_id for ordered saved voting rows and community references. Voting codes, timer units and current Senate state remain unverified."),
		mcp.WithNumber("resolution_id", mcp.Description("Saved resolution ID for detailed voting rows"))), handleGalacticCommunity)
}

func handleGalacticCommunity(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	if _, provided := req.GetArguments()["resolution_id"]; provided {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			maxExact := int64(1<<53 - 1)
			maxID = int(maxExact)
		}
		id, err := rawInteger(req, "resolution_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		return renderResolutionDetail(gs, id), nil
	}
	return renderCommunity(gs), nil
}

func renderCommunity(gs *gamestate.GameState) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintln(&b, "=== Galactic community (saved records) ===")
	if gs.Community == nil {
		fmt.Fprintln(&b, "Community data absent from this save.")
		return mcp.NewToolResultText(b.String())
	}
	c := gs.Community
	if c.Unsupported {
		fmt.Fprintln(&b, "Community record unsupported or duplicate. Use raw tree/byte tools.")
		return mcp.NewToolResultText(b.String())
	}
	fmt.Fprintln(&b, "Voting codes, timers and the present Senate state remain unverified.")
	fmt.Fprintf(&b, "Formed: %s\nMembers: %s\n", councilSavedString(c.Formed), communityListStatus(c.Members))
	seen := map[int]bool{}
	for _, id := range c.Members.Values {
		fmt.Fprintf(&b, "  %s\n", communityCountryLabel(gs, id, true, seen[id]))
		seen[id] = true
	}
	writeCommunityEmissaries(&b, gs, c.Emissaries)
	if len(c.SavedFields) > 0 {
		writeSavedValue(&b, "Saved Senate fields", clausewitz.Value{Object: &clausewitz.Object{Pairs: c.SavedFields}}, 0)
	}
	writeSavedTableStatus(&b, "resolution", gs.Resolutions)
	for _, field := range []struct {
		key string
		ref gamestate.OptionalIntRef
	}{{"last", c.Last}, {"voting", c.Voting}} {
		fmt.Fprintf(&b, "  %s=%s (saved reference)\n", field.key, optionalIntRefText(field.ref))
		if field.ref.Value != nil {
			writeResolutionSummary(&b, gs, *field.ref.Value, false)
		}
	}
	for _, group := range communityResolutionLists(c) {
		fmt.Fprintf(&b, "\n%s IDs: %s\n", group.label, communityListStatus(group.refs))
		seen := map[int]bool{}
		for _, id := range group.refs.Values {
			if seen[id] {
				fmt.Fprintf(&b, "  ID %d: duplicate saved reference\n", id)
				continue
			}
			seen[id] = true
			writeResolutionSummary(&b, gs, id, true)
		}
	}
	return mcp.NewToolResultText(b.String())
}

type communityResolutionList struct {
	label string
	refs  gamestate.OptionalIntList
}

func communityResolutionLists(c *gamestate.GalacticCommunity) []communityResolutionList {
	return []communityResolutionList{{"Passed", c.Passed}, {"Proposed", c.Proposed}, {"Failed", c.Failed}}
}

func communityListStatus(refs gamestate.OptionalIntList) string {
	switch {
	case refs.Unsupported:
		return "unsupported or duplicate"
	case !refs.Recorded:
		return "not recorded"
	case len(refs.Values) == 0:
		return "empty"
	default:
		return fmt.Sprintf("%d saved references", len(refs.Values))
	}
}

func communityCountryLabel(gs *gamestate.GameState, id int, idLabel, duplicate bool) string {
	number := strconv.Itoa(id)
	if idLabel {
		number = "ID " + number
	}
	label := fmt.Sprintf("%s (%s)", countryName(gs, id), number)
	if _, found := gs.Country[id]; !found {
		label += " [record unavailable]"
	}
	if duplicate {
		label += " [duplicate saved reference]"
	}
	return label
}

func writeCommunityEmissaries(b *strings.Builder, gs *gamestate.GameState, refs gamestate.OptionalIntList) {
	fmt.Fprintf(b, "Emissaries: %s\n", communityListStatus(refs))
	seen := map[int]bool{}
	for _, id := range refs.Values {
		if seen[id] {
			fmt.Fprintf(b, "  Leader ID %d: duplicate saved emissary reference\n", id)
			continue
		}
		seen[id] = true
		if leader, found := gs.Leaders[id]; found {
			fmt.Fprintf(b, "  Leader ID candidate %d (%s) country=%s\n", id, name(leader.Name.FullNames), optionalIntRefText(leader.CountryRef))
		} else {
			fmt.Fprintf(b, "  Leader ID candidate %d (%s)\n", id, diplomaticRecordIssue(gs.LeaderRecords.DecodeErrors[id]))
		}
	}
}

func writeResolutionSummary(b *strings.Builder, gs *gamestate.GameState, id int, sides bool) {
	r, found := gs.Resolutions.Items[id]
	if !found {
		if issue := gs.Resolutions.DecodeErrors[id]; issue != "" {
			fmt.Fprintf(b, "  ID %d: %s\n", id, diplomaticRecordIssue(issue))
		} else {
			fmt.Fprintf(b, "  ID %d: record unavailable\n", id)
		}
		return
	}
	if r.Type.Recorded && !r.Type.Unsupported {
		fmt.Fprintf(b, "  ID %d: %s [%s]", id, l(r.Type.Value), r.Type.Value)
	} else {
		fmt.Fprintf(b, "  ID %d: type=%s", id, councilSavedString(r.Type))
	}
	if r.Country.Value != nil {
		fmt.Fprintf(b, ", country=%s", communityCountryLabel(gs, *r.Country.Value, false, false))
	} else {
		fmt.Fprintf(b, ", country=%s", optionalIntRefText(r.Country))
	}
	fmt.Fprintf(b, ", pass_date=%s\n", councilSavedString(r.PassDate))
	if !sides {
		return
	}
	for _, side := range []struct {
		label string
		refs  gamestate.OptionalIntList
	}{{"supporters", r.Supporters}, {"opponents", r.Opponents}} {
		if len(side.refs.Values) == 0 {
			fmt.Fprintf(b, "    %s: %s\n", side.label, communityListStatus(side.refs))
			continue
		}
		seen := map[int]bool{}
		var names []string
		for _, cid := range side.refs.Values {
			names = append(names, communityCountryLabel(gs, cid, false, seen[cid]))
			seen[cid] = true
		}
		fmt.Fprintf(b, "    %s: %s\n", side.label, strings.Join(names, ", "))
	}
}

func renderResolutionDetail(gs *gamestate.GameState, id int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved resolution %d ===\n", id)
	fmt.Fprintln(&b, "Voting keys are compared with saved country IDs. The meaning of voting codes remains unverified.")
	writeSavedTableStatus(&b, "resolution", gs.Resolutions)
	writeResolutionSummary(&b, gs, id, true)
	if c := gs.Community; c != nil && !c.Unsupported {
		for _, group := range communityResolutionLists(c) {
			for index, ref := range group.refs.Values {
				if ref == id {
					fmt.Fprintf(&b, "  Saved community %s[%d]=%d\n", strings.ToLower(group.label), index, id)
				}
			}
		}
		if c.Last.Value != nil && *c.Last.Value == id {
			fmt.Fprintf(&b, "  Saved community last=%d\n", id)
		}
		if c.Voting.Value != nil && *c.Voting.Value == id {
			fmt.Fprintf(&b, "  Saved community voting=%d\n", id)
		}
	}
	if r, found := gs.Resolutions.Items[id]; found {
		writeResolutionVoting(&b, gs, r)
	}
	return mcp.NewToolResultText(b.String())
}

func writeResolutionVoting(b *strings.Builder, gs *gamestate.GameState, r gamestate.Resolution) {
	switch {
	case r.VotingUnsupported:
		fmt.Fprintln(b, "Voting rows unsupported or duplicate. Use raw tree/byte tools.")
	case !r.VotingRecorded:
		fmt.Fprintln(b, "Voting rows not recorded")
	case len(r.Votes) == 0:
		fmt.Fprintln(b, "Voting rows empty")
	}
	counts := map[int]int{}
	for _, row := range r.Votes {
		if row.Country != nil {
			counts[*row.Country]++
		}
	}
	for index, row := range r.Votes {
		writeSavedValue(b, fmt.Sprintf("  voting[%d] key=%q", index, row.Key), row.Value, 0)
		switch {
		case row.Country == nil:
			fmt.Fprintln(b, "    Country ID candidate unresolved (unsupported key)")
		case counts[*row.Country] > 1:
			fmt.Fprintln(b, "    Country ID candidate unresolved (duplicate saved numeric key)")
		default:
			fmt.Fprintf(b, "    Country ID candidate: %s\n", communityCountryLabel(gs, *row.Country, true, false))
		}
	}
}
