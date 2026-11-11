package gamestate

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

// SavedLinkTables lists root tables that saved integer fields may name. Paths were
// observed in local Circinus 3.14 and Cygnus 4.5 saves; absent tables are reported per call.
var SavedLinkTables = []struct {
	Label string
	Path  []string
}{
	{"ships", []string{"ships"}}, {"fleet", []string{"fleet"}}, {"fleet_template", []string{"fleet_template"}},
	{"ship_design", []string{"ship_design"}}, {"leader", []string{"leaders"}}, {"country", []string{"country"}},
	{"planet", []string{"planets", "planet"}}, {"system", []string{"galactic_object"}}, {"species", []string{"species_db"}},
	{"army", []string{"army"}}, {"starbase", []string{"starbase_mgr", "starbases"}}, {"colony", []string{"colony"}},
	{"megastructure", []string{"megastructures"}}, {"war", []string{"war"}}, {"federation", []string{"federation"}},
	{"truce", []string{"truce"}}, {"sector", []string{"sectors"}}, {"spy_network", []string{"spy_networks"}},
	{"espionage_operation", []string{"espionage_operations", "operations"}}, {"first_contact", []string{"first_contacts", "contacts"}},
	{"archaeological_site", []string{"archaeological_sites", "sites"}}, {"agreement", []string{"agreements", "agreements"}},
	{"situation", []string{"situations", "situations"}}, {"construction_queue", []string{"construction", "queue_mgr", "queues"}},
	{"construction_item", []string{"construction", "item_mgr", "items"}}, {"pop_group", []string{"pop_groups"}},
	{"pop_job", []string{"pop_jobs"}}, {"pop_faction", []string{"pop_factions"}}, {"legacy_pop", []string{"pop"}},
	{"building", []string{"buildings"}}, {"district", []string{"districts"}}, {"zone", []string{"zones"}},
	{"deposit", []string{"deposit"}}, {"ambient_object", []string{"ambient_object"}}, {"debris", []string{"debris"}},
	{"bypass", []string{"bypasses"}}, {"natural_wormhole", []string{"natural_wormholes"}}, {"missile", []string{"missile"}},
	{"strike_craft", []string{"strike_craft"}}, {"resolution", []string{"resolution"}}, {"trade_route", []string{"trade_routes"}},
	{"council_position", []string{"council_positions", "council_positions"}}, {"astral_rift", []string{"astral_rifts", "rifts"}},
	{"cosmic_storm", []string{"storms", "storms"}}, {"focus_card", []string{"focus_cards"}}, {"ground_combat", []string{"ground_combat"}},
	{"trade_deal", []string{"trade_deal"}},
}

// savedLinkFieldTables maps saved field names to candidate table labels. A name match is a
// heuristic: it says which table a numeric value is compared with, not what the field means.
var savedLinkFieldTables = map[string][]string{
	"ship": {"ships"}, "ships": {"ships"}, "station": {"ships"}, "kill_target": {"ships"}, "source_ship": {"ships"},
	"fleet": {"fleet"}, "fleets": {"fleet"}, "target_fleet": {"fleet"}, "owned_fleets": {"fleet"}, "sensor_range_fleets": {"fleet"}, "fleet_template": {"fleet_template"},
	"ship_design": {"ship_design"}, "design": {"ship_design"}, "design_upgrade": {"ship_design"}, "global_ship_design": {"ship_design"},
	"leader": {"leader"}, "leaders": {"leader"}, "ruler": {"leader"}, "heir": {"leader"}, "scientist": {"leader"}, "admiral": {"leader"},
	"general": {"leader"}, "governor": {"leader"}, "envoy": {"leader"}, "owned_leaders": {"leader"},
	"country": {"country"}, "owner": {"country"}, "controller": {"country"}, "original_owner": {"country"}, "paying_country": {"country"},
	"receiver": {"country"}, "attacker": {"country"}, "defender": {"country"}, "attackers": {"country"}, "defenders": {"country"},
	"overlord": {"country"}, "subject": {"country"}, "target_country": {"country"}, "building_owner": {"country"},
	"planet": {"planet"}, "planets": {"planet"}, "capital": {"planet"}, "home_planet": {"planet"}, "target_planet": {"planet"},
	"owned_planets": {"planet"}, "controlled_planets": {"planet"}, "local_capital": {"planet"},
	"system": {"system"}, "systems": {"system"}, "galactic_object": {"system"}, "target_system": {"system"}, "starting_system": {"system"},
	"home_system": {"system"}, "restricted_systems": {"system"},
	"species": {"species"}, "base_species": {"species"}, "species_index": {"species"},
	"army": {"army"}, "armies": {"army"}, "owned_armies": {"army"}, "starbase": {"starbase"}, "starbases": {"starbase"},
	"colony": {"colony"}, "colonies": {"colony"}, "controlled_colonies": {"colony"},
	"megastructure": {"megastructure"}, "megastructures": {"megastructure"}, "owned_megastructures": {"megastructure"},
	"war": {"war"}, "wars": {"war"}, "federation": {"federation"}, "truce": {"truce"}, "sector": {"sector"}, "sectors": {"sector"},
	"spy_network": {"spy_network"}, "espionage_operation": {"espionage_operation"}, "first_contact": {"first_contact"},
	"archaeological_site": {"archaeological_site"}, "agreement": {"agreement"}, "agreements": {"agreement"},
	"situation": {"situation"}, "situations": {"situation"},
	"build_queue": {"construction_queue"}, "army_build_queue": {"construction_queue"}, "shipyard_build_queue": {"construction_queue"},
	"queue":     {"construction_queue"},
	"pop_group": {"pop_group"}, "pop_groups": {"pop_group"}, "pop_job": {"pop_job"}, "pop_jobs": {"pop_job"},
	"pop_faction": {"pop_faction"}, "pop_factions": {"pop_faction"},
	"buildings": {"building"}, "buildings_cache": {"building"}, "districts": {"district"}, "zone": {"zone"}, "zones": {"zone"},
	"deposit": {"deposit"}, "deposits": {"deposit"}, "ambient_object": {"ambient_object"}, "ambient_objects": {"ambient_object"},
	"debris": {"debris"}, "bypass": {"bypass"}, "bypasses": {"bypass"}, "natural_wormhole": {"natural_wormhole"},
	"resolution": {"resolution"}, "resolutions": {"resolution"}, "trade_route": {"trade_route"}, "trade_routes": {"trade_route"},
	"council_position": {"council_position"},
}

