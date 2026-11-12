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

const countrySavedFieldsPageSize = 20

func registerCountrySavedFields(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_country_saved_fields", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("List the selected country's immediate saved fields in source order, including unknown fields, duplicate keys and anonymous entries. Values have bounded previews, absolute read_save_tree paths and exact uncompressed source byte ranges. Use the returned save_id, revision and entry_index with the raw save tools for full values."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games. Omit for latest.")),
		mcp.WithNumber("country_id", mcp.Description("Country ID. If omitted, every saved player record must refer to the same unique country.")),
		mcp.WithString("query", mcp.Description("Literal substring of immediate keys after Unicode lowercasing, with no Unicode normalization or full case folding. Omit or pass an empty string to include all entries.")),
		mcp.WithNumber("offset", mcp.Description("Zero-based offset among matching entries. Up to 20 per page.")),
		mcp.WithString("page_context", mcp.Description("Token required for later pages. Continue with the same save, country and query."))), handleCountrySavedFields)
}

func handleCountrySavedFields(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := requireStringArgs(req, "save_id", "query", "page_context"); err != nil {
		return toolError(err), nil
	}
	var requestedCountry *int
	if _, ok := req.GetArguments()["country_id"]; ok {
		id, err := rawInteger(req, "country_id", 0, 0, maxRecordID)
		if err != nil {
			return toolError(err), nil
		}
		requestedCountry = &id
	}
	offset, err := rawInteger(req, "offset", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	if offset > 0 && req.GetString("page_context", "") == "" {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(fmt.Errorf("cannot open selected save")), nil
	}
	country, err := gamestate.LoadCountrySavedFieldsFromArchive(ctx, archive.zip, requestedCountry)
	if err != nil {
		return toolError(err), nil
	}
	query := req.GetString("query", "")
	token := rawPageToken(archive, country.EntryIndex, "get_country_saved_fields", strconv.Itoa(country.CountryID), query)
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}

	counts := make(map[string]int)
	for _, child := range country.Node.Children {
		if child.Key != nil {
			counts[child.Key.Text]++
		}
	}
	fields := make([]map[string]any, 0, countrySavedFieldsPageSize)
	lowerQuery := strings.ToLower(query)
	matches := 0
	for i, child := range country.Node.Children {
		if err := ctx.Err(); err != nil {
			return toolError(err), nil
		}
		if lowerQuery != "" && (child.Key == nil || !strings.Contains(strings.ToLower(child.Key.Text), lowerQuery)) {
			continue
		}
		if matches >= offset && len(fields) < countrySavedFieldsPageSize {
			_, key := rawChild(country.Node, i)
			field := rawNode(country.Path+"/"+strconv.Itoa(i), key, child)
			field["child_index"], field["anonymous"] = i, key == nil
			if key != nil {
				field["key_occurrences"] = counts[*key]
			}
			fields = append(fields, field)
		}
		matches++
	}
	if offset > matches {
		return toolError(fmt.Errorf("offset exceeds matched field count")), nil
	}
	var next *int
	if end := offset + len(fields); end < matches {
		next = &end
	}
	result := rawTreePage(archive, country.EntryIndex)
	result["country_id"], result["country_path"], result["country_kind"] = country.CountryID, country.Path, country.Node.Kind
	result["fields"], result["field_count"], result["match_count"] = fields, len(country.Node.Children), matches
	result["offset"], result["next_offset"], result["page_context"] = offset, next, token
	return rawJSON(result)
}
