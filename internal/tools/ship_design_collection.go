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

const shipDesignCollectionPageSize = 20

func registerShipDesignCollection(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_ship_design_collection", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Page through a country's saved ship design collection and count matching design references in saved ship records."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID. If omitted, all saved player records must refer to one country.")),
		mcp.WithNumber("offset", mcp.Description("First saved entry, zero-based; 20 per page")),
		mcp.WithString("page_context", mcp.Description("Token from the first page; required for later pages"))), handleShipDesignCollection)
}

func handleShipDesignCollection(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	for _, key := range []string{"save_id", "page_context"} {
		if value, ok := req.GetArguments()[key]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", key)), nil
			}
		}
	}
	var countryID *int
	if _, ok := req.GetArguments()["country_id"]; ok {
		maxID := int(min(int64(^uint(0)>>1), int64(9007199254740991)))
		id, err := rawInteger(req, "country_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		countryID = &id
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
	data, err := gamestate.LoadShipDesignCollectionFromArchive(archive.zip, countryID)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, -1, "get_ship_design_collection", strconv.Itoa(data.CountryID))
	if _, err := validateRawPageContext(req, offset, token); err != nil {
		return toolError(err), nil
	}
	if offset > len(data.Entries) {
		return toolError(fmt.Errorf("offset exceeds saved entry count")), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved country ship design collection ===\nsave_id=%s\ncountry_id=%d\npage_context=%s\n", archive.saveID, data.CountryID, token)
	fmt.Fprintf(&b, "collection_status=%s collection_occurrences=%d unsupported_collections=%d ship_design_fields=%d design_references=%d saved_entries=%d offset=%d\n",
		shipDesignCollectionStatus(data), data.CollectionOccurrences, data.UnsupportedCollections, data.DesignFields, data.References, len(data.Entries), offset)
	fmt.Fprintf(&b, "design_table_status=%s design_unindexed_keys=%d ship_table_status=%s ship_unresolved_records=%d\n",
		data.DesignTableStatus, data.DesignUnindexedKeys, data.ShipTableStatus, data.ShipUnresolvedRecords)
	fmt.Fprintln(&b, "Collection entries are saved country references. Ship counts compare unambiguous ship_design and ship_design_implementation.design fields across saved ship records. Construction, template and event references are outside this comparison. Ownership and current use remain unverified. Paths select values in read_save_tree.")
	end := offset + min(shipDesignCollectionPageSize, len(data.Entries)-offset)
	for i := offset; i < end; i++ {
		entry := data.Entries[i]
		label := fmt.Sprintf("entry[%d] path=%q kind=%s key=%q", i, entry.Path, entry.Kind, entry.Key)
		if entry.RepeatedField {
			label += " repeated_ship_design_field=true"
		}
		if entry.ID == nil {
			if len(entry.Value.Scalar) > 512 || !galaxyRegionPreviewFits(entry.Value, 0) {
				fmt.Fprintf(&b, "%s: large saved value; use raw tree/byte tools\n", label)
			} else {
				writeSavedValue(&b, label, entry.Value, 0)
			}
			continue
		}
		fmt.Fprintf(&b, "%s design_id=%d saved_reference=%q design_status=%s", label, *entry.ID, entry.Value.Scalar, entry.DesignStatus)
		if entry.RepeatedReference {
			fmt.Fprint(&b, " repeated_saved_reference=true")
		}
		if entry.DesignPath != "" {
			fmt.Fprintf(&b, " design_path=%q", entry.DesignPath)
		}
		switch data.ShipTableStatus {
		case "recorded":
			fmt.Fprintf(&b, " ship_reference_matches=%d", entry.ShipMatches)
		case "partial":
			fmt.Fprintf(&b, " observed_ship_reference_matches=%d", entry.ShipMatches)
		default:
			fmt.Fprint(&b, " ship_reference_matches=unavailable")
		}
		fmt.Fprintln(&b)
	}
	if end < len(data.Entries) {
		fmt.Fprintf(&b, "next_offset=%d\n", end)
	} else {
		fmt.Fprintln(&b, "next_offset=none")
	}
	return mcp.NewToolResultText(b.String()), nil
}

func shipDesignCollectionStatus(data gamestate.ShipDesignCollection) string {
	switch {
	case data.CollectionOccurrences == 0:
		return "not-recorded"
	case data.CollectionOccurrences > 1:
		return "duplicate"
	case data.UnsupportedCollections > 0:
		return "unsupported"
	case len(data.Entries) == 1 && data.Entries[0].Kind == "empty-collection":
		return "empty"
	default:
		return "recorded"
	}
}
