package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// saveRootTools points root keys at tools that read them. Keys without an entry are still
// reachable through read_save_tree, search_save_tree and get_saved_record_links.
var saveRootTools = map[string]string{
	"country":                             "get_empire_overview, get_country_saved_fields",
	"fleet":                               "get_fleet_power, get_fleet_orders",
	"ships":                               "get_ship_loadout, get_fleet_power",
	"ship_design":                         "get_ship_loadout, get_ship_design_collection",
	"planets":                             "get_planets, get_location_construction",
	"galactic_object":                     "get_system_contents, get_system_connections",
	"species_db":                          "get_species",
	"leaders":                             "get_leaders",
	"war":                                 "get_wars",
	"federation":                          "get_federations",
	"galactic_community":                  "get_galactic_community",
	"construction":                        "get_construction, get_location_construction",
	"starbase_mgr":                        "get_starbases",
	"army":                                "get_armies",
	"colony":                              "get_colonies, get_colony_population_records",
	"pop_groups":                          "get_colony_population_records",
	"first_contacts":                      "get_first_contacts",
	"archaeological_sites":                "get_archaeology",
	"espionage_operations":                "get_espionage_operations",
	"spy_networks":                        "get_espionage",
	"sectors":                             "get_sectors",
	"market":                              "get_resource_market",
	"trade_routes":                        "get_trade_routes",
	"debris":                              "get_debris",
	"missile":                             "get_space_combat_objects",
	"strike_craft":                        "get_space_combat_objects",
	"fleet_template":                      "get_fleet_templates",
	"council_positions":                   "get_council",
	"galaxy":                              "get_galaxy_metadata",
	"nebula":                              "get_galaxy_regions",
	"clusters":                            "get_galaxy_regions",
	"bypasses":                            "get_system_connections",
	"natural_wormholes":                   "get_system_connections",
	"player_event":                        "get_player_events",
	"saved_event_target":                  "get_saved_event_targets",
	"message":                             "get_saved_messages",
	"flags":                               "get_saved_flags",
	"pop_factions":                        "get_factions",
	"open_player_event_selection_history": "get_player_events",
}

func registerSaveRootFields(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_save_root_fields", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("List every root field of the save's meta and gamestate entries in saved order: scalars (version, date, tick, flags such as cheated_on_save), scalar lists (required_dlcs, achievement and similar, first 100 values) and the size of each table, with repeated keys marked and the dedicated tools that read known tables. Use it as an index of everything the save records; meanings of values are not inferred."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithString("entry", mcp.Enum("meta", "gamestate", "both"), mcp.Description("Which entry to list; default both")),
		mcp.WithString("key", mcp.Description("Optional exact root key; shows only fields with this key"))), handleSaveRootFields)
}

func handleSaveRootFields(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	for _, name := range []string{"save_id", "entry", "key"} {
		if value, ok := req.GetArguments()[name]; ok {
			if _, ok := value.(string); !ok {
				return toolError(fmt.Errorf("%s must be a string", name)), nil
			}
		}
	}
	entry := req.GetString("entry", "both")
	if entry != "meta" && entry != "gamestate" && entry != "both" {
		return toolError(fmt.Errorf("entry must be meta, gamestate or both")), nil
	}
	key, filtered := req.GetArguments()["key"]
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved root fields (save_id=%s, revision=%s) ===\n", archive.saveID, archive.revision)
	fmt.Fprintln(&b, "Fields are listed in saved order with their root index (usable as read_save_tree path /<index> on that entry). Values are shown as saved; DLC activation, achievement eligibility and other meanings are not inferred.")
	names := []string{"meta", "gamestate"}
	if entry != "both" {
		names = []string{entry}
	}
	for _, saved := range gamestate.LoadSaveRootFieldsFromArchive(archive.zip, names...) {
		fmt.Fprintf(&b, "\n[%s]\n", saved.Name)
		if saved.Issue != "" {
			fmt.Fprintf(&b, "  %s\n", saved.Issue)
		}
		shown := 0
		for _, field := range saved.Fields {
			if filtered && field.Key != key {
				continue
			}
			shown++
			label := fmt.Sprintf("  /%d %s", field.Index, field.Key)
			if field.Repeated {
				label += fmt.Sprintf(" [repeated key, occurrence %d]", field.Occurrence)
			}
			switch field.Shape {
			case "scalar":
				fmt.Fprintf(&b, "%s = %q\n", label, field.Scalar)
			case "list":
				fmt.Fprintf(&b, "%s: list of %d", label, field.ListTotal)
				if field.ListMixed {
					fmt.Fprint(&b, " (includes non-scalar items; use raw tools)")
				}
				if len(field.Values) > 0 {
					quoted := make([]string, len(field.Values))
					for i, value := range field.Values {
						quoted[i] = fmt.Sprintf("%q", value)
					}
					fmt.Fprintf(&b, ": %s", strings.Join(quoted, " "))
					if field.Scalars > len(field.Values) {
						fmt.Fprintf(&b, " ... (first %d shown)", len(field.Values))
					}
				}
				fmt.Fprintln(&b)
			case "object":
				fmt.Fprintf(&b, "%s: block with %d saved fields", label, field.Children)
				if tools := saveRootTools[field.Key]; tools != "" && saved.Name == "gamestate" {
					fmt.Fprintf(&b, " (dedicated tools: %s)", tools)
				}
				fmt.Fprintln(&b)
			case "empty":
				fmt.Fprintf(&b, "%s: empty block\n", label)
			default:
				fmt.Fprintf(&b, "%s: mixed saved block; use raw tools\n", label)
			}
		}
		if filtered && shown == 0 && saved.Issue == "" {
			fmt.Fprintln(&b, "  key not recorded")
		}
	}
	return mcp.NewToolResultText(b.String()), nil
}
