package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerResearchPotential(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_research_potential", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithString("revision", mcp.Description("Archive revision returned by first page; required for later pages")), mcp.WithString("page_context", mcp.Description("Context token returned by first page; required for later pages")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit for the sole player")), mcp.WithString("query", mcp.Description("Optional case-insensitive technology key filter")), mcp.WithNumber("offset", mcp.DefaultNumber(0)), mcp.WithNumber("limit", mcp.Description("Records per page, 1..200"), mcp.DefaultNumber(50)), mcp.WithDescription("Return saved research potential entries in their original order. Values are raw internal fields, not inferred research options or probabilities.")), handleResearchPotential)
}

func handleResearchPotential(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	for _, key := range []string{"query", "save_id", "revision", "page_context"} {
		if v, ok := req.GetArguments()[key]; ok {
			if _, ok := v.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	offset, err := rawInteger(req, "offset", 0, 0, math.MaxInt32)
	if err != nil {
		return toolError(err), nil
	}
	limit, err := rawInteger(req, "limit", 50, 1, 200)
	if err != nil {
		return toolError(err), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	revision := req.GetString("revision", "")
	if offset > 0 && revision == "" {
		return toolError(fmt.Errorf("archive revision required for later pages")), nil
	}
	if revision != "" && revision != archive.revision {
		return toolError(fmt.Errorf("selected save changed; restart pagination")), nil
	}
	var data []byte
	found := false
	for _, f := range archive.zip.File {
		if f.Name != "gamestate" {
			continue
		}
		if found {
			return toolError(fmt.Errorf("multiple gamestate entries")), nil
		}
		found = true
		stream, err := f.Open()
		if err != nil {
			return toolError(err), nil
		}
		data, err = io.ReadAll(stream)
		stream.Close()
		if err != nil {
			return toolError(err), nil
		}
	}
	if !found {
		return toolError(fmt.Errorf("no gamestate entry found")), nil
	}
	gs, err := gamestate.Load(data)
	if err != nil {
		return toolError(err), nil
	}
	id, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	query := strings.ToLower(req.GetString("query", ""))
	contextHash := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%s", archive.saveID, archive.revision, id, query)))
	contextToken := hex.EncodeToString(contextHash[:])
	suppliedContext := req.GetString("page_context", "")
	if offset > 0 && suppliedContext == "" {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	if suppliedContext != "" && suppliedContext != contextToken {
		return toolError(fmt.Errorf("country, query or save changed; restart pagination")), nil
	}
	entries := make([]map[string]any, 0)
	for _, v := range c.TechStatus.Potential {
		if !strings.Contains(strings.ToLower(v.Key), query) {
			continue
		}
		entries = append(entries, map[string]any{"key": v.Key, "name": l(v.Key), "kind": v.Kind, "raw_value": v.Value})
	}
	if offset > len(entries) {
		return toolError(fmt.Errorf("offset exceeds matching entries")), nil
	}
	end := offset + min(limit, len(entries)-offset)
	var next *int
	if end < len(entries) {
		next = &end
	}
	return rawJSON(map[string]any{"save_id": archive.saveID, "revision": archive.revision, "page_context": contextToken, "revision_algorithm": "sha256_archive_bytes", "country_id": id, "potential_blocks": c.TechStatus.PotentialBlocks, "unsupported_blocks": c.TechStatus.PotentialUnsupported, "entries": entries[offset:end], "total_matches": len(entries), "offset": offset, "next_offset": next, "note": "Raw saved potential values; their interpretation as availability or probabilities is not established. Non-scalar entries require raw tree inspection."})
}
