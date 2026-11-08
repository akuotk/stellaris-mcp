package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const eventTargetPageSize = 20

func registerSavedEventTargets(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_saved_event_targets", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read save-wide root saved_event_target records in saved order. Numeric IDs are compared with country, system, planet, ship and fleet tables only as candidates; scope type, ID namespace and ownership remain unverified. Nested event targets require raw tools."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("offset", mcp.Description("First saved record, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from first page; required for later pages"))), handleSavedEventTargets)
}

func handleSavedEventTargets(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	token := rawPageToken(archive, -1, "get_saved_event_targets")
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	if !verified {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	data, err := gamestate.LoadEventTargetsFromArchive(archive.zip)
	if err != nil {
		return toolError(err), nil
	}
	if offset > len(data.Records) {
		return toolError(fmt.Errorf("offset exceeds saved record count")), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Root saved event targets (save-wide) ===\nsave_id=%s\npage_context=%s\n", archive.saveID, token)
	fmt.Fprintln(&b, "Numeric matches are candidates only; scope types, namespaces, ownership and nested event targets remain unverified. Deep or mixed data requires raw tools.")
	fmt.Fprintf(&b, "saved_records=%d offset=%d\n", len(data.Records), offset)
	idCounts := map[int]int{}
	for _, target := range data.Records {
		if target.ID.Value != nil {
			idCounts[*target.ID.Value]++
		}
	}
	tables := []struct {
		name  string
		table gamestate.ConnectionRecordTable[gamestate.SpaceObjectCandidate]
	}{{"country", data.Countries}, {"system", data.Systems}, {"planet", data.Planets}, {"ship", data.Ships}, {"fleet", data.Fleets}}
	end := offset + min(eventTargetPageSize, len(data.Records)-offset)
	for i := offset; i < end; i++ {
		target := data.Records[i]
		writeSavedValue(&b, fmt.Sprintf("target[%d] (saved order)", i), target.Value, 0)
		if !target.Value.IsObject() {
			fmt.Fprintln(&b, "  Target shape unsupported; use raw tools.")
			continue
		}
		if target.ID.Unsupported {
			fmt.Fprintln(&b, "  ID unsupported or duplicate; numeric candidates unresolved.")
			continue
		}
		if target.ID.Value == nil {
			fmt.Fprintln(&b, "  ID not recorded; numeric candidates unresolved.")
			continue
		}
		if count := idCounts[*target.ID.Value]; count > 1 {
			fmt.Fprintf(&b, "  Unqualified numeric ID occurs in %d root records; namespace unverified.\n", count)
		}
		if target.Type.Unsupported || target.Name.Unsupported {
			fmt.Fprintln(&b, "  Type or name unsupported or duplicate; identity unverified.")
		}
		var matches []string
		for _, entry := range tables {
			if !entry.table.Recorded {
				fmt.Fprintf(&b, "  %s candidate table not recorded.\n", entry.name)
				continue
			}
			if _, ok := entry.table.Items[*target.ID.Value]; ok {
				matches = append(matches, entry.name)
			}
			if entry.table.TableIssue || entry.table.DecodeErrors[*target.ID.Value] != "" {
				fmt.Fprintf(&b, "  %s candidate table incomplete or record ambiguous; use raw tools.\n", entry.name)
			}
		}
		if len(matches) == 0 {
			fmt.Fprintln(&b, "  No decoded numeric ID candidates")
		} else {
			fmt.Fprintf(&b, "  Numeric ID candidates: %s (namespace unverified)\n", strings.Join(matches, ", "))
		}
	}
	if end < len(data.Records) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}
