package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGalaxyMetadata(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_galaxy_metadata", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read root galaxy records in saved order, including duplicate fields. Values may describe configuration or state; their meanings, units and current applicability are not inferred. Deep or mixed structures are available through raw tools."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest"))), handleGalaxyMetadata)
}

func handleGalaxyMetadata(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := requireStringArgs(req, "save_id"); err != nil {
		return toolError(err), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	records, err := gamestate.LoadGalaxyRecordsFromArchive(archive.zip)
	if err != nil {
		return toolError(err), nil
	}
	var b strings.Builder
	fmt.Fprintln(&b, "=== Saved galaxy records ===")
	fmt.Fprintln(&b, "Saved values may describe configuration or state; meanings, units and current applicability are unverified. Deep or mixed data is available through raw tree/byte tools.")
	if len(records) == 0 {
		fmt.Fprintln(&b, "galaxy not recorded")
	}
	if len(records) > 1 {
		fmt.Fprintln(&b, "Multiple root galaxy records; relationships are unverified.")
	}
	for index, record := range records {
		writeSavedValue(&b, fmt.Sprintf("galaxy[%d] (saved order)", index), record, 0)
	}
	return mcp.NewToolResultText(b.String()), nil
}
