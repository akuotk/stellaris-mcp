package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const galaxyRegionPageSize = 20

func registerGalaxyRegions(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_galaxy_regions", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through saved galaxy clusters, nebula records, or their object entries in saved order. Object IDs are shown as numeric key candidates with unverified namespaces."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithString("kind", mcp.Description("clusters (default), cluster_objects, nebulae, or nebula_objects")),
		mcp.WithNumber("offset", mcp.Description("First saved entry, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from this view's first page; required for later pages"))), handleGalaxyRegions)
}

func handleGalaxyRegions(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	for _, key := range []string{"kind", "save_id", "page_context"} {
		if value, ok := req.GetArguments()[key]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	kind := req.GetString("kind", "clusters")
	switch kind {
	case "clusters", "cluster_objects", "nebulae", "nebula_objects":
	default:
		return toolError(fmt.Errorf("kind must be clusters, cluster_objects, nebulae, or nebula_objects")), nil
	}
	offset, err := rawInteger(req, "offset", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, -1, "get_galaxy_regions", kind)
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	if !verified {
		return toolError(fmt.Errorf("page_context required for later pages")), nil
	}
	data, err := gamestate.LoadGalaxyRegionsFromArchive(archive.zip)
	if err != nil {
		return toolError(err), nil
	}
	set := data.Clusters
	fieldKey, objectKind := "objects", "cluster_objects"
	if kind == "nebulae" || kind == "nebula_objects" {
		set = data.Nebulae
		fieldKey, objectKind = "galactic_object", "nebula_objects"
	}
	objects := kind == objectKind
	count := len(set.Regions)
	if objects {
		count = len(set.Objects)
	}
	if offset > count {
		return toolError(fmt.Errorf("offset exceeds saved entry count")), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved galaxy regions ===\nsave_id=%s\npage_context=%s\nkind=%s root_occurrences=%d saved_entries=%d offset=%d\n", archive.saveID, token, kind, set.Roots, count, offset)
	fmt.Fprintln(&b, "Object ID matches are numeric candidates. Identifier namespaces, region membership, routes, coordinates, origins, radii and visibility remain unverified. Use raw tools for deep, large or mixed values.")
	if objects {
		fmt.Fprint(&b, "Candidate tables:")
		for _, table := range data.Tables {
			status := "recorded"
			if !table.Recorded {
				status = "not-recorded"
			} else if table.Incomplete {
				status = "incomplete"
			}
			fmt.Fprintf(&b, " %s=%s", table.Label, status)
		}
		fmt.Fprintln(&b)
	}
	end := offset + min(galaxyRegionPageSize, count-offset)
	for i := offset; i < end; i++ {
		if objects {
			writeGalaxyRegionObject(&b, i, set.Objects[i], set.ParentsByID, data.Tables)
		} else {
			writeGalaxyRegion(&b, i, set.Regions[i], fieldKey, objectKind)
		}
	}
	if end < count {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}

func writeGalaxyRegion(b *strings.Builder, index int, region gamestate.SavedGalaxyRegion, fieldKey, objectKind string) {
	label := fmt.Sprintf("entry[%d] %s", index, region.Path)
	fmt.Fprintf(b, "%s: %s_fields=%d child_kind=%s child_offset=%d child_entries=%d\n", label, fieldKey, region.ObjectFields, objectKind, region.ObjectOffset, region.Objects)
	if region.Status != "" {
		writeGalaxyRegionPreview(b, "  "+region.Status, region.Value)
		return
	}
	if len(region.Value.Object.Pairs) == 0 {
		fmt.Fprintln(b, "  empty object")
	}
	seen := map[string]bool{}
	shown := 0
	for i, field := range region.Value.Object.Pairs {
		if field.Key == fieldKey {
			continue
		}
		if shown == galaxyRegionPageSize {
			fmt.Fprintln(b, "  Further parent fields require raw tools.")
			break
		}
		fieldLabel := fmt.Sprintf("  field[%d] key=%q", i, field.Key)
		if seen[field.Key] {
			fieldLabel += " [duplicate saved key]"
		}
		seen[field.Key] = true
		shown++
		writeGalaxyRegionPreview(b, fieldLabel, field.Value)
	}
}

func writeGalaxyRegionObject(b *strings.Builder, index int, object gamestate.SavedGalaxyRegionObject, parents map[int]int, tables []gamestate.GalaxyRegionTable) {
	label := fmt.Sprintf("entry[%d] %s", index, object.Path)
	if object.Status == "field-not-recorded" {
		fmt.Fprintf(b, "%s: object field not recorded\n", label)
		return
	}
	if object.Status != "" {
		label += " (" + object.Status + ")"
	}
	writeGalaxyRegionPreview(b, label, object.Value)
	if object.ID == nil {
		return
	}
	id := *object.ID
	fmt.Fprintf(b, "  Numeric ID lookup: occurrences_in_parent=%d parents_with_id=%d", object.OccurrencesInParent, parents[id])
	for _, table := range tables {
		key := table.Keys[id]
		status := "no-candidate"
		switch {
		case !table.Recorded:
			status = "table-not-recorded"
		case key.Occurrences > 1:
			status = "duplicate-key"
		case key.Occurrences == 1 && !key.Object:
			status = "unsupported-record"
		case key.Object:
			status = "candidate"
		case table.Incomplete:
			status = "incomplete-table"
		}
		fmt.Fprintf(b, " %s=%s", table.Label, status)
	}
	fmt.Fprintln(b)
}

func writeGalaxyRegionPreview(b *strings.Builder, label string, value clausewitz.Value) {
	if !galaxyRegionPreviewFits(value, 0) {
		fmt.Fprintf(b, "%s: large saved block; use raw tree/byte tools\n", label)
		return
	}
	writeSavedValue(b, label, value, 0)
}

func galaxyRegionPreviewFits(value clausewitz.Value, depth int) bool {
	if depth == 2 {
		return true
	}
	switch {
	case value.IsObject():
		if len(value.Object.Pairs) > galaxyRegionPageSize {
			return false
		}
		for _, field := range value.Object.Pairs {
			if !galaxyRegionPreviewFits(field.Value, depth+1) {
				return false
			}
		}
	case value.IsList():
		if len(value.List) > galaxyRegionPageSize {
			return false
		}
		for _, item := range value.List {
			if !galaxyRegionPreviewFits(item, depth+1) {
				return false
			}
		}
	}
	return true
}