// savedLinkParentTables applies when a field sits directly under a named parent block.
var savedLinkParentTables = map[string][]string{
	"saved_leaders": {"leader"},
}

// savedLinkQualifiedTables applies to "parent.field" pairs.
var savedLinkQualifiedTables = map[string][]string{
	"externally_owned_build_queues.value": {"construction_queue"},
	"externally_owned_build_queues.key":   {"country"},
	"coordinate.origin":                   {"system"},
	"target_coordinate.origin":            {"system"},
	"source.ship":                         {"ships"},
	"federation.members":                  {"country"},
	"galactic_object.colonies":            {"planet", "colony"},
}

const SavedRecordLinksMaxLimit = 200

type SavedRecordLinkRow struct {
	Path       string                        `json:"path"`
	Field      string                        `json:"field"`
	Parent     string                        `json:"named_parent,omitempty"`
	Value      string                        `json:"value"`
	Rule       string                        `json:"rule"`
	Candidates []CountryExplorationCandidate `json:"candidates"`
}

type SavedRecordLinkSummary struct {
	Rule     string         `json:"rule"`
	Field    string         `json:"field"`
	Table    string         `json:"table"`
	Count    int            `json:"count"`
	Statuses map[string]int `json:"statuses"`
}

type SavedFieldCount struct {
	Field string `json:"field"`
	Count int    `json:"count"`
}

type SavedRecordLinks struct {
	RowCount         int                       `json:"row_count"`
	IntegerScalars   int                       `json:"integer_scalars"`
	Rows             []SavedRecordLinkRow      `json:"rows"`
	Summary          []SavedRecordLinkSummary  `json:"summary"`
	Unmapped         []SavedFieldCount         `json:"unmapped_integer_fields_preview"`
	UnmappedFields   int                       `json:"unmapped_integer_field_names"`
	Tables           []CountryExplorationTable `json:"tables_used"`
	TablesNotPresent []string                  `json:"tables_not_recorded,omitempty"`
}

type savedLinkFrame struct {
	node        *clausewitz.RawNode
	path, field string
	parent      string
}

