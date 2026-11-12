package tools

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSaveRecordRefs(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_save_record_refs", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Find all direct children of a selected saved table whose keys represent the supplied nonnegative decimal integer. Preserves order, duplicate spellings, none and unsupported values. Use returned paths or byte ranges with the raw save tools for full values."),
		mcp.WithString("revision", mcp.Required(), mcp.Description("Archive SHA-256 from inspect_save_archive or the raw tree. Stale revisions are rejected.")),
		mcp.WithString("table_path", mcp.Required(), mcp.Description("Absolute positional path from read_save_tree. An empty string selects the entry root.")),
		mcp.WithNumber("record_id", mcp.Required(), mcp.Min(0), mcp.Max(float64(maxSaveRecordID)),
			func(schema map[string]any) { schema["type"] = "integer" }, mcp.Description("Nonnegative exact JSON integer. Zero is valid.")),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games. Omit for latest.")),
		mcp.WithNumber("entry_index", mcp.DefaultNumber(-1), mcp.Description("ZIP entry index from inspect_save_archive. Default -1 selects the unique gamestate.")),
		mcp.WithNumber("offset", mcp.DefaultNumber(0), mcp.Description("Zero-based offset among matching rows.")),
		mcp.WithNumber("limit", mcp.DefaultNumber(50), mcp.Description("Matches per page, 1 to 200.")),
		mcp.WithString("page_context", mcp.Description("Token required for later pages. Continue with the same source, table and record ID."))), handleSaveRecordRefs)
}

func handleSaveRecordRefs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	path, ok := args["table_path"].(string)
	if !ok {
		return toolError(fmt.Errorf("table_path is required and must be a positional path string")), nil
	}
	if _, ok := args["record_id"]; !ok {
		return toolError(fmt.Errorf("record_id is required")), nil
	}
	id, err := rawInteger(req, "record_id", 0, 0, maxRecordID)
	if err != nil {
		return toolError(err), nil
	}
	offset, err := rawInteger(req, "offset", 0, 0, math.MaxInt32)
	if err != nil {
		return toolError(err), nil
	}
	limit, err := rawInteger(req, "limit", 50, 1, 200)
	if err != nil {
		return toolError(err), nil
	}
	if offset > 0 && req.GetString("page_context", "") == "" {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	archive, index, node, key, err := rawTreeAtPath(ctx, req, path)
	if err != nil {
		return toolError(err), nil
	}
	want := strconv.Itoa(id)
	token := rawPageToken(archive, index, "get_save_record_refs", path, want)
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}

	table := rawNode(path, key, node)
	counts := map[string]int{
		"numeric_key_count": 0, "unaddressable_numeric_key_count": 0, "nonnumeric_key_count": 0, "anonymous_count": 0,
		"object_count": 0, "none_count": 0, "empty_block_count": 0, "unsupported_value_count": 0,
		"duplicate_numeric_key_count": 0, "duplicate_numeric_extra_count": 0,
	}
	seen := make(map[string]int)
	maxKey := strconv.FormatInt(maxSaveRecordID, 10)
	candidates := make([]map[string]any, 0, limit)
	matches := 0
	matchedValueStatus := ""
	for i, child := range node.Children {
		if err := ctx.Err(); err != nil {
			return toolError(err), nil
		}
		if child.Key == nil {
			counts["anonymous_count"]++
			continue
		}
		numericKey, ok := normalizeSaveRecordKey(child.Key.Text)
		if !ok {
			counts["nonnumeric_key_count"]++
			continue
		}
		counts["numeric_key_count"]++
		if len(numericKey) > len(maxKey) || (len(numericKey) == len(maxKey) && numericKey > maxKey) {
			counts["unaddressable_numeric_key_count"]++
		}
		seen[numericKey]++
		if seen[numericKey] == 2 {
			counts["duplicate_numeric_key_count"]++
		}
		if seen[numericKey] > 1 {
			counts["duplicate_numeric_extra_count"]++
		}
		valueStatus, countKey := saveRecordValueStatus(child)
		counts[countKey]++
		if numericKey != want {
			continue
		}
		if matches >= offset && len(candidates) < limit {
			row := rawNode(path+"/"+strconv.Itoa(i), &child.Key.Text, child)
			row["child_index"], row["record_status"] = i, valueStatus
			candidates = append(candidates, row)
		}
		matchedValueStatus = valueStatus
		matches++
	}
	if offset > matches {
		return toolError(fmt.Errorf("offset exceeds matching record count")), nil
	}

	tableStatus := "unsupported"
	switch node.Kind {
	case "object", "empty_block":
		tableStatus = "complete"
		if counts["nonnumeric_key_count"] > 0 {
			tableStatus = "partial"
		}
	case "mixed":
		tableStatus = "partial"
	}
	lookup := "table-unsupported"
	switch {
	case matches > 1:
		lookup = "duplicate"
	case matches == 1 && tableStatus == "partial":
		lookup = "observed"
	case matches == 1:
		lookup = matchedValueStatus
		if lookup == "object" {
			lookup = "unique"
		}
	case tableStatus == "complete":
		lookup = "missing"
	case tableStatus == "partial":
		lookup = "unresolved"
	}
	var next *int
	if end := offset + len(candidates); end < matches {
		next = &end
	}
	table["status"], table["counts"] = tableStatus, counts
	result := rawTreePage(archive, index)
	result["table"], result["record_id"], result["lookup_status"] = table, id, lookup
	if lookup == "observed" {
		result["observed_record_status"] = matchedValueStatus
	}
	result["candidates"], result["match_count"] = candidates, matches
	result["offset"], result["next_offset"], result["page_context"] = offset, next, token
	return rawJSON(result)
}

// Normalize decimal keys without narrowing them to the platform's integer range.
func normalizeSaveRecordKey(key string) (string, bool) {
	digits := key
	if len(digits) > 0 && (digits[0] == '+' || digits[0] == '-') {
		digits = digits[1:]
	}
	if digits == "" {
		return "", false
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return "", false
		}
	}
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return "0", true
	}
	return digits, key[0] != '-'
}

func saveRecordValueStatus(node *clausewitz.RawNode) (status, countKey string) {
	switch node.Kind {
	case "object":
		return "object", "object_count"
	case "empty_block":
		return "empty-block", "empty_block_count"
	case "scalar":
		if node.Scalar.Text == "none" && !node.Scalar.Quoted {
			return "none", "none_count"
		}
	}
	return "unsupported-record", "unsupported_value_count"
}
