package gamestate

import (
	"archive/zip"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type SavedControlledPlanets struct {
	CountryID, ListOccurrences, UnsupportedListEntries int
	ControllerMatchRecords, UnresolvedPlanetRecords    int
	PlanetTable, ColonyTable                           ControlledPlanetTableStatus
	Listed, Unlisted                                   []ControlledPlanetEntry
}

type ControlledPlanetTableStatus struct {
	Status, Path  string
	UnindexedKeys int
}

type ControlledPlanetEntry struct {
	Path, Kind, ListMembership string
	Value                      clausewitz.Value
	ID                         *int
	ListOccurrences            int
	PlanetStatus, PlanetPath   string
	PlanetCandidates           int
	PlanetCandidatePaths       []string
	Controller, Owner, Colony  ControlledPlanetField
	ColonyStatus, ColonyPath   string
	ColonyCandidatePaths       []string
	CarrierType                OptionalString
	CarrierReference           ControlledPlanetField
}

type ControlledPlanetField struct {
	Status, Path string
	Value        clausewitz.Value
	ID           *int
}

type controlledPlanetRecord struct {
	path  string
	value clausewitz.Value
	id    *int
}

type controlledPlanetTable struct {
	ControlledPlanetTableStatus
	records []controlledPlanetRecord
	byID    map[int][]int
}

func LoadControlledPlanetsFromArchive(archive *zip.Reader, requestedCountry *int) (SavedControlledPlanets, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return SavedControlledPlanets{}, fmt.Errorf("cannot read selected gamestate")
	}
	doc, err := clausewitz.Parse(data)
	if err != nil {
		return SavedControlledPlanets{}, fmt.Errorf("cannot parse selected gamestate")
	}
	id, country, path, err := selectSavedCountry(doc, requestedCountry)
	if err != nil {
		return SavedControlledPlanets{}, err
	}
	result := SavedControlledPlanets{CountryID: id}
	counts := map[int]int{}
	for i, pair := range country.Object.Pairs {
		if pair.Key == "controlled_planets" {
			result.readList(path+"/"+strconv.Itoa(i), pair.Value, counts)
		}
	}
	root := clausewitz.Value{Object: doc}
	planets := indexControlledPlanetTable(root, "planets", "planet")
	colonies := indexControlledPlanetTable(root, "colony")
	result.PlanetTable, result.ColonyTable = planets.ControlledPlanetTableStatus, colonies.ControlledPlanetTableStatus
	for i := range result.Listed {
		entry := &result.Listed[i]
		if entry.ID == nil {
			continue
		}
		entry.ListOccurrences = counts[*entry.ID]
		record, status, candidates := planets.lookup(*entry.ID)
		entry.PlanetStatus, entry.PlanetCandidates = status, candidates
		if status == "duplicate" {
			entry.PlanetCandidatePaths = planets.candidatePaths(*entry.ID)
		}
		if record != nil {
			entry.PlanetPath = record.path
			if status == "unique" {
				entry.readPlanet(*record, colonies)
			}
		}
	}
	for _, record := range planets.records {
		if !record.value.IsObject() {
			result.UnresolvedPlanetRecords++
			continue
		}
		controller := controlledPlanetField(record.value, record.path, "controller")
		unresolved := controller.Status == "duplicate" || controller.Status == "unsupported" || record.id == nil ||
			(record.id != nil && len(planets.byID[*record.id]) > 1)
		if unresolved {
			result.UnresolvedPlanetRecords++
		}
		matches := controller.ID != nil && *controller.ID == id
		duplicateMatch := controller.Status == "duplicate" && duplicateControllerMatches(record.value, id)
		if !matches && !duplicateMatch {
			continue
		}
		if matches {
			result.ControllerMatchRecords++
		}
		if record.id != nil && counts[*record.id] > 0 {
			continue
		}
		entry := ControlledPlanetEntry{Path: record.path, Kind: "controller-record", ID: record.id, Value: record.value, ListMembership: "not-observed"}
		if record.id != nil {
			_, entry.PlanetStatus, entry.PlanetCandidates = planets.lookup(*record.id)
			if result.ListOccurrences == 1 && result.UnsupportedListEntries == 0 {
				entry.ListMembership = "outside-list"
			}
		} else {
			entry.PlanetStatus = "unsupported-key"
			entry.ListMembership = "unresolved"
		}
		entry.readPlanet(record, colonies)
		result.Unlisted = append(result.Unlisted, entry)
	}
	return result, nil
}

func (result *SavedControlledPlanets) readList(path string, value clausewitz.Value, counts map[int]int) {
	result.ListOccurrences++
	switch {
	case value.IsList():
		for i, item := range value.List {
			entry := ControlledPlanetEntry{Path: path + "/" + strconv.Itoa(i), Kind: "unsupported-reference", Value: item}
			if id := controlledPlanetID(item); id != nil {
				entry.ID, entry.Kind = id, "planet-reference"
				counts[*id]++
			} else {
				result.UnsupportedListEntries++
			}
			result.Listed = append(result.Listed, entry)
		}
	case value.IsObject() && len(value.Object.Pairs) == 0:
		result.Listed = append(result.Listed, ControlledPlanetEntry{Path: path, Kind: "empty-list", Value: value})
	default:
		result.UnsupportedListEntries++
		result.Listed = append(result.Listed, ControlledPlanetEntry{Path: path, Kind: "unsupported-list", Value: value})
	}
}

