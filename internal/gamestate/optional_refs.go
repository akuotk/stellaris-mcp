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
	if err != nil {
		v.Origin = nil
		v.Unsupported = true
	}
	return nil
}
