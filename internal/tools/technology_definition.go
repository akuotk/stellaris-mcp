package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const technologyDefinitionPageSize = 20

func registerTechnologyDefinition(s *server.MCPServer, source gamestate.TechnologyDefinitionSource) {
	s.AddTool(mcp.NewTool("get_technology_definition", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through static local definitions for an exact technology key. Optionally count its exact matches in one saved country's research records."),
		mcp.WithString("technology_id", mcp.Required(), mcp.Description("Exact saved or installed technology key")),
		mcp.WithBoolean("include_save_status", mcp.Description("Include exact matches in a selected save"), mcp.DefaultBool(false)),
		mcp.WithString("save_id", mcp.Description("Save ID when including save status; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID when including save status. If omitted, all saved player records must refer to one country.")),
		mcp.WithNumber("offset", mcp.Description("First definition field, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from the first page; required for later pages"))),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handleTechnologyDefinition(ctx, req, source)
		})
}

func handleTechnologyDefinition(_ context.Context, req mcp.CallToolRequest, source gamestate.TechnologyDefinitionSource) (*mcp.CallToolResult, error) {
	for _, key := range []string{"technology_id", "save_id", "page_context"} {
		if value, ok := req.GetArguments()[key]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	technologyID := req.GetString("technology_id", "")
	if strings.TrimSpace(technologyID) == "" || len(technologyID) > gamestate.MaxTechnologyKeyBytes {
		return toolError(fmt.Errorf("technology_id must be a nonempty key of at most %d bytes", gamestate.MaxTechnologyKeyBytes)), nil
	}
	includeSaveStatus := false
	if value, ok := req.GetArguments()["include_save_status"]; ok {
		var valid bool
		includeSaveStatus, valid = value.(bool)
		if !valid {
			return toolError(fmt.Errorf("include_save_status must be a boolean")), nil
		}
	}
	_, hasSaveID := req.GetArguments()["save_id"]
	_, hasCountryID := req.GetArguments()["country_id"]
	if !includeSaveStatus && (hasSaveID || hasCountryID) {
		return toolError(fmt.Errorf("save_id and country_id require include_save_status")), nil
	}
	var countryID *int
	selector := "static"
	if includeSaveStatus {
		selector = "sole-player"
		if hasCountryID {
			maxID := int(min(int64(^uint(0)>>1), int64(9007199254740991)))
			id, err := rawInteger(req, "country_id", 0, 0, maxID)
			if err != nil {
				return toolError(err), nil
			}
			countryID = &id
			selector = strconv.Itoa(id)
		}
	}
	offset, err := rawInteger(req, "offset", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	if offset > 0 && req.GetString("page_context", "") == "" {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	archive := &rawSaveArchive{}
	if includeSaveStatus {
		archive, err = openRawSave(saveDir, req.GetString("save_id", ""))
		if err != nil {
			return toolError(fmt.Errorf("cannot open selected save")), nil
		}
	}
	index := gamestate.TechnologyDefinitionIndex{Status: "unavailable", Fingerprint: "unavailable"}
	if source != nil {
		index = source()
	}
	token := rawPageToken(archive, -1, "get_technology_definition", index.Fingerprint, technologyID, selector)
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	definitions := index.Definitions(technologyID)
	rows := technologyDefinitionRows(definitions)
	if offset > len(rows) {
		return toolError(fmt.Errorf("offset exceeds definition field count")), nil
	}
	var saved []gamestate.TechnologySaveMatch
	if includeSaveStatus {
		saved, err = gamestate.LoadTechnologySaveMatches(archive.zip, countryID, technologyID)
		if err != nil {
			return toolError(err), nil
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Static local technology definition ===\ntechnology_id=%q\npage_context=%s\n", technologyID, token)
	fmt.Fprintf(&b, "source_status=%s match_status=%s definition_matches=%d fields=%d offset=%d\n",
		index.Status, technologyDefinitionMatch(index.Status, len(definitions)), len(definitions), len(rows), offset)
	fmt.Fprintln(&b, "Raw static fields from installed common/technology and DLC ZIP text. DLC activation, MOD overrides, research availability, prerequisite satisfaction and applied effects remain unverified.")
	end := offset + min(technologyDefinitionPageSize, len(rows)-offset)
	for i := offset; i < end; i++ {
		row := rows[i]
		label := fmt.Sprintf("entry[%d] definition[%d] source=%s", i, row.definition, row.source)
		if row.field >= 0 {
			label += fmt.Sprintf(" field[%d] key=%s", row.field, technologyTokenText(row.value.Key))
			if row.duplicate {
				label += " [duplicate field]"
			}
		} else if row.value == nil || row.value.Kind != "empty_block" {
			label += " (unsupported definition)"
		}
		previewEntries := technologyDefinitionPageSize
		writeTechnologyRaw(&b, label, row.value, 0, &previewEntries)
	}
	if end < len(rows) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	if includeSaveStatus {
		fmt.Fprintf(&b, "\n=== Saved research key matches ===\nsave_id=%s\n", archive.saveID)
		for _, match := range saved {
			switch {
			case match.Partial:
				fmt.Fprintf(&b, "  %s partial observed_matches=%d", match.Field, match.Count)
			case !match.Recorded:
				fmt.Fprintf(&b, "  %s not recorded", match.Field)
			case match.Empty:
				fmt.Fprintf(&b, "  %s empty", match.Field)
			default:
				fmt.Fprintf(&b, "  %s matches=%d", match.Field, match.Count)
			}
			if match.SpecialProjectOnly > 0 {
				fmt.Fprintf(&b, " special_project_only=%d", match.SpecialProjectOnly)
			}
			if match.TechnologyAndSpecialProject > 0 {
				fmt.Fprintf(&b, " technology_and_special_project=%d", match.TechnologyAndSpecialProject)
			}
			fmt.Fprintln(&b)
		}
	}
	return mcp.NewToolResultText(b.String()), nil
}

func technologyDefinitionMatch(status string, count int) string {
	switch {
	case status == "unavailable":
		return "unavailable"
	case count > 1:
		return "multiple"
	case status == "partial" && count == 0:
		return "unresolved"
	case status == "partial":
		return "unique-observed"
	case count == 0:
		return "unmatched"
	default:
		return "unique"
	}
}

type technologyDefinitionRow struct {
	definition, field int
	source            string
	value             *clausewitz.RawNode
	duplicate         bool
}

func technologyDefinitionRows(definitions []gamestate.TechnologyDefinition) []technologyDefinitionRow {
	var rows []technologyDefinitionRow
	for i, definition := range definitions {
		node := definition.Value
		if node == nil || node.Kind != "object" {
			rows = append(rows, technologyDefinitionRow{definition: i, field: -1, source: definition.Source, value: node})
			continue
		}
		counts := map[string]int{}
		for _, child := range node.Children {
			counts[child.Key.Text]++
		}
		for j, child := range node.Children {
			rows = append(rows, technologyDefinitionRow{
				definition: i, field: j, source: definition.Source, value: child, duplicate: counts[child.Key.Text] > 1,
			})
		}
	}
	return rows
}

func writeTechnologyRaw(b *strings.Builder, label string, value *clausewitz.RawNode, depth int, remaining *int) {
	fmt.Fprintf(b, "%s%s: ", strings.Repeat("  ", depth), label)
	switch {
	case value == nil:
		fmt.Fprintln(b, "static value exceeds index limits; inspect the local source")
	case value.Scalar != nil:
		spelling := "unquoted"
		if value.Scalar.Quoted {
			spelling = "quoted"
		}
		if value.Scalar.Escaped {
			spelling += ", escaped"
		}
		fmt.Fprintf(b, "%s (raw scalar; %s)\n", technologyTokenText(value.Scalar), spelling)
	case len(value.Children) == 0:
		fmt.Fprintln(b, "empty static block")
	case len(value.Children) > *remaining || depth == 2:
		fmt.Fprintf(b, "%s static block with %d entries; inspect the local source for contents\n", value.Kind, len(value.Children))
	default:
		fmt.Fprintf(b, "%s static block with %d ordered entries\n", value.Kind, len(value.Children))
		*remaining -= len(value.Children)
		for i, child := range value.Children {
			childLabel := fmt.Sprintf("entry[%d]", i)
			if child.Key != nil {
				childLabel += " key=" + technologyTokenText(child.Key)
			}
			writeTechnologyRaw(b, childLabel, child, depth+1, remaining)
		}
	}
}

func technologyTokenText(token *clausewitz.RawToken) string {
	if len(token.Text) > 512 {
		return "[large raw token; inspect the local source]"
	}
	return strconv.Quote(token.Text)
}
