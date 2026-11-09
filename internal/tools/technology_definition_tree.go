package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTechnologyDefinitionTree(s *server.MCPServer, source gamestate.TechnologyDefinitionSource) {
	s.AddTool(mcp.NewTool("read_technology_definition", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read ordered nodes or normalized key/scalar bytes from a local technology definition. Original script source bytes are unavailable."),
		mcp.WithString("technology_id", mcp.Required(), mcp.Description("Exact installed technology key")),
		mcp.WithNumber("definition_index", mcp.Description("Zero-based definition position; required for multiple matches")),
		mcp.WithString("path", mcp.Description("Empty for the definition root, or positional child indexes such as /4/0")),
		mcp.WithString("operation", mcp.Description("tree (default) or scalar")),
		mcp.WithString("part", mcp.Description("For scalar operation, value (default) or key")),
		mcp.WithNumber("offset", mcp.Description("Zero-based child or normalized byte offset")),
		mcp.WithNumber("limit", mcp.Description("Child page size 1–200 (default 50), or byte page size 1–65536 (default 8192)")),
		mcp.WithString("page_context", mcp.Description("Token from the first page of the same query; required for later pages"))),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handleTechnologyDefinitionTree(ctx, req, source)
		})
}

func handleTechnologyDefinitionTree(_ context.Context, req mcp.CallToolRequest, source gamestate.TechnologyDefinitionSource) (*mcp.CallToolResult, error) {
	for _, key := range []string{"technology_id", "path", "operation", "part", "page_context"} {
		if value, ok := req.GetArguments()[key]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	id := req.GetString("technology_id", "")
	if strings.TrimSpace(id) == "" || len(id) > gamestate.MaxTechnologyKeyBytes {
		return toolError(fmt.Errorf("technology_id must be a nonempty key of at most %d bytes", gamestate.MaxTechnologyKeyBytes)), nil
	}
	path := req.GetString("path", "")
	operation := req.GetString("operation", "tree")
	part := ""
	defaultLimit, maxLimit := 50, 200
	switch operation {
	case "tree":
		if _, ok := req.GetArguments()["part"]; ok {
			return toolError(fmt.Errorf("part requires operation=scalar")), nil
		}
	case "scalar":
		part = req.GetString("part", "value")
		if part != "value" && part != "key" {
			return toolError(fmt.Errorf("part must be value or key")), nil
		}
		defaultLimit, maxLimit = 8192, 65536
	default:
		return toolError(fmt.Errorf("operation must be tree or scalar")), nil
	}
	offset, err := rawInteger(req, "offset", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	if offset > 0 && req.GetString("page_context", "") == "" {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	limit, err := rawInteger(req, "limit", defaultLimit, 1, maxLimit)
	if err != nil {
		return toolError(err), nil
	}
	definitionIndex, err := rawInteger(req, "definition_index", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	_, indexProvided := req.GetArguments()["definition_index"]
	index := gamestate.TechnologyDefinitionIndex{Status: "unavailable", Fingerprint: "unavailable"}
	if source != nil {
		index = source()
	}
	token := rawPageToken(&rawSaveArchive{}, definitionIndex, "read_technology_definition", index.Fingerprint, id, path, operation, part)
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	definitions := index.Definitions(id)
	matchStatus := technologyDefinitionMatch(index.Status, len(definitions))
	result := map[string]any{
		"technology_id": id, "source_status": index.Status, "match_status": matchStatus, "definition_matches": len(definitions),
		"exact_source_bytes_available": false,
		"representation":               "Ordered parsed Clausewitz nodes and normalized key/value text. Original script bytes, comments and whitespace are unavailable.",
	}
	if len(definitions) == 0 {
		if offset != 0 {
			return toolError(fmt.Errorf("offset requires a retained definition")), nil
		}
		result["status"] = matchStatus
		return rawJSON(result)
	}
	if len(definitions) > 1 && !indexProvided {
		if offset != 0 {
			return toolError(fmt.Errorf("definition_index required for multiple matches")), nil
		}
		result["status"] = "ambiguous"
		result["message"] = "Specify a zero-based definition_index to select an observed definition."
		return rawJSON(result)
	}
	if definitionIndex >= len(definitions) {
		return toolError(fmt.Errorf("definition_index exceeds the observed definition count")), nil
	}
	definition := definitions[definitionIndex]
	result["definition_index"], result["definition_source"] = definitionIndex, definition.Source
	if definition.Value == nil {
		if offset != 0 {
			return toolError(fmt.Errorf("offset requires a retained definition")), nil
		}
		result["status"] = "definition-not-retained"
		result["message"] = "This definition exceeded the local index retention limits."
		return rawJSON(result)
	}
	node, key, err := rawAtPath(definition.Value, path)
	if err != nil {
		return toolError(err), nil
	}
	if path == "" && node.Key != nil {
		key = &node.Key.Text
	}
	total := rawChildCount(node)
	var text string
	if operation == "scalar" {
		if part == "key" {
			if key == nil {
				return toolError(fmt.Errorf("selected node has no key")), nil
			}
			text = *key
		} else {
			if !node.IsScalar() {
				return toolError(fmt.Errorf("path must select a scalar value")), nil
			}
			text = node.Scalar.Text
		}
		total = len(text)
	}
	if offset > total {
		return toolError(fmt.Errorf("offset exceeds selected node %s size", operation)), nil
	}
	end := offset + min(limit, total-offset)
	var next *int
	if end < total {
		next = &end
	}
	result["status"], result["operation"], result["path"] = "ok", operation, path
	result["offset"], result["next_offset"], result["page_context"] = offset, next, token
	if operation == "tree" {
		children := make([]map[string]any, 0, end-offset)
		for i := offset; i < end; i++ {
			child, childKey := rawChild(node, i)
			entry := technologyTreeNode(path+"/"+strconv.Itoa(i), childKey, child)
			entry["child_index"] = i
			children = append(children, entry)
		}
		result["node"], result["children"], result["total_children"] = technologyTreeNode(path, key, node), children, total
	} else {
		result["part"], result["encoding"], result["total_bytes"], result["returned_bytes"] = part, "base64", total, end-offset
		result["data"] = base64.StdEncoding.EncodeToString([]byte(text[offset:end]))
	}
	return rawJSON(result)
}

func technologyTreeNode(path string, key *string, value *clausewitz.RawNode) map[string]any {
	node := rawNode(path, key, value)
	for _, field := range []string{
		"entry_source_offset", "entry_source_bytes", "value_source_offset", "value_source_bytes", "key_source_offset", "key_source_bytes",
	} {
		delete(node, field)
	}
	return node
}
