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

type NestedSavedEventTarget struct {
	Path   string
	Target SavedEventTarget
}

func decodeSavedEventTarget(value clausewitz.Value) SavedEventTarget {
	target := SavedEventTarget{Value: value}
	if value.IsObject() {
		target.ID = savedPathInt(value, "id")
		target.Type, target.Name = savedFieldString(value, "type"), savedFieldString(value, "name")
	}
	return target
}

func rootEventTargets(root clausewitz.Value) []SavedEventTarget {
	var records []SavedEventTarget
	if !root.IsObject() {
		return records
	}
	for _, pair := range root.Object.Pairs {
		if pair.Key == "saved_event_target" {
			records = append(records, decodeSavedEventTarget(pair.Value))
		}
	}
	return records
}

// NestedEventTargets retains positions while leaving mixed blocks unresolved.
func NestedEventTargets(value clausewitz.Value) ([]NestedSavedEventTarget, bool) {
	type item struct {
		path   string
		value  clausewitz.Value
		target bool
	}
	stack := []item{{path: "", value: value}}
	var result []NestedSavedEventTarget
	incomplete := false
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current.target {
			result = append(result, NestedSavedEventTarget{Path: current.path, Target: decodeSavedEventTarget(current.value)})
		}
		switch {
		case current.value.Mixed:
			incomplete = true
		case current.value.IsObject():
			pairs := current.value.Object.Pairs
			for i := len(pairs) - 1; i >= 0; i-- {
				pair := pairs[i]
				path := fmt.Sprintf("%s/field[%d]=%s", current.path, i, pair.Key)
				stack = append(stack, item{path: path, value: pair.Value, target: pair.Key == "saved_event_target"})
			}
		case current.value.IsList():
			for i := len(current.value.List) - 1; i >= 0; i-- {
				stack = append(stack, item{path: fmt.Sprintf("%s/item[%d]", current.path, i), value: current.value.List[i]})
			}
		}
	}
	return result, incomplete
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
	result.Records = rootEventTargets(root)
	for _, target := range result.Records {
		if target.ID.Value != nil {
			ids[*target.ID.Value] = true
		}
	}
	result.Countries = systemSubsetTable[SpaceObjectCandidate](root, ids, "country")
	result.Systems = systemSubsetTable[SpaceObjectCandidate](root, ids, "galactic_object")
	result.Planets = systemSubsetTable[SpaceObjectCandidate](root, ids, "planets", "planet")
	result.Ships = systemSubsetTable[SpaceObjectCandidate](root, ids, "ships")
	result.Fleets = systemSubsetTable[SpaceObjectCandidate](root, ids, "fleet")
	return result
}
