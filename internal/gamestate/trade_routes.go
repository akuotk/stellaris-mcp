package gamestate

import (
	"archive/zip"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type SavedTradeRouteEntry struct {
	Path        string
	Value       clausewitz.Value
	ID          *int
	Unsupported bool
}

type SavedTradeRouteReference struct {
	Path string
	ID   OptionalIntRef
}

type TradeRouteKeyCount struct {
	Occurrences int
	Objects     int
}

type TradeRouteCandidateTable struct {
	Label string
	Table ConnectionRecordTable[SpaceObjectCandidate]
}

type SavedTradeRoutes struct {
	Routes, Manager                        []SavedTradeRouteEntry
	RouteRoots, ManagerRoots, ManagerLists int
	RouteIncomplete, ManagerIncomplete     bool
	RouteKeys                              map[int]TradeRouteKeyCount
	ManagerIDs                             map[int]int
	CandidateTables                        []TradeRouteCandidateTable
}

func LoadTradeRoutesFromArchive(archive *zip.Reader) (SavedTradeRoutes, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return SavedTradeRoutes{}, err
	}
	root, err := clausewitz.Parse(data)
	if err != nil {
		return SavedTradeRoutes{}, fmt.Errorf("parse gamestate: %w", err)
	}
	result := SavedTradeRoutes{RouteKeys: map[int]TradeRouteKeyCount{}, ManagerIDs: map[int]int{}}
	wanted := map[int]bool{}
	for i, pair := range root.Pairs {
		switch pair.Key {
		case "trade_routes":
			path := fmt.Sprintf("root[%d] key=%q", i, pair.Key)
			result.readRoutes(path, pair.Value, wanted)
		case "trade_routes_manager":
			path := fmt.Sprintf("root[%d] key=%q", i, pair.Key)
			result.readManager(path, pair.Value)
		}
	}
	rootValue := clausewitz.Value{Object: root}
	for _, target := range []struct {
		label string
		path  []string
	}{
		{"country", []string{"country"}},
		{"system", []string{"galactic_object"}},
		{"planet", []string{"planets", "planet"}},
		{"starbase", []string{"starbase_mgr", "starbases"}},
		{"fleet", []string{"fleet"}},
	} {
		result.CandidateTables = append(result.CandidateTables, TradeRouteCandidateTable{
			Label: target.label,
			Table: systemSubsetTable[SpaceObjectCandidate](rootValue, wanted, target.path...),
		})
	}
	return result, nil
}

func (result *SavedTradeRoutes) readRoutes(path string, value clausewitz.Value, wanted map[int]bool) {
	result.RouteRoots++
	if !value.IsObject() || len(value.Object.Pairs) == 0 {
		result.RouteIncomplete = result.RouteIncomplete || !value.IsObject()
		result.Routes = append(result.Routes, SavedTradeRouteEntry{Path: path, Value: value, Unsupported: !value.IsObject()})
		return
	}
	for i, pair := range value.Object.Pairs {
		entry := SavedTradeRouteEntry{Path: fmt.Sprintf("%s/field[%d] key=%q", path, i, pair.Key), Value: pair.Value}
		if id, err := strconv.Atoi(pair.Key); err == nil {
			entry.ID = &id
			count := result.RouteKeys[id]
			count.Occurrences++
			if entry.Value.IsObject() {
				count.Objects++
			}
			result.RouteKeys[id] = count
			for _, ref := range entry.References() {
				if ref.ID.Value != nil {
					wanted[*ref.ID.Value] = true
				}
			}
		} else {
			entry.Unsupported = true
		}
		result.Routes = append(result.Routes, entry)
	}
}

func (result *SavedTradeRoutes) readManager(path string, value clausewitz.Value) {
	result.ManagerRoots++
	if !value.IsObject() || len(value.Object.Pairs) == 0 {
		result.ManagerIncomplete = result.ManagerIncomplete || !value.IsObject()
		result.Manager = append(result.Manager, SavedTradeRouteEntry{Path: path, Value: value, Unsupported: !value.IsObject()})
		return
	}
	for i, pair := range value.Object.Pairs {
		fieldPath := fmt.Sprintf("%s/field[%d] key=%q", path, i, pair.Key)
		field := pair.Value
		if pair.Key != "trade_routes" {
			result.Manager = append(result.Manager, SavedTradeRouteEntry{Path: fieldPath, Value: field})
			continue
		}
		result.ManagerLists++
		if !field.IsList() {
			// An empty block has no items in the saved format.
			unsupported := !field.IsObject() || len(field.Object.Pairs) > 0
			result.ManagerIncomplete = result.ManagerIncomplete || unsupported
			result.Manager = append(result.Manager, SavedTradeRouteEntry{Path: fieldPath, Value: field, Unsupported: unsupported})
			continue
		}
		for j, item := range field.List {
			var ref OptionalIntRef
			ref.UnmarshalClausewitz(item)
			if ref.Value != nil {
				result.ManagerIDs[*ref.Value]++
			} else {
				result.ManagerIncomplete = true
			}
			result.Manager = append(result.Manager, SavedTradeRouteEntry{
				Path: fmt.Sprintf("%s/item[%d]", fieldPath, j), Value: item, ID: ref.Value, Unsupported: ref.Unsupported,
			})
		}
	}
}

func (entry SavedTradeRouteEntry) References() []SavedTradeRouteReference {
	if entry.ID == nil || !entry.Value.IsObject() {
		return nil
	}
	var refs []SavedTradeRouteReference
	for _, key := range []string{"owner", "from", "to"} {
		refs = append(refs, SavedTradeRouteReference{Path: key, ID: savedPathInt(entry.Value, key)})
	}
	for i, pair := range entry.Value.Object.Pairs {
		if pair.Key != "path" || !pair.Value.IsList() {
			continue
		}
		for j, item := range pair.Value.List {
			refs = append(refs, SavedTradeRouteReference{
				Path: fmt.Sprintf("path field[%d]/item[%d].id", i, j), ID: savedPathInt(item, "id"),
			})
		}
	}
	return refs
}
