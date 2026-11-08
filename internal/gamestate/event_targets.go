package gamestate

import (
	"archive/zip"
	"fmt"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type SavedEventTarget struct {
	Value      clausewitz.Value
	ID         OptionalIntRef
	Type, Name OptionalString
}

type SavedEventTargets struct {
	Records                                    []SavedEventTarget
	Countries, Systems, Planets, Ships, Fleets ConnectionRecordTable[SpaceObjectCandidate]
}

func LoadEventTargetsFromArchive(archive *zip.Reader) (SavedEventTargets, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return SavedEventTargets{}, err
	}
	root, err := clausewitz.Parse(data)
	if err != nil {
		return SavedEventTargets{}, fmt.Errorf("parse gamestate: %w", err)
	}
	return decodeEventTargets(clausewitz.Value{Object: root}), nil
}

func decodeEventTargets(root clausewitz.Value) SavedEventTargets {
	var result SavedEventTargets
	ids := map[int]bool{}
	if !root.IsObject() {
		return result
	}
	for _, pair := range root.Object.Pairs {
		if pair.Key != "saved_event_target" {
			continue
		}
		target := SavedEventTarget{Value: pair.Value}
		if pair.Value.IsObject() {
			target.ID = savedPathInt(pair.Value, "id")
			target.Type, target.Name = savedFieldString(pair.Value, "type"), savedFieldString(pair.Value, "name")
			if target.ID.Value != nil {
				ids[*target.ID.Value] = true
			}
		}
		result.Records = append(result.Records, target)
	}
	result.Countries = systemSubsetTable[SpaceObjectCandidate](root, ids, "country")
	result.Systems = systemSubsetTable[SpaceObjectCandidate](root, ids, "galactic_object")
	result.Planets = systemSubsetTable[SpaceObjectCandidate](root, ids, "planets", "planet")
	result.Ships = systemSubsetTable[SpaceObjectCandidate](root, ids, "ships")
	result.Fleets = systemSubsetTable[SpaceObjectCandidate](root, ids, "fleet")
	return result
}
