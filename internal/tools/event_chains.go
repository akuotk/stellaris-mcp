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

const eventChainPageSize = 20

func registerEventChains(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_event_chains", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read saved event_chain and completed_event_chain entries in country.events order. Values and scope types are not interpreted. Page contexts bind the selected save and country; deep or mixed data remains available through raw tools."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")),
		mcp.WithNumber("offset", mcp.Description("First saved entry, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from first page; required for later pages"))), handleEventChains)
}

func handleEventChains(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	for _, key := range []string{"save_id", "page_context"} {
		if value, ok := req.GetArguments()[key]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	offset, err := rawInteger(req, "offset", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	gs, err := gamestate.LoadFromArchive(archive.zip)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, -1, "get_event_chains", strconv.Itoa(id))
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	if !verified {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	events := country.SpecialProjects
	if offset > len(events.EventChains) {
		return toolError(fmt.Errorf("offset exceeds saved entry count")), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved event-chain entries (country %d) ===\nsave_id=%s\npage_context=%s\n", id, archive.saveID, token)
	fmt.Fprintln(&b, "Field names and values are saved records; scope types, counters, completion semantics and current availability are unverified. Deep or mixed data requires raw tools.")
	switch {
	case events.Unsupported:
		fmt.Fprintln(&b, "country.events unsupported or duplicate. Use raw tools.")
	case !events.Recorded:
		fmt.Fprintln(&b, "country.events not recorded")
	}
	fmt.Fprintf(&b, "saved_entries=%d offset=%d\n", len(events.EventChains), offset)
	rootCounts := indexRootEventTargets(&b, gs.RootEventTargets)
	end := offset + min(eventChainPageSize, len(events.EventChains)-offset)
	for i := offset; i < end; i++ {
		entry := events.EventChains[i]
		writeSavedValue(&b, fmt.Sprintf("entry[%d] key=%q (saved order)", i, entry.Kind), entry.Value, 0)
		writeNestedEventTargets(&b, gs, entry.Value, rootCounts)
	}
	if end < len(events.EventChains) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}

type eventTargetKey struct {
	name, kind string
	id         int
}

func indexRootEventTargets(b *strings.Builder, targets []gamestate.SavedEventTarget) map[eventTargetKey]int {
	counts := map[eventTargetKey]int{}
	unresolved := 0
	for _, target := range targets {
		if key, ok := eventTargetMatchKey(target); ok {
			counts[key]++
		} else {
			unresolved++
		}
	}
	if unresolved > 0 {
		fmt.Fprintf(b, "Root target comparison incomplete: %d unsupported or incomplete records. Use raw tools.\n", unresolved)
	}
	return counts
}

func writeNestedEventTargets(b *strings.Builder, gs *gamestate.GameState, value clausewitz.Value, rootCounts map[eventTargetKey]int) {
	targets, incomplete := gamestate.NestedEventTargets(value)
	if incomplete {
		fmt.Fprintln(b, "  Nested target scan incomplete for mixed data. Use raw tools.")
	}
	for _, nested := range targets {
		writeSavedValue(b, "  Nested saved_event_target "+nested.Path, nested.Target.Value, 0)
		if key, ok := eventTargetMatchKey(nested.Target); ok {
			fmt.Fprintf(b, "    %d root candidate(s) match saved name/type/id; opener and identity unverified.\n", rootCounts[key])
		} else {
			fmt.Fprintln(b, "    Root candidate comparison unresolved: name/type/id missing, unsupported or duplicate.")
		}
		if nested.Target.ID.Value != nil && !nested.Target.ID.Unsupported {
			id := *nested.Target.ID.Value
			var matches []string
			if _, ok := gs.Country[id]; ok {
				matches = append(matches, "country")
			}
			if _, ok := gs.Systems[id]; ok {
				matches = append(matches, "system")
			}
			if _, ok := gs.Planets.Planet[id]; ok {
				matches = append(matches, "planet")
			}
			if _, ok := gs.Ships[id]; ok {
				matches = append(matches, "ship")
			}
			if _, ok := gs.Fleet[id]; ok {
				matches = append(matches, "fleet")
			}
			if len(matches) == 0 {
				fmt.Fprintln(b, "    No numeric candidates in decoded tables; source coverage unverified.")
			} else {
				fmt.Fprintf(b, "    Numeric candidates in decoded tables: %s; namespace, identity and source uniqueness unverified.\n", strings.Join(matches, ", "))
			}
		}
	}
}

func eventTargetMatchKey(target gamestate.SavedEventTarget) (eventTargetKey, bool) {
	if target.ID.Value == nil || target.ID.Unsupported || !target.Type.Recorded || target.Type.Unsupported || !target.Name.Recorded || target.Name.Unsupported {
		return eventTargetKey{}, false
	}
	return eventTargetKey{name: target.Name.Value, kind: target.Type.Value, id: *target.ID.Value}, true
}
