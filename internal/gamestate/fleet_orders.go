package gamestate

import (
	"archive/zip"
	"context"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

const FleetOrdersPageSize = 20

type FleetOrderScope struct {
	Status             string `json:"status"`
	ManagerOccurrences int    `json:"manager_occurrences"`
	ListOccurrences    int    `json:"owned_fleets_occurrences"`
	Unsupported        int    `json:"unsupported_rows"`
}

type FleetOrderRow struct {
	Path, ParentPath, OrderPath, Role, RefStatus, RefTarget string
	Node                                                    *clausewitz.RawNode
	Index, OwnedOccurrences, OrderOccurrences               int
	Fleet                                                   *CountryExplorationCandidate
	Reference                                               *CountryExplorationCandidate
}

type SavedFleetOrders struct {
	CountryID, EntryIndex, RowCount, OrderOccurrences, OwnedOccurrences int
	CountryPath, ViewStatus                                             string
	Scope                                                               FleetOrderScope
	FleetTable                                                          CountryExplorationTable
	Fleet                                                               *CountryExplorationCandidate
	Tables                                                              []CountryExplorationTable
	OwnedPaths                                                          []string
	Rows                                                                []FleetOrderRow
	offset                                                              int
}

type savedFleetScopeRow struct {
	path, parent, role string
	node               *clausewitz.RawNode
	index              int
	id                 *int
}

func LoadFleetOrdersFromArchive(ctx context.Context, archive *zip.Reader, countryID, fleetID *int, offset int) (SavedFleetOrders, error) {
	country, err := loadRawSavedCountry(ctx, archive, countryID, false)
	if err != nil {
		return SavedFleetOrders{}, err
	}
	scope, entries, counts := readFleetOrderScope(country.Node, country.Path)
	out := SavedFleetOrders{
		CountryID: country.CountryID, EntryIndex: country.EntryIndex, CountryPath: country.Path,
		Scope: scope, ViewStatus: scope.Status, offset: offset, Rows: make([]FleetOrderRow, 0, FleetOrdersPageSize),
		FleetTable: explorationTable(country.root, "fleet", "fleet"),
	}
	if fleetID == nil {
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return SavedFleetOrders{}, err
			}
			if out.inPage() {
				row := FleetOrderRow{Path: entry.path, ParentPath: entry.parent, Role: entry.role, Node: entry.node, Index: entry.index}
				if entry.id != nil {
					candidate := out.FleetTable.candidate(*entry.id)
					row.Fleet, row.OwnedOccurrences = &candidate, counts[*entry.id]
					if candidate.RecordStatus == "object" {
						for _, field := range out.FleetTable.byID[*entry.id][0].node.Children {
							if field.Key != nil && field.Key.Text == "current_order" {
								row.OrderOccurrences++
							}
						}
					}
				}
				out.Rows = append(out.Rows, row)
			}
			out.RowCount++
		}
		return out, nil
	}
	out.OwnedOccurrences = counts[*fleetID]
	if out.OwnedOccurrences == 0 {
		out.ViewStatus = "fleet-not-observed-in-owned-fleets"
		return out, nil
	}
	for _, entry := range entries {
		if entry.id != nil && *entry.id == *fleetID && len(out.OwnedPaths) < FleetOrdersPageSize {
			out.OwnedPaths = append(out.OwnedPaths, entry.path)
		}
	}
	candidate := out.FleetTable.candidate(*fleetID)
	out.Fleet = &candidate
	if candidate.RecordStatus != "object" {
		out.ViewStatus = "fleet-" + candidate.Status
		for _, record := range out.FleetTable.byID[*fleetID] {
			if out.inPage() {
				out.Rows = append(out.Rows, FleetOrderRow{
					Path: out.FleetTable.Path + "/" + strconv.Itoa(record.index), ParentPath: out.FleetTable.Path,
					Role: "fleet-record-candidate", Node: record.node, Index: record.index,
				})
			}
			out.RowCount++
		}
		return out, nil
	}
	out.Tables = []CountryExplorationTable{
		out.FleetTable,
		explorationTable(country.root, "planet", "planets", "planet"),
		fleetOrderProjectTable(country.Node, country.Path),
		explorationTable(country.root, "archaeological_site", "archaeological_sites", "sites"),
	}
	fleet := out.FleetTable.byID[*fleetID][0].node
	out.ViewStatus = "current-order-not-recorded"
	for i, order := range fleet.Children {
		if order.Key == nil || order.Key.Text != "current_order" {
			continue
		}
		out.OrderOccurrences++
		out.ViewStatus = "recorded"
		switch order.Kind {
		case "empty_block":
			out.ViewStatus = "empty"
		case "mixed":
			out.ViewStatus = "partial"
		case "object":
		default:
			out.ViewStatus = "unsupported"
		}
		path := candidate.Path + "/" + strconv.Itoa(i)
		if out.inPage() {
			out.Rows = append(out.Rows, FleetOrderRow{
				Path: path, ParentPath: candidate.Path, OrderPath: path, Role: "current-order", Node: order, Index: i,
			})
		}
		out.RowCount++
		if err := out.walkOrder(ctx, order, path); err != nil {
			return SavedFleetOrders{}, err
		}
	}
	if out.OrderOccurrences > 1 {
		out.ViewStatus = "duplicate"
	}
	return out, nil
}

