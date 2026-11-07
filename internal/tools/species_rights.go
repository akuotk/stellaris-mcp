package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSpeciesRights(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_species_rights", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read the selected country's saved standard_species_rights_module. Shows default, primary and built_species records and the ordered species_rights list. record_index expands a saved species_rights entry to all scalar fields, including former and last_changed fields. Species ID matches are candidates. Effective rights, inheritance and date meanings remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")),
		mcp.WithNumber("record_index", mcp.Description("Zero-based position in saved species_rights list for all scalar fields"))), handleSpeciesRights)
}

func handleSpeciesRights(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["record_index"]; ok {
		entries := country.SpeciesRightsModules.Rights.Entries.Items
		if len(entries) == 0 {
			return toolError(fmt.Errorf("no decoded species_rights entries; omit record_index to inspect the saved module state")), nil
		}
		index, err := rawInteger(req, "record_index", 0, 0, len(entries)-1)
		if err != nil {
			return toolError(err), nil
		}
		selected = &index
	}
	return renderSpeciesRights(gs, id, country, selected), nil
}

func renderSpeciesRights(gs *gamestate.GameState, countryID int, country *gamestate.Country, selected *int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved species rights (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope follows the containing country's saved rights module. Effective rights, inheritance, module activation and date meanings remain unverified.")
	modules, module := country.SpeciesRightsModules, country.SpeciesRightsModules.Rights
	switch {
	case modules.Unsupported || module.Unsupported:
		fmt.Fprintln(&b, "Unsupported modules shape or duplicate known fields; use raw tree/byte tools")
		return mcp.NewToolResultText(b.String())
	case !modules.Recorded:
		fmt.Fprintln(&b, "country.modules not recorded")
		return mcp.NewToolResultText(b.String())
	case !module.Recorded:
		fmt.Fprintln(&b, "standard_species_rights_module not recorded")
		return mcp.NewToolResultText(b.String())
	}
	switch {
	case module.Enabled.Unsupported:
		fmt.Fprintln(&b, "standard_species_rights_module.enabled unsupported; use raw tools")
	case !module.Enabled.Recorded:
		fmt.Fprintln(&b, "standard_species_rights_module.enabled not recorded")
	default:
		fmt.Fprintf(&b, "standard_species_rights_module.enabled=%q (saved value)\n", module.Enabled.Value)
	}
	for _, section := range []struct {
		label  string
		record gamestate.SpeciesRightsRecord
	}{{"default", module.Default}, {"primary", module.Primary}, {"built_species", module.Built}} {
		writeSpeciesRightsRecord(&b, gs, section.label, section.record, true)
	}
	fmt.Fprintln(&b, "\nspecies_rights (saved order; record_index shows all saved scalar fields)")
	entries := module.Entries
	switch {
	case entries.Unsupported:
		fmt.Fprintln(&b, "  Unsupported saved list shape; use raw tree/byte tools")
	case !entries.Recorded:
		fmt.Fprintln(&b, "  Not recorded")
	case len(entries.Items) == 0:
		fmt.Fprintln(&b, "  Empty")
	}
	for index, record := range entries.Items {
		if selected != nil && *selected != index {
			continue
		}
		writeSpeciesRightsRecord(&b, gs, fmt.Sprintf("species_rights[%d]", index), record, selected != nil)
	}
	return mcp.NewToolResultText(b.String())
}

func writeSpeciesRightsRecord(b *strings.Builder, gs *gamestate.GameState, label string, record gamestate.SpeciesRightsRecord, detail bool) {
	fmt.Fprintln(b, label)
	if !record.Recorded {
		fmt.Fprintln(b, "  Not recorded")
		return
	}
	if record.Unsupported {
		fmt.Fprintln(b, "  Unsupported saved fields; displayed scalar fields may be partial. Use raw tree/byte tools.")
	} else if len(record.Fields) == 0 {
		fmt.Fprintln(b, "  Empty")
		return
	}
	if record.DuplicateFields {
		fmt.Fprintln(b, "  Duplicate saved keys retained in order")
	}
	switch {
	case record.SpeciesIndex.Unsupported:
		fmt.Fprintln(b, "  species_index unsupported or duplicate; species candidate unresolved")
	case record.SpeciesIndex.Value == nil:
		fmt.Fprintln(b, "  species_index not recorded")
	default:
		id := *record.SpeciesIndex.Value
		if species, ok := gs.Species[id]; ok {
			fmt.Fprintf(b, "  species_index=%d; species database ID candidate: %s\n", id, name(species.Name))
		} else {
			fmt.Fprintf(b, "  species_index=%d; species database ID candidate unavailable\n", id)
		}
	}
	for index, field := range record.Fields {
		if detail || field.Key == "citizenship" || field.Key == "living_standard" {
			fmt.Fprintf(b, "  Field %d key=%q value=%q\n", index, field.Key, field.Value)
		}
	}
}
