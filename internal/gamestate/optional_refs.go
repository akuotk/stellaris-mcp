package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

// OptionalIntList isolates unsupported reference shapes from unrelated save data.
type OptionalIntList struct {
	Recorded    bool
	Values      []int
	Unsupported bool
}

func (v *OptionalIntList) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = OptionalIntList{Recorded: true}
	if err := clausewitz.DecodeValue(value, &v.Values); err != nil {
		v.Values = nil
		v.Unsupported = true
	}
	return nil
}

// OptionalCoordinate isolates unsupported coordinate shapes from unrelated save data.
type OptionalCoordinate struct {
	Recorded    bool
	Origin      *int `clausewitz:"origin"`
	Unsupported bool
}

func (v *OptionalCoordinate) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields OptionalCoordinate
	var decoded fields
	err := clausewitz.DecodeValue(value, &decoded)
	*v = OptionalCoordinate(decoded)
	v.Recorded = true
	if err != nil || uniqueSavedFields(value, "origin") != nil {
		v.Origin = nil
		v.Unsupported = true
	}
	return nil
}

func savedPathInt(value clausewitz.Value, path ...string) OptionalIntRef {
	field, recorded, unsupported := savedFieldPath(value, path...)
	ref := OptionalIntRef{Recorded: recorded, Unsupported: unsupported}
	if recorded && !unsupported {
		ref.UnmarshalClausewitz(field)
	}
	return ref
}

func savedFieldString(value clausewitz.Value, key string) OptionalString {
	field, recorded, unsupported := savedFieldPath(value, key)
	ref := OptionalString{Recorded: recorded, Unsupported: unsupported}
	if recorded && !unsupported {
		ref.UnmarshalClausewitz(field)
	}
	return ref
}

func savedFieldPath(value clausewitz.Value, path ...string) (clausewitz.Value, bool, bool) {
	for _, key := range path {
		if !value.IsObject() {
			return value, true, true
		}
		var next clausewitz.Value
		found := false
		for _, pair := range value.Object.Pairs {
			if pair.Key != key {
				continue
			}
			if found {
				return clausewitz.Value{}, true, true
			}
			next, found = pair.Value, true
		}
		if !found {
			return clausewitz.Value{}, false, false
		}
		value = next
	}
	return value, true, false
}