func readFleetOrderScope(country *clausewitz.RawNode, path string) (FleetOrderScope, []savedFleetScopeRow, map[int]int) {
	scope := FleetOrderScope{Status: "not-recorded"}
	var rows []savedFleetScopeRow
	ids := map[int]int{}
	for i, manager := range country.Children {
		if manager.Key == nil || manager.Key.Text != "fleets_manager" {
			continue
		}
		scope.ManagerOccurrences++
		managerPath := path + "/" + strconv.Itoa(i)
		if manager.Kind == "mixed" {
			scope.Unsupported++
		} else if manager.Kind != "object" && manager.Kind != "empty_block" {
			scope.Unsupported++
			rows = append(rows, savedFleetScopeRow{managerPath, path, "unsupported-manager", manager, i, nil})
			continue
		}
		before := scope.ListOccurrences
		for j, list := range manager.Children {
			if list.Key == nil || list.Key.Text != "owned_fleets" {
				continue
			}
			scope.ListOccurrences++
			listPath := managerPath + "/" + strconv.Itoa(j)
			switch list.Kind {
			case "empty_block":
				scope.Status = "empty"
				rows = append(rows, savedFleetScopeRow{listPath, managerPath, "empty-owned-fleets", list, j, nil})
				continue
			case "list", "mixed":
				scope.Status = "recorded"
			default:
				scope.Unsupported++
				rows = append(rows, savedFleetScopeRow{listPath, managerPath, "unsupported-owned-fleets", list, j, nil})
				continue
			}
			for k, entry := range list.Children {
				row := savedFleetScopeRow{listPath + "/" + strconv.Itoa(k), listPath, "unsupported-owned-reference", entry, k, nil}
				if entry.Key == nil && (entry.Kind == "object" || entry.Kind == "mixed") {
					node, state := uniqueRawField(entry, "fleet")
					if state == "unique" {
						row.id = fleetOrderNumber(node)
					} else if state == "duplicate" {
						row.role = "duplicate-fleet-field"
					}
				}
				if row.id != nil {
					row.role = "owned-fleet-reference"
					ids[*row.id]++
				}
				if row.id == nil || entry.Kind == "mixed" {
					scope.Unsupported++
				}
				rows = append(rows, row)
			}
		}
		if scope.ListOccurrences == before {
			rows = append(rows, savedFleetScopeRow{managerPath, path, "owned-fleets-not-recorded", manager, i, nil})
		}
	}
	if scope.ManagerOccurrences > 1 || scope.ListOccurrences > 1 {
		scope.Status = "duplicate"
	} else if scope.Unsupported > 0 {
		scope.Status = "partial"
	}
	return scope, rows, ids
}

func fleetOrderNumber(node *clausewitz.RawNode) *int {
	if node.IsScalar() {
		if n, err := strconv.Atoi(node.Scalar.Text); err == nil && n >= 0 {
			return &n
		}
	}
	return nil
}

func fleetOrderProjectTable(country *clausewitz.RawNode, path string) CountryExplorationTable {
	table := CountryExplorationTable{Table: "special_project", SavedTable: "selected country.events.special_project.id", Status: "not-recorded", Path: path}
	var events *clausewitz.RawNode
	var paths []string
	for i, child := range country.Children {
		if child.Key != nil && child.Key.Text == "events" {
			table.Occurrences++
			events = child
			if len(paths) < FleetOrdersPageSize {
				paths = append(paths, path+"/"+strconv.Itoa(i))
			}
		}
	}
	if table.Occurrences > 1 {
		table.Status, table.DuplicatePaths = "duplicate", paths
		return table
	}
	if events == nil {
		return table
	}
	table.Path = paths[0]
	switch events.Kind {
	case "object", "empty_block":
		table.Status = "recorded"
	case "mixed":
		table.Status = "partial"
	default:
		table.Status = "unsupported"
		return table
	}
	table.byID = make(map[int][]explorationCandidateRecord)
	for i, project := range events.Children {
		if project.Key == nil || project.Key.Text != "special_project" {
			continue
		}
		if project.Kind == "object" || project.Kind == "mixed" {
			if field, state := uniqueRawField(project, "id"); state == "unique" {
				if id := fleetOrderNumber(field); id != nil {
					table.byID[*id] = append(table.byID[*id], explorationCandidateRecord{i, project})
					continue
				}
			}
		}
		table.Unindexed++
		table.Status = "partial"
	}
	return table
}

func (out *SavedFleetOrders) inPage() bool {
	return out.RowCount >= out.offset && len(out.Rows) < FleetOrdersPageSize
}

func (out *SavedFleetOrders) walkOrder(ctx context.Context, root *clausewitz.RawNode, path string) error {
	type frame struct {
		node *clausewitz.RawNode
		next int
	}
	stack := []frame{{node: root}}
	var buffer []byte
	for len(stack) > 0 {
		parent := &stack[len(stack)-1]
		if parent.next == len(parent.node.Children) {
			stack = stack[:len(stack)-1]
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		i := parent.next
		child := parent.node.Children[i]
		parent.next++
		if out.inPage() {
			buffer = append(buffer[:0], path...)
			for _, ancestor := range stack[:len(stack)-1] {
				buffer = append(buffer, '/')
				buffer = strconv.AppendInt(buffer, int64(ancestor.next-1), 10)
			}
			container := string(buffer)
			row := FleetOrderRow{Path: container + "/" + strconv.Itoa(i), ParentPath: container, OrderPath: path, Role: "saved-field", Node: child, Index: i}
			if child.Key != nil {
				for _, table := range out.Tables {
					if child.Key.Text == table.Table {
						row.RefTarget = table.Table
						_, row.RefStatus = uniqueRawField(parent.node, table.Table)
						if row.RefStatus == "unique" {
							row.RefStatus = "unsupported"
							if id := fleetOrderNumber(child); id != nil {
								candidate := table.candidate(*id)
								row.RefStatus, row.Reference = "integer", &candidate
							}
						}
						break
					}
				}
			}
			out.Rows = append(out.Rows, row)
		}
		out.RowCount++
		if len(child.Children) > 0 {
			stack = append(stack, frame{node: child})
		}
	}
	return nil
}
