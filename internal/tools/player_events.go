package tools

import (
	"context"
	"fmt"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"strings"
)

func registerPlayerEvents(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_player_events", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through saved root player events and selection history in saved order. Country ID matches are candidates; current UI state and event semantics are unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID; omit for latest")),
		mcp.WithNumber("offset", mcp.Description("First record, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from first page; required for later pages"))), handlePlayerEvents)
}
func handlePlayerEvents(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	for _, key := range []string{"save_id", "page_context"} {
		if v, ok := req.GetArguments()[key]; ok {
			if _, ok := v.(string); !ok {
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
	token := rawPageToken(archive, -1, "get_player_events")
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	if !verified {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	data, err := gamestate.LoadPlayerEventRecordsFromArchive(archive.zip)
	if err != nil {
		return toolError(err), nil
	}
	if offset > len(data.Records) {
		return toolError(fmt.Errorf("offset exceeds saved record count")), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved player events and selection history (save-wide) ===\nsave_id=%s\npage_context=%s\nsaved_records=%d offset=%d\n", archive.saveID, token, len(data.Records), offset)
	fmt.Fprintln(&b, "Saved records do not establish current UI state, event availability, selection meaning or ownership. Country ID matches are candidates only. Deep or mixed data requires raw tools.")
	end := offset + min(20, len(data.Records)-offset)
	for i := offset; i < end; i++ {
		record := data.Records[i]
		writeSavedValue(&b, fmt.Sprintf("record[%d] %s", i, record.Path), record.Value, 0)
		if strings.Contains(record.Path, `key="player_event"`) {
			switch {
			case record.Country.Unsupported:
				fmt.Fprintln(&b, "  Country field unsupported or duplicate; candidate unresolved.")
			case record.Country.Value == nil:
				fmt.Fprintln(&b, "  Country field missing or unsupported; candidate unresolved.")
			case !data.Countries.Recorded:
				fmt.Fprintln(&b, "  Country table not recorded; candidate unresolved.")
			default:
				id := *record.Country.Value
				if _, ok := data.Countries.Items[id]; ok {
					fmt.Fprintln(&b, "  Country ID matches a decoded country record (candidate only).")
				} else {
					fmt.Fprintln(&b, "  No decoded country ID candidate.")
				}
				if data.Countries.TableIssue || data.Countries.DecodeErrors[id] != "" {
					fmt.Fprintln(&b, "  Country table incomplete or record ambiguous; use raw tools.")
				}
			}
		}
	}
	if end < len(data.Records) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}
