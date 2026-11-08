package gamestate

import (
	"archive/zip"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type SavedSpaceObject struct {
	ID                    *int
	Key                   string
	Value                 clausewitz.Value
	Fleet, PositionOrigin OptionalIntRef
}

type SavedSpaceObjects struct {
	Recorded, Unsupported bool
	NoneCount             int
	Records               []SavedSpaceObject
	NumericKeyCounts      map[int]int
	Fleets, Systems       ConnectionRecordTable[SpaceObjectCandidate]
}

type SpaceObjectCandidate struct{}

func (*SpaceObjectCandidate) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsObject() {
		return fmt.Errorf("unsupported candidate record")
	}
	return nil
}

func LoadSpaceObjectsFromArchive(archive *zip.Reader, kind string) (SavedSpaceObjects, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return SavedSpaceObjects{}, err
	}
	root, err := clausewitz.Parse(data)
	if err != nil {
		return SavedSpaceObjects{}, fmt.Errorf("parse gamestate: %w", err)
	}
	return decodeSpaceObjects(clausewitz.Value{Object: root}, kind), nil
}

func decodeSpaceObjects(root clausewitz.Value, kind string) SavedSpaceObjects {
	value, recorded, unsupported := savedFieldPath(root, kind)
	result := SavedSpaceObjects{Recorded: recorded, Unsupported: unsupported}
	if !recorded || unsupported {
		return result
	}
	if !value.IsObject() {
		result.Unsupported = true
		return result
	}
	result.NumericKeyCounts = map[int]int{}
	fleetIDs, systemIDs := map[int]bool{}, map[int]bool{}
	for _, pair := range value.Object.Pairs {
		id, idErr := strconv.Atoi(pair.Key)
		if idErr == nil {
			result.NumericKeyCounts[id]++
		}
		if pair.Value.IsScalar() && pair.Value.Scalar == "none" {
			result.NoneCount++
			continue
		}
		entry := SavedSpaceObject{Key: pair.Key, Value: pair.Value}
		if pair.Value.IsObject() {
			entry.Fleet = savedPathInt(pair.Value, "fleet")
			entry.PositionOrigin = savedPathInt(pair.Value, "position", "origin")
			if entry.Fleet.Value != nil {
				fleetIDs[*entry.Fleet.Value] = true
			}
			if entry.PositionOrigin.Value != nil {
				systemIDs[*entry.PositionOrigin.Value] = true
			}
		}
		if idErr == nil {
			entry.ID = &id
		}
		result.Records = append(result.Records, entry)
	}
	result.Fleets = systemSubsetTable[SpaceObjectCandidate](root, fleetIDs, "fleet")
	result.Systems = systemSubsetTable[SpaceObjectCandidate](root, systemIDs, "galactic_object")
	return result
}
