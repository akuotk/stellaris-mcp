package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSavedMessages(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_saved_messages", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read saved root message records in order, including duplicates and unusual shapes. Shows saved fields with unverified reference and read-state semantics."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("offset", mcp.Description("First record, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from first page; required for later pages"))), handleSavedMessages)
}

func handleSavedMessages(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	token := rawPageToken(archive, -1, "get_saved_messages")
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	if !verified {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	messages, err := gamestate.LoadSavedMessagesFromArchive(archive.zip)
	if err != nil {
		return toolError(err), nil
	}
	if offset > len(messages) {
		return toolError(fmt.Errorf("offset exceeds saved record count")), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved root messages (save-wide) ===\nsave_id=%s\npage_context=%s\nsaved_records=%d offset=%d\n", archive.saveID, token, len(messages), offset)
	fmt.Fprintln(&b, "Message field meanings, references, read status and current UI visibility are unverified. Use raw tools for deep or mixed data.")
	end := offset + min(20, len(messages)-offset)
	for i := offset; i < end; i++ {
		message := messages[i]
		writeSavedValue(&b, fmt.Sprintf("record[%d] root[%d] key=%q", i, message.RootIndex, "message"), message.Value, 0)
	}
	if end < len(messages) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}