func controlledPlanetID(value clausewitz.Value) *int {
	if value.IsScalar() {
		if id, err := strconv.Atoi(value.Scalar); err == nil && id >= 0 {
			return &id
		}
	}
	return nil
}

func indexControlledPlanetTable(value clausewitz.Value, fields ...string) controlledPlanetTable {
	table := controlledPlanetTable{byID: map[int][]int{}}
	path := ""
	for _, field := range fields {
		entry := controlledPlanetRawField(value, path, field)
		if entry.Status != "unique" || !entry.Value.IsObject() {
			table.Status, table.Path = entry.Status, entry.Path
			if table.Status == "unique" {
				table.Status = "unsupported"
			}
			return table
		}
		value, path = entry.Value, entry.Path
	}
	table.Status, table.Path = "recorded", path
	for i, pair := range value.Object.Pairs {
		record := controlledPlanetRecord{path: path + "/" + strconv.Itoa(i), value: pair.Value}
		if id, err := strconv.Atoi(pair.Key); err == nil && id >= 0 {
			record.id = &id
			table.byID[id] = append(table.byID[id], i)
		} else {
			table.UnindexedKeys++
			table.Status = "partial"
		}
		table.records = append(table.records, record)
	}
	return table
}

func (table controlledPlanetTable) lookup(id int) (*controlledPlanetRecord, string, int) {
	indexes := table.byID[id]
	switch {
	case table.Status != "recorded" && table.Status != "partial":
		return nil, "table-" + table.Status, 0
	case len(indexes) > 1:
		return nil, "duplicate", len(indexes)
	case len(indexes) == 0 && table.Status == "partial":
		return nil, "unresolved", 0
	case len(indexes) == 0:
		return nil, "missing", 0
	default:
		record := &table.records[indexes[0]]
		if !record.value.IsObject() {
			return record, "unsupported", 1
		}
		return record, "unique", 1
	}
}

func (table controlledPlanetTable) candidatePaths(id int) []string {
	indexes := table.byID[id]
	paths := make([]string, 0, min(20, len(indexes)))
	for _, index := range indexes[:min(20, len(indexes))] {
		paths = append(paths, table.records[index].path)
	}
	return paths
}

func controlledPlanetField(value clausewitz.Value, path, key string) ControlledPlanetField {
	field := controlledPlanetRawField(value, path, key)
	if field.Status == "unique" {
		field.ID = controlledPlanetID(field.Value)
		if field.ID == nil {
			field.Status = "unsupported"
		} else {
			field.Status = "integer"
		}
	}
	return field
}

func controlledPlanetRawField(value clausewitz.Value, path, key string) ControlledPlanetField {
	field := ControlledPlanetField{Status: "not-recorded", Path: path}
	if !value.IsObject() {
		field.Status = "unsupported"
		return field
	}
	for i, pair := range value.Object.Pairs {
		if pair.Key != key {
			continue
		}
		if field.Status != "not-recorded" {
			return ControlledPlanetField{Status: "duplicate", Path: path}
		}
		field.Path = path + "/" + strconv.Itoa(i)
		field.Value, field.Status = pair.Value, "unique"
	}
	return field
}

func duplicateControllerMatches(value clausewitz.Value, countryID int) bool {
	for _, pair := range value.Object.Pairs {
		if pair.Key == "controller" {
			if id := controlledPlanetID(pair.Value); id != nil && *id == countryID {
				return true
			}
		}
	}
	return false
}

func (entry *ControlledPlanetEntry) readPlanet(record controlledPlanetRecord, colonies controlledPlanetTable) {
	entry.PlanetPath = record.path
	entry.Controller = controlledPlanetField(record.value, record.path, "controller")
	entry.Owner = controlledPlanetField(record.value, record.path, "owner")
	entry.Colony = controlledPlanetField(record.value, record.path, "colony")
	if entry.Colony.ID == nil {
		return
	}
	colony, status, _ := colonies.lookup(*entry.Colony.ID)
	entry.ColonyStatus = status
	if status == "duplicate" {
		entry.ColonyCandidatePaths = colonies.candidatePaths(*entry.Colony.ID)
	}
	if colony != nil {
		entry.ColonyPath = colony.path
	}
	if status != "unique" {
		return
	}
	carrier := controlledPlanetRawField(colony.value, colony.path, "carrier")
	if carrier.Status != "unique" {
		entry.ColonyStatus = "carrier-" + carrier.Status
		return
	}
	if !carrier.Value.IsObject() {
		entry.ColonyStatus = "carrier-unsupported"
		return
	}
	entry.CarrierType = savedFieldString(carrier.Value, "type")
	entry.CarrierReference = controlledPlanetField(carrier.Value, carrier.Path, "reference")
	switch {
	case !entry.CarrierType.Recorded || entry.CarrierType.Unsupported || entry.CarrierType.Value == "":
		entry.ColonyStatus = "carrier-type-unresolved"
	case entry.CarrierType.Value != "planet":
		entry.ColonyStatus = "other-carrier-type"
	case entry.CarrierReference.ID == nil:
		entry.ColonyStatus = "carrier-reference-" + entry.CarrierReference.Status
	case entry.ID == nil || entry.PlanetStatus != "unique":
		entry.ColonyStatus = "planet-identity-unresolved"
	case *entry.CarrierReference.ID != *entry.ID:
		entry.ColonyStatus = "carrier-reference-differs"
	default:
		entry.ColonyStatus = "reciprocal"
	}
}