// LinkSavedRecord walks node (located at path within root) and compares integer scalars of
// registered field names with root tables. Rows are emitted in saved depth-first order.
func LinkSavedRecord(ctx context.Context, root, node *clausewitz.RawNode, path, field, parent string, offset, limit int) (SavedRecordLinks, error) {
	out := SavedRecordLinks{Rows: []SavedRecordLinkRow{}}
	tables := map[string]CountryExplorationTable{}
	used := map[string]bool{}
	tableFor := func(label string) CountryExplorationTable {
		if table, ok := tables[label]; ok {
			return table
		}
		for _, spec := range SavedLinkTables {
			if spec.Label == label {
				tables[label] = explorationTable(root, label, spec.Path...)
				break
			}
		}
		return tables[label]
	}
	summary := map[[2]string]*SavedRecordLinkSummary{}
	unmapped := map[[2]string]int{} // {parent, field}; names are joined only once at the end
	stack := []savedLinkFrame{{node: node, path: path, field: field, parent: parent}}
	steps := 0
	for len(stack) > 0 {
		if steps++; steps%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return SavedRecordLinks{}, err
			}
		}
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		current := frame.node
		if current.IsScalar() {
			text := current.Scalar.Text
			wide, err := strconv.ParseUint(text, 10, 64)
			if err != nil || strconv.FormatUint(wide, 10) != text || current.Scalar.Quoted {
				continue
			}
			out.IntegerScalars++
			labels, rule := savedLinkRule(frame.field, frame.parent)
			if rule == "qualified:galactic_object.colonies" {
				// Older saves store planet IDs here and have no colony table; newer saves store colony IDs.
				if tableFor("colony").Status == "not-recorded" {
					labels = []string{"planet"}
				} else {
					labels = []string{"colony"}
				}
			}
			if len(labels) == 0 {
				if frame.field != "" {
					unmapped[[2]string{frame.parent, frame.field}]++
				}
				continue
			}
			row := SavedRecordLinkRow{Path: frame.path, Field: frame.field, Parent: frame.parent, Value: text, Rule: rule}
			for _, label := range labels {
				table := tableFor(label)
				used[label] = true
				var candidate CountryExplorationCandidate
				switch {
				case wide == 1<<32-1:
					// Frequently saved where no record is referenced; kept raw and not looked up.
					candidate = CountryExplorationCandidate{Table: label, Status: "uint32-max-value"}
				case wide > uint64(int(^uint(0)>>1)):
					candidate = CountryExplorationCandidate{Table: label, Status: "out-of-int-range"}
				default:
					candidate = table.candidate(int(wide))
				}
				row.Candidates = append(row.Candidates, candidate)
				key := [2]string{rule, label}
				if summary[key] == nil {
					summary[key] = &SavedRecordLinkSummary{Rule: rule, Field: frame.field, Table: label, Statuses: map[string]int{}}
				}
				summary[key].Count++
				summary[key].Statuses[candidate.Status]++
			}
			if out.RowCount >= offset && len(out.Rows) < limit {
				out.Rows = append(out.Rows, row)
			}
			out.RowCount++
			continue
		}
		// Push children in reverse so they pop in saved order.
		for i := len(current.Children) - 1; i >= 0; i-- {
			child := current.Children[i]
			next := savedLinkFrame{node: child, path: frame.path + "/" + strconv.Itoa(i), field: frame.field, parent: frame.parent}
			if child.Key != nil {
				name := child.Key.Text
				if _, numeric := strconv.Atoi(name); numeric == nil {
					// Numeric keys are record IDs or indexes; keep the enclosing field name.
					next.field = frame.field
				} else {
					next.field = name
					if frame.field != "" {
						next.parent = frame.field
					}
				}
			}
			stack = append(stack, next)
		}
	}
	for _, item := range summary {
		out.Summary = append(out.Summary, *item)
	}
	sort.Slice(out.Summary, func(i, j int) bool {
		if out.Summary[i].Count != out.Summary[j].Count {
			return out.Summary[i].Count > out.Summary[j].Count
		}
		return out.Summary[i].Rule+"\x00"+out.Summary[i].Table < out.Summary[j].Rule+"\x00"+out.Summary[j].Table
	})
	names := map[string]int{}
	for key, count := range unmapped {
		name := key[1]
		if key[0] != "" {
			name = key[0] + "." + key[1]
		}
		names[name] += count
	}
	out.UnmappedFields = len(names)
	for name, count := range names {
		out.Unmapped = append(out.Unmapped, SavedFieldCount{name, count})
	}
	sort.Slice(out.Unmapped, func(i, j int) bool {
		if out.Unmapped[i].Count != out.Unmapped[j].Count {
			return out.Unmapped[i].Count > out.Unmapped[j].Count
		}
		return out.Unmapped[i].Field < out.Unmapped[j].Field
	})
	if len(out.Unmapped) > CountryExplorationLimit {
		out.Unmapped = out.Unmapped[:CountryExplorationLimit]
	}
	labels := make([]string, 0, len(used))
	for label := range used {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		table := tables[label]
		if table.Status == "not-recorded" {
			out.TablesNotPresent = append(out.TablesNotPresent, label)
		}
		out.Tables = append(out.Tables, table)
	}
	return out, nil
}

func savedLinkRule(field, parent string) ([]string, string) {
	if parent == "flags" {
		// Flag names are arbitrary script strings; their values are not record IDs.
		return nil, ""
	}
	if labels, ok := savedLinkParentTables[parent]; ok {
		return labels, "parent:" + parent
	}
	if labels, ok := savedLinkQualifiedTables[parent+"."+field]; ok {
		return labels, "qualified:" + parent + "." + field
	}
	if labels, ok := savedLinkFieldTables[field]; ok {
		return labels, "field:" + field
	}
	return nil, ""
}

// SavedNodeContext returns the nearest named field and its named parent above a positional path.
func SavedNodeContext(root *clausewitz.RawNode, path string) (field, parent string) {
	if path == "" {
		return "", ""
	}
	node := root
	for _, component := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		index, err := strconv.Atoi(component)
		if err != nil || index < 0 || index >= len(node.Children) {
			return field, parent
		}
		node = node.Children[index]
		if node.Key != nil {
			if _, numeric := strconv.Atoi(node.Key.Text); numeric != nil {
				if field != "" {
					parent = field
				}
				field = node.Key.Text
			}
		}
	}
	return field, parent
}
