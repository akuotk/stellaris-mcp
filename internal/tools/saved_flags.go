package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const savedFlagPageSize = 20

func registerSavedFlags(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_saved_flags", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read saved root flags and their fields in saved order, including duplicates and unusual shapes. Flag names, values and state semantics are unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("offset", mcp.Description("First saved entry, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from first page; required for later pages"))), handleSavedFlags)
}

func handleSavedFlags(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	token := rawPageToken(archive, -1, "get_saved_flags")
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	if !verified {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	flags, err := gamestate.LoadSavedFlagsFromArchive(archive.zip)
	if err != nil {
		return toolError(err), nil
	}
	if offset > len(flags.Entries) {
		return toolError(fmt.Errorf("offset exceeds saved entry count")), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved root flags (save-wide) ===\nsave_id=%s\npage_context=%s\nroot_occurrences=%d saved_entries=%d offset=%d\n", archive.saveID, token, flags.Roots, len(flags.Entries), offset)
	fmt.Fprintln(&b, "Flag names, numeric units, current state and clearing conditions remain unverified. Use raw tools for deep or mixed values.")
	end := offset + min(savedFlagPageSize, len(flags.Entries)-offset)
	for i := offset; i < end; i++ {
		entry := flags.Entries[i]
		label := fmt.Sprintf("entry[%d] %s", i, entry.Path)
		if entry.RepeatedKey {
			label += " [duplicate saved key]"
		}
		if entry.UnsupportedRoot {
			label += " (unsupported root shape; use raw tools)"
		}
		if !galaxyRegionPreviewFits(entry.Value, 0) {
			fmt.Fprintf(&b, "%s: large saved block; use raw tree/byte tools\n", label)
			continue
		}
		writeSavedValue(&b, label, entry.Value, 0)
	}
	if end < len(flags.Entries) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}
