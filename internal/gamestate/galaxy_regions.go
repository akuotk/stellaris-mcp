package gamestate

import (
	"archive/zip"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type SavedGalaxyRegion struct {
	Path, Status          string
	Value                 clausewitz.Value
	ObjectFields          int
	ObjectOffset, Objects int
}

type SavedGalaxyRegionObject struct {
	Path, Status        string
	Value               clausewitz.Value
	ID                  *int
	OccurrencesInParent int
}

type SavedGalaxyRegionSet struct {
	Roots       int
	Regions     []SavedGalaxyRegion
	Objects     []SavedGalaxyRegionObject
	ParentsByID map[int]int
}

type GalaxyRegionTableKey struct {
	Occurrences int
	Object      bool
}

type GalaxyRegionTable struct {
	Label                string
	Recorded, Incomplete bool
	Keys                 map[int]GalaxyRegionTableKey
}

type SavedGalaxyRegions struct {
	Clusters, Nebulae SavedGalaxyRegionSet
	Tables            []GalaxyRegionTable
}

func LoadGalaxyRegionsFromArchive(archive *zip.Reader) (SavedGalaxyRegions, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return SavedGalaxyRegions{}, err
	}
	root, err := clausewitz.Parse(data)
	if err != nil {
		return SavedGalaxyRegions{}, fmt.Errorf("parse gamestate: %w", err)
	}
	result := SavedGalaxyRegions{
		Clusters: SavedGalaxyRegionSet{ParentsByID: map[int]int{}},
		Nebulae:  SavedGalaxyRegionSet{ParentsByID: map[int]int{}},
	}
	for i, pair := range root.Pairs {
		switch pair.Key {
		case "clusters":
			result.Clusters.Roots++
			path := fmt.Sprintf("root[%d] key=%q", i, pair.Key)
			switch value := pair.Value; {
			case value.IsList():
				for j, item := range value.List {
					result.Clusters.addRegion(fmt.Sprintf("%s/item[%d]", path, j), item, "objects", "")
				}
			case value.IsObject() && len(value.Object.Pairs) == 0:
				result.Clusters.addRegion(path, value, "objects", "empty-root")
			default:
				result.Clusters.addRegion(path, value, "objects", "unsupported-root")
			}
		case "nebula":
			result.Nebulae.Roots++
			result.Nebulae.addRegion(fmt.Sprintf("root[%d] key=%q", i, pair.Key), pair.Value, "galactic_object", "")
		}
	}
	rootValue := clausewitz.Value{Object: root}
	result.Tables = []GalaxyRegionTable{
		regionCandidateTable(rootValue, result, "system", "galactic_object"),
		regionCandidateTable(rootValue, result, "planet", "planets", "planet"),
		regionCandidateTable(rootValue, result, "country", "country"),
	}
	return result, nil
}

func (set *SavedGalaxyRegionSet) addRegion(path string, value clausewitz.Value, fieldKey, status string) {
	if status == "" && !value.IsObject() {
		status = "unsupported-region"
	}
	region := SavedGalaxyRegion{Path: path, Value: value, Status: status, ObjectOffset: len(set.Objects)}
	if status != "" {
		set.Objects = append(set.Objects, SavedGalaxyRegionObject{Path: path, Status: status, Value: value})
	} else {
		for i, field := range value.Object.Pairs {
			if field.Key != fieldKey {
				continue
			}
			region.ObjectFields++
			fieldPath := fmt.Sprintf("%s/field[%d] key=%q", path, i, field.Key)
			switch {
			case fieldKey == "galactic_object":
				set.addObject(fieldPath, field.Value)
			case field.Value.IsList():
				for j, item := range field.Value.List {
					set.addObject(fmt.Sprintf("%s/item[%d]", fieldPath, j), item)
				}
			default:
				status := "unsupported-list"
				if field.Value.IsObject() && len(field.Value.Object.Pairs) == 0 {
					status = "empty-list"
				}
				set.Objects = append(set.Objects, SavedGalaxyRegionObject{Path: fieldPath, Status: status, Value: field.Value})
			}
		}
		if region.ObjectFields == 0 {
			set.Objects = append(set.Objects, SavedGalaxyRegionObject{Path: path, Status: "field-not-recorded"})
		}
	}
	counts := map[int]int{}
	for _, object := range set.Objects[region.ObjectOffset:] {
		if object.ID != nil {
			counts[*object.ID]++
		}
	}
	for id := range counts {
		set.ParentsByID[id]++
	}
	for i := region.ObjectOffset; i < len(set.Objects); i++ {
		if object := &set.Objects[i]; object.ID != nil {
			object.OccurrencesInParent = counts[*object.ID]
		}
	}
	region.Objects = len(set.Objects) - region.ObjectOffset
	set.Regions = append(set.Regions, region)
}

func (set *SavedGalaxyRegionSet) addObject(path string, value clausewitz.Value) {
	var ref OptionalIntRef
	ref.UnmarshalClausewitz(value)
	object := SavedGalaxyRegionObject{Path: path, Value: value, ID: ref.Value}
	if ref.Unsupported {
		object.Status = "unsupported-id"
	}
	set.Objects = append(set.Objects, object)
}

func regionCandidateTable(root clausewitz.Value, regions SavedGalaxyRegions, label string, path ...string) GalaxyRegionTable {
	value, recorded, unsupported := savedFieldPath(root, path...)
	table := GalaxyRegionTable{Label: label, Recorded: recorded, Incomplete: recorded && (unsupported || !value.IsObject())}
	if !recorded || table.Incomplete {
		return table
	}
	table.Keys = map[int]GalaxyRegionTableKey{}
	for _, pair := range value.Object.Pairs {
		id, err := strconv.Atoi(pair.Key)
		if err != nil {
			table.Incomplete = true
		} else if regions.Clusters.ParentsByID[id] > 0 || regions.Nebulae.ParentsByID[id] > 0 {
			key := table.Keys[id]
			key.Occurrences++
			key.Object = pair.Value.IsObject()
			table.Keys[id] = key
		}
	}
	return table
}
