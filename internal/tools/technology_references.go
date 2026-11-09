package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const technologyReferencePageSize = 20

func registerTechnologyReferences(s *server.MCPServer, source gamestate.TechnologyReferenceSource) {
	s.AddTool(mcp.NewTool("get_technology_references", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Find exact literal occurrences of a technology key in direct prerequisites fields of installed definitions. Reports static references and prerequisite shapes."),
		mcp.WithString("technology_id", mcp.Required(), mcp.Description("Exact technology key")),
		mcp.WithNumber("offset", mcp.Description("First matching prerequisite field, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from the first page; required for later pages"))),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handleTechnologyReferences(ctx, req, source)
		})
}

func handleTechnologyReferences(_ context.Context, req mcp.CallToolRequest, source gamestate.TechnologyReferenceSource) (*mcp.CallToolResult, error) {
	for _, key := range []string{"technology_id", "page_context"} {
		if value, ok := req.GetArguments()[key]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	id := req.GetString("technology_id", "")
	if !gamestate.IsTechnologyReferenceLiteral(id) {
		return toolError(fmt.Errorf("technology_id must be an explicit literal key of at most %d bytes", gamestate.MaxTechnologyKeyBytes)), nil
	}
	offset, err := rawInteger(req, "offset", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	if offset > 0 && req.GetString("page_context", "") == "" {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	index := gamestate.TechnologyReferenceIndex{Status: "unavailable", Fingerprint: "unavailable"}
	if source != nil {
		index = source()
	}
	token := rawPageToken(&rawSaveArchive{}, -1, "get_technology_references", index.Fingerprint, id)
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	references := index.References(id)
	if offset > len(references) {
		return toolError(fmt.Errorf("offset exceeds observed reference count")), nil
	}
	matchStatus := "observed"
	switch {
	case index.Status == "unavailable":
		matchStatus = "unavailable"
	case index.Status == "partial" && len(references) == 0:
		matchStatus = "unresolved"
	case len(references) == 0:
		matchStatus = "unmatched"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Static local technology prerequisite references ===\ntechnology_id=%q\npage_context=%s\n", id, token)
	fmt.Fprintf(&b, "source_status=%s match_status=%s observed_fields=%d offset=%d\n", index.Status, matchStatus, len(references), offset)
	fmt.Fprintf(&b, "inline_script_observed=%t inline_script_expansion=unavailable unresolved_fields_observed=%t mod_coverage=not-inspected\n", index.InlineScriptObserved, index.UnresolvedFieldsObserved)
	fmt.Fprintln(&b, "Exact literals are read from direct prerequisites fields. Nested conditions and has_technology checks are outside this lookup. Multiple prerequisites, DLC activation, MOD overrides and unlock availability remain unevaluated.")
	end := offset + min(technologyReferencePageSize, len(references)-offset)
	for i := offset; i < end; i++ {
		ref := references[i]
		definitionOccurrences := index.DefinitionOccurrences(ref)
		fmt.Fprintf(&b, "entry[%d] category=%s source=%s", i, ref.Category, ref.Source)
		if ref.WrapperKey != "" {
			fmt.Fprintf(&b, " wrapper_key=%q", ref.WrapperKey)
		}
		if ref.DefinitionKey == "" {
			fmt.Fprint(&b, " definition_key_status=unresolved")
		} else {
			fmt.Fprintf(&b, " definition_key=%q definition_key_occurrences=%d", ref.DefinitionKey, definitionOccurrences)
		}
		fmt.Fprintf(&b, " field_index=%d prerequisites_fields=%d prerequisites_shape=%s literal_matches=%d direct_literal_count=%d",
			ref.FieldIndex, ref.FieldOccurrences, ref.Shape, ref.Matches, ref.DirectLiteralCount)
		if definitionOccurrences > 1 {
			fmt.Fprint(&b, " definition_key_status=duplicate")
		}
		if ref.FieldOccurrences > 1 {
			fmt.Fprint(&b, " prerequisites_field_status=duplicate")
		}
		fmt.Fprintln(&b)
		preview := technologyReferencePageSize
		writeTechnologyRaw(&b, "  prerequisites", ref.Prerequisites, 0, &preview)
	}
	if end < len(references) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}
