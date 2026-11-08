package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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
	end := offset + min(eventChainPageSize, len(events.EventChains)-offset)
	for i := offset; i < end; i++ {
		entry := events.EventChains[i]
		writeSavedValue(&b, fmt.Sprintf("entry[%d] key=%q (saved order)", i, entry.Kind), entry.Value, 0)
	}
	if end < len(events.EventChains) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}
