package gamestate

import (
	"archive/zip"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type ShipDesignCollectionEntry struct {
	Path, Key, Kind                  string
	Value                            clausewitz.Value
	ID                               *int
	RepeatedReference, RepeatedField bool
	DesignStatus, DesignPath         string
	ShipMatches                      int
}

type ShipDesignCollection struct {
	CountryID                                     int
	CollectionOccurrences, UnsupportedCollections int
	DesignFields, References                      int
	DesignTableStatus, ShipTableStatus            string
	DesignUnindexedKeys, ShipUnresolvedRecords    int
	Entries                                       []ShipDesignCollectionEntry
}

func LoadShipDesignCollectionFromArchive(archive *zip.Reader, countryID *int) (ShipDesignCollection, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return ShipDesignCollection{}, fmt.Errorf("cannot read selected gamestate")
	}
	doc, err := clausewitz.Parse(data)
	if err != nil {
		return ShipDesignCollection{}, fmt.Errorf("cannot parse selected gamestate")
	}
	id, country, path, err := selectSavedCountry(doc, countryID)
	if err != nil {
		return ShipDesignCollection{}, err
	}
	result := ShipDesignCollection{CountryID: id}
	counts := map[int]int{}
	for i, pair := range country.Object.Pairs {
		if pair.Key == "ship_design_collection" {
			result.readCollection(path+"/"+strconv.Itoa(i), pair.Value, counts)
		}
	}
	designs, designPath, designStatus := designCollectionTable(doc, "ship_design")
	result.DesignTableStatus = designStatus
	byDesign := map[int]designCollectionRecord{}
	if designStatus == "recorded" {
		for i, pair := range designs.Object.Pairs {
			id, err := strconv.Atoi(pair.Key)
			if err != nil || id < 0 {
				result.DesignUnindexedKeys++
				result.DesignTableStatus = "partial"
				continue
			}
			if counts[id] == 0 {
				continue
			}
			record := byDesign[id]
			record.occurrences++
			record.object, record.path = pair.Value.IsObject(), designPath+"/"+strconv.Itoa(i)
			byDesign[id] = record
		}
	}
	ships, _, shipStatus := designCollectionTable(doc, "ships")
	result.ShipTableStatus = shipStatus
	var shipMatches map[int]int
	if shipStatus == "recorded" {
		shipMatches, result.ShipUnresolvedRecords = countShipDesignCollectionMatches(ships, counts)
		if result.ShipUnresolvedRecords > 0 {
			result.ShipTableStatus = "partial"
		}
	}
	for i := range result.Entries {
		entry := &result.Entries[i]
		if entry.ID == nil {
			continue
		}
		id := *entry.ID
		entry.RepeatedReference = counts[id] > 1
		entry.ShipMatches = shipMatches[id]
		record := byDesign[id]
		switch {
		case designStatus != "recorded":
			entry.DesignStatus = "table-" + designStatus
		case record.occurrences > 1:
			entry.DesignStatus = "duplicate"
		case record.occurrences == 0 && result.DesignUnindexedKeys > 0:
			entry.DesignStatus = "unresolved"
		case record.occurrences == 0:
			entry.DesignStatus = "missing"
		case !record.object:
			entry.DesignStatus, entry.DesignPath = "nonobject", record.path
		default:
			entry.DesignStatus, entry.DesignPath = "unique", record.path
		}
	}
	return result, nil
}

func (result *ShipDesignCollection) readCollection(path string, value clausewitz.Value, counts map[int]int) {
	result.CollectionOccurrences++
	switch {
	case !value.IsObject():
		result.UnsupportedCollections++
		result.Entries = append(result.Entries, ShipDesignCollectionEntry{Path: path, Key: "ship_design_collection", Kind: "unsupported-collection", Value: value})
	case len(value.Object.Pairs) == 0:
		result.Entries = append(result.Entries, ShipDesignCollectionEntry{Path: path, Key: "ship_design_collection", Kind: "empty-collection", Value: value})
	default:
		fields := 0
		for _, pair := range value.Object.Pairs {
			if pair.Key == "ship_design" {
				fields++
			}
		}
		result.DesignFields += fields
		for i, pair := range value.Object.Pairs {
			entry := ShipDesignCollectionEntry{Path: path + "/" + strconv.Itoa(i), Key: pair.Key, Value: pair.Value}
			switch {
			case pair.Key != "ship_design":
				entry.Kind = "collection-field"
			case pair.Value.IsList() && len(pair.Value.List) > 0:
				for j, item := range pair.Value.List {
					child := entry
					child.Path += "/" + strconv.Itoa(j)
					child.Value, child.RepeatedField = item, fields > 1
					result.addDesignReference(child, counts)
				}
				continue
			case pair.Value.IsScalar():
				entry.RepeatedField = fields > 1
				result.addDesignReference(entry, counts)
				continue
			case pair.Value.IsList(), pair.Value.IsObject() && len(pair.Value.Object.Pairs) == 0:
				entry.Kind, entry.RepeatedField = "empty-design-list", fields > 1
			default:
				entry.Kind, entry.RepeatedField = "unsupported-design-list", fields > 1
			}
			result.Entries = append(result.Entries, entry)
		}
	}
}

func (result *ShipDesignCollection) addDesignReference(entry ShipDesignCollectionEntry, counts map[int]int) {
	entry.Kind = "unsupported-reference"
	if entry.Value.IsScalar() {
		if id, err := strconv.Atoi(entry.Value.Scalar); err == nil && id >= 0 {
			entry.ID, entry.Kind = &id, "design-reference"
			result.References++
			counts[id]++
		}
	}
	result.Entries = append(result.Entries, entry)
}

type designCollectionRecord struct {
	occurrences int
	object      bool
	path        string
}

func designCollectionTable(doc *clausewitz.Object, key string) (clausewitz.Value, string, string) {
	position := -1
	var value clausewitz.Value
	for i, pair := range doc.Pairs {
		if pair.Key != key {
			continue
		}
		if position >= 0 {
			return clausewitz.Value{}, "", "duplicate"
		}
		value, position = pair.Value, i
	}
	switch {
	case position < 0:
		return value, "", "not-recorded"
	case !value.IsObject():
		return value, "", "unsupported"
	default:
		return value, "/" + strconv.Itoa(position), "recorded"
	}
}

func countShipDesignCollectionMatches(ships clausewitz.Value, wanted map[int]int) (map[int]int, int) {
	shipKeys := map[int]int{}
	for _, pair := range ships.Object.Pairs {
		if id, err := strconv.Atoi(pair.Key); err == nil && id >= 0 {
			shipKeys[id]++
		}
	}
	matches, unresolved := map[int]int{}, 0
	for _, pair := range ships.Object.Pairs {
		id, err := strconv.Atoi(pair.Key)
		switch {
		case err != nil || id < 0 || shipKeys[id] != 1:
			unresolved++
			continue
		case pair.Value.IsScalar() && pair.Value.Scalar == "none":
			continue
		case !pair.Value.IsObject():
			unresolved++
			continue
		}
		legacy := savedPathInt(pair.Value, "ship_design")
		impl := savedPathInt(pair.Value, "ship_design_implementation", "design")
		design := unambiguousShipDesign(legacy, impl)
		if design == nil || *design < 0 {
			unresolved++
		} else if wanted[*design] > 0 {
			matches[*design]++
		}
	}
	return matches, unresolved
}
