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

func registerSavedFlags(s *server.MCPServer, source gamestate.FlagScriptSource) {
	s.AddTool(mcp.NewTool("get_saved_flags", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read saved root flags and their fields in saved order, including duplicates and unusual shapes. Flag names, values and state semantics are unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithBoolean("include_script_references", mcp.Description("Include static literal reference counts from local installed game scripts"), mcp.DefaultBool(false)),
		mcp.WithNumber("offset", mcp.Description("First saved entry, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from first page; required for later pages"))),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handleSavedFlags(ctx, req, source)
		})
}

func handleSavedFlags(_ context.Context, req mcp.CallToolRequest, source gamestate.FlagScriptSource) (*mcp.CallToolResult, error) {
	for _, key := range []string{"save_id", "page_context"} {
		if value, ok := req.GetArguments()[key]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	includeReferences := false
	if value, ok := req.GetArguments()["include_script_references"]; ok {
		var valid bool
		includeReferences, valid = value.(bool)
		if !valid {
			return toolError(fmt.Errorf("include_script_references must be a boolean")), nil
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
	var index gamestate.FlagScriptIndex
	if includeReferences {
		if offset > 0 && req.GetString("page_context", "") == "" {
			return toolError(fmt.Errorf("page_context required for later pages")), nil
		}
		index = gamestate.FlagScriptIndex{Status: "unavailable", Fingerprint: "unavailable"}
		if source != nil {
			index = source()
		}
		token = rawPageToken(archive, -1, "get_saved_flags", "include_script_references", index.Fingerprint)
	}
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
	if includeReferences {
		fmt.Fprintf(&b, "script_reference_status=%s\n", index.Status)
		fmt.Fprintln(&b, "Counts cover indexed static literals in the local installed scripts. Script execution and saved value semantics remain unverified.")
	}
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
		} else {
			writeSavedValue(&b, label, entry.Value, 0)
		}
		if includeReferences && index.Status != "unavailable" && entry.Key != nil {
			for _, ref := range index.References(*entry.Key) {
				fmt.Fprintf(&b, "  %s sites=%d files=%d\n", ref.Command, ref.Sites, ref.Files)
			}
		}
	}
	if end < len(flags.Entries) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}
