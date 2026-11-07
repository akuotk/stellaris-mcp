package gamestate

import (
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type OptionalIntRef struct {
	Recorded    bool
	Value       *int
	Unsupported bool
}

func (v *OptionalIntRef) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = OptionalIntRef{Recorded: true}
	var id int
	if err := clausewitz.DecodeValue(value, &id); err != nil {
		v.Unsupported = true
	} else {
		v.Value = &id
	}
	return nil
}

type CountrySectorRefs struct {
	Recorded    bool
	Owned       OptionalIntList `clausewitz:"owned"`
	Unsupported bool
}

func (v *CountrySectorRefs) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = CountrySectorRefs{Recorded: true}
	if uniqueSavedFields(value, "owned") != nil {
		v.Unsupported = true
		return nil
	}
	type fields CountrySectorRefs
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		v.Unsupported = true
		return nil
	}
	*v = CountrySectorRefs(decoded)
	v.Recorded = true
	return nil
}

type Sectors struct {
	Recorded     bool
	Items        map[int]Sector
	DecodeErrors map[int]string
	TableIssue   bool
}

type Sector struct {
	Name         LocalizedName   `clausewitz:"name"`
	Owner        *int            `clausewitz:"owner"`
	LocalCapital *int            `clausewitz:"local_capital"`
	Type         *string         `clausewitz:"type"`
	AutoExpand   *string         `clausewitz:"auto_expand"`
	Systems      OptionalIntList `clausewitz:"systems"`
}

func (s *Sector) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "name", "owner", "local_capital", "type", "auto_expand", "systems"); err != nil {
		return err
	}
	type fields Sector
	return clausewitz.DecodeValue(value, (*fields)(s))
}

func (s *Sectors) UnmarshalClausewitz(value clausewitz.Value) error {
	*s = Sectors{Recorded: true, Items: map[int]Sector{}, DecodeErrors: map[int]string{}}
	if !value.IsObject() {
		s.TableIssue = true
		return nil
	}
	seen := map[int]bool{}
	for _, pair := range value.Object.Pairs {
		id, err := strconv.Atoi(pair.Key)
		if err != nil {
			s.TableIssue = true
			continue
		}
		if seen[id] {
			delete(s.Items, id)
			s.DecodeErrors[id] = "duplicate sector ID"
			continue
		}
		seen[id] = true
		if pair.Value.IsScalar() && pair.Value.Scalar == "none" {
			continue
		}
		var sector Sector
		if err := clausewitz.DecodeValue(pair.Value, &sector); err != nil {
			s.DecodeErrors[id] = err.Error()
			continue
		}
		s.Items[id] = sector
	}
	return nil
}
