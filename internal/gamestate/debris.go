package gamestate

import (
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type DebrisTable struct {
	Recorded    bool
	Unsupported bool
	InvalidKeys int
	Items       map[int][]SavedDebris
}

type SavedDebris struct {
	Value   clausewitz.Value
	Country OptionalIntRef
}

func (v *DebrisTable) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = DebrisTable{Recorded: true}
	if !value.IsObject() {
		v.Unsupported = true
		return nil
	}
	v.Items = make(map[int][]SavedDebris, len(value.Object.Pairs))
	for _, pair := range value.Object.Pairs {
		id, err := strconv.Atoi(pair.Key)
		if err != nil {
			v.InvalidKeys++
			continue
		}
		record := SavedDebris{Value: pair.Value}
		if pair.Value.IsObject() {
			for _, field := range pair.Value.Object.Pairs {
				if field.Key != "country" {
					continue
				}
				if record.Country.Recorded {
					record.Country = OptionalIntRef{Recorded: true, Unsupported: true}
				} else {
					record.Country.UnmarshalClausewitz(field.Value)
				}
			}
		}
		v.Items[id] = append(v.Items[id], record)
	}
	return nil
}

func (r SavedDebris) IsSavedNone() bool {
	return r.Value.IsScalar() && r.Value.Scalar == "none"
}
