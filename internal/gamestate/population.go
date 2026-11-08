package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

// Parallel arrays preserve each save's IDs and raw population amounts without inferred scaling.
type PopEthos struct {
	Ethics []string `clausewitz:"ethics"`
	Pops   []int    `clausewitz:"pops"`
}

type PopFactionDistribution struct {
	Objects     []int `clausewitz:"objects"`
	Pops        []int `clausewitz:"pops"`
	Duplicate   bool  `clausewitz:"-"`
	Unsupported bool  `clausewitz:"-"`
}

func (d *PopFactionDistribution) UnmarshalClausewitz(value clausewitz.Value) error {
	if uniqueSavedFields(value, "objects", "pops") != nil {
		*d = PopFactionDistribution{Duplicate: true}
		return nil
	}
	type fields PopFactionDistribution
	if err := clausewitz.DecodeValue(value, (*fields)(d)); err != nil {
		*d = PopFactionDistribution{Unsupported: true}
	}
	return nil
}

type Species struct {
	Name         LocalizedName     `clausewitz:"name"`
	Class        string            `clausewitz:"class"`
	Portrait     string            `clausewitz:"portrait"`
	BaseRef      SpeciesRef        `clausewitz:"base_ref"`
	Home         SpeciesHome       `clausewitz:"home_planet"`
	SavedDetails []clausewitz.Pair `clausewitz:"-"`
	Traits       struct {
		Trait []string `clausewitz:"trait"`
	} `clausewitz:"traits"`
}

type SpeciesRef struct {
	OptionalIntRef
	Duplicate bool
}

type SpeciesRefList struct {
	OptionalIntList
	Duplicate bool
}

func (r *SpeciesRefList) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsList() && (!value.IsObject() || len(value.Object.Pairs) != 0) {
		r.OptionalIntList = OptionalIntList{Recorded: true, Unsupported: true}
		return nil
	}
	return r.OptionalIntList.UnmarshalClausewitz(value)
}

type SpeciesHome struct {
	Saved       clausewitz.Value
	Recorded    bool
	Duplicate   bool
	Unsupported bool
	CarrierType *string
	Reference   OptionalIntRef
}

func (h *SpeciesHome) UnmarshalClausewitz(value clausewitz.Value) error {
	*h = SpeciesHome{Saved: value, Recorded: true}
	if value.IsScalar() {
		return h.Reference.UnmarshalClausewitz(value)
	}
	if uniqueSavedFields(value, "type", "reference") != nil {
		h.Duplicate = true
		return nil
	}
	var fields struct {
		Type      *string        `clausewitz:"type"`
		Reference OptionalIntRef `clausewitz:"reference"`
	}
	if clausewitz.DecodeValue(value, &fields) != nil {
		h.Unsupported = true
		return nil
	}
	h.CarrierType, h.Reference = fields.Type, fields.Reference
	return nil
}

func (s *Species) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields Species
	if err := clausewitz.DecodeValue(value, (*fields)(s)); err != nil {
		return err
	}
	if uniqueSavedFields(value, "base_ref") != nil {
		s.BaseRef = SpeciesRef{Duplicate: true}
	}
	if uniqueSavedFields(value, "home_planet") != nil {
		s.Home = SpeciesHome{Duplicate: true}
	}
	for _, pair := range value.Object.Pairs {
		switch pair.Key {
		case "adjective", "plural", "gender", "sapient", "immortal", "name_list", "extra_trait_points", "uplifter":
			s.SavedDetails = append(s.SavedDetails, pair)
		}
	}
	return nil
}
