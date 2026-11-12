package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSaveCatalog(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("list_save_games", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("List available local save files in newest-first order. Supply save_id to analysis or raw save tools to select one; omit it for latest. Catalog revision is required on subsequent pages."), mcp.WithString("revision", mcp.Description("Catalog revision returned by the first page")), mcp.WithNumber("offset", mcp.DefaultNumber(0)), mcp.WithNumber("limit", mcp.Description("Entries per page, 1..200"), mcp.DefaultNumber(50))), handleSaveCatalog)
}

func handleSaveCatalog(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	files, err := gamestate.ListSaveFiles(saveDir)
	if err != nil {
		return toolError(err), nil
	}
	offset, err := rawInteger(req, "offset", 0, 0, len(files))
	if err != nil {
		return toolError(err), nil
	}
	limit, err := rawInteger(req, "limit", 50, 1, 200)
	if err != nil {
		return toolError(err), nil
	}
	if err := requireStringArgs(req, "revision"); err != nil {
		return toolError(err), nil
	}
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00%s\x00%d\x00", f.ID, f.Modified.UTC().Format(time.RFC3339Nano), f.Size)
	}
	revision := hex.EncodeToString(h.Sum(nil))
	supplied := req.GetString("revision", "")
	if offset > 0 && supplied == "" {
		return toolError(fmt.Errorf("catalog revision required for subsequent pages")), nil
	}
	if supplied != "" && supplied != revision {
		return toolError(fmt.Errorf("save catalog changed; restart pagination")), nil
	}
	end := offset + min(limit, len(files)-offset)
	entries := make([]map[string]any, 0, end-offset)
	for _, f := range files[offset:end] {
		entries = append(entries, map[string]any{"save_id": f.ID, "relative_path": f.RelativePath, "size_bytes": f.Size, "modified_utc": f.Modified.UTC().Format(time.RFC3339Nano)})
	}
	var next *int
	if end < len(files) {
		next = &end
	}
	return rawJSON(map[string]any{"revision": revision, "revision_algorithm": "sha256_catalog_metadata", "count": len(files), "offset": offset, "next_offset": next, "saves": entries})
}
