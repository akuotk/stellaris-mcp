package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const spaceObjectPageSize = 20

func registerSpaceCombatObjects(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_space_combat_objects", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read paginated saved missile or strike_craft records in saved order. Scalar none entries are counted separately. Ownership, target type, activity and units are not inferred; deep or mixed values remain available through raw tools."),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Saved table: missile or strike_craft")),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("offset", mcp.Description("First non-none saved record, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from first page; required for later pages"))), handleSpaceCombatObjects)
}

func handleSpaceCombatObjects(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := requireStringArgs(req, "kind", "save_id", "page_context"); err != nil {
		return toolError(err), nil
	}
	kind := req.GetString("kind", "")
	if kind != "missile" && kind != "strike_craft" {
		return toolError(fmt.Errorf("kind must be missile or strike_craft")), nil
	}
	offset, err := rawInteger(req, "offset", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, -1, "get_space_combat_objects", kind)
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	if !verified {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	data, err := gamestate.LoadSpaceObjectsFromArchive(archive.zip, kind)
	if err != nil {
		return toolError(err), nil
	}
	if offset > len(data.Records) {
		return toolError(fmt.Errorf("offset exceeds saved record count")), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved %s records ===\nsave_id=%s\npage_context=%s\n", kind, archive.saveID, token)
	fmt.Fprintln(&b, "Saved values do not establish current activity, ownership, target type or units. Deep or mixed fields require raw tools.")
	switch {
	case !data.Recorded:
		fmt.Fprintln(&b, "Table not recorded")
	case data.Unsupported:
		fmt.Fprintln(&b, "Table unsupported or duplicate. Use raw tools.")
	}
	fmt.Fprintf(&b, "none_entries=%d non_none_entries=%d offset=%d\n", data.NoneCount, len(data.Records), offset)
	end := offset + min(spaceObjectPageSize, len(data.Records)-offset)
	for i := offset; i < end; i++ {
		record := data.Records[i]
		label := fmt.Sprintf("record[%d] key=%q", i, record.Key)
		if record.ID == nil {
			label += " (non-numeric key; identity unresolved)"
		} else if data.NumericKeyCounts[*record.ID] > 1 {
			label += " (duplicate numeric ID; identity ambiguous)"
		}
		writeSavedValue(&b, label, record.Value, 0)
		for _, ref := range []struct {
			label string
			value gamestate.OptionalIntRef
			table gamestate.ConnectionRecordTable[gamestate.SpaceObjectCandidate]
		}{
			{"fleet", record.Fleet, data.Fleets}, {"position.origin", record.PositionOrigin, data.Systems},
		} {
			if !ref.value.Recorded {
				continue
			}
			if ref.value.Unsupported || ref.value.Value == nil {
				fmt.Fprintf(&b, "  %s reference unsupported or duplicate\n", ref.label)
				continue
			}
			id := *ref.value.Value
			if _, ok := ref.table.Items[id]; ok {
				fmt.Fprintf(&b, "  %s=%d: saved table ID candidate exists; relationship unverified\n", ref.label, id)
			} else {
				fmt.Fprintf(&b, "  %s=%d: %s\n", ref.label, id, diplomaticRecordIssue(ref.table.DecodeErrors[id]))
			}
			if ref.table.TableIssue {
				fmt.Fprintln(&b, "  Candidate table incomplete or duplicate. Use raw tools.")
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
