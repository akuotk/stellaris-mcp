package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type CountrySpeciesRightsModules struct {
	Recorded    bool
	Rights      SpeciesRightsModule `clausewitz:"standard_species_rights_module"`
	Unsupported bool
}

func (v *CountrySpeciesRightsModules) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = CountrySpeciesRightsModules{Recorded: true}
	type fields CountrySpeciesRightsModules
	var decoded fields
	if uniqueSavedFields(value, "standard_species_rights_module") != nil || clausewitz.DecodeValue(value, &decoded) != nil {
		v.Unsupported = true
		return nil
	}
	*v = CountrySpeciesRightsModules(decoded)
	v.Recorded = true
	return nil
}

type SpeciesRightsModule struct {
	Recorded    bool
	Enabled     OptionalString      `clausewitz:"enabled"`
	Default     SpeciesRightsRecord `clausewitz:"default"`
	Primary     SpeciesRightsRecord `clausewitz:"primary"`
	Built       SpeciesRightsRecord `clausewitz:"built_species"`
	Entries     SpeciesRightsList   `clausewitz:"species_rights"`
	Unsupported bool
}

func (v *SpeciesRightsModule) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = SpeciesRightsModule{Recorded: true}
	type fields SpeciesRightsModule
	var decoded fields
	if uniqueSavedFields(value, "enabled", "default", "primary", "built_species", "species_rights") != nil || clausewitz.DecodeValue(value, &decoded) != nil {
		v.Unsupported = true
		return nil
	}
	*v = SpeciesRightsModule(decoded)
	v.Recorded = true
	return nil
}

type SpeciesRightsList struct {
	Recorded    bool
	Items       []SpeciesRightsRecord
	Unsupported bool
}

func (v *SpeciesRightsList) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = SpeciesRightsList{Recorded: true}
	if !value.IsList() && (!value.IsObject() || len(value.Object.Pairs) != 0) {
		v.Unsupported = true
		return nil
	}
	if err := clausewitz.DecodeValue(value, &v.Items); err != nil {
		v.Items = nil
		v.Unsupported = true
	}
	return nil
}

type SpeciesRightsRecord struct {
	Recorded        bool
	Fields          []SpeciesRightsField
	SpeciesIndex    OptionalIntRef
	Unsupported     bool
	DuplicateFields bool
}

type SpeciesRightsField struct {
	Key   string
	Value string
}

func (v *SpeciesRightsRecord) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = SpeciesRightsRecord{Recorded: true}
	if !value.IsObject() {
		v.Unsupported = true
		return nil
	}
	seen := map[string]bool{}
	for _, pair := range value.Object.Pairs {
		duplicate := seen[pair.Key]
		seen[pair.Key] = true
		v.DuplicateFields = v.DuplicateFields || duplicate
		if pair.Key == "species_index" {
			if duplicate {
				v.SpeciesIndex = OptionalIntRef{Recorded: true, Unsupported: true}
			} else {
				v.SpeciesIndex.UnmarshalClausewitz(pair.Value)
			}
		}
		if !pair.Value.IsScalar() {
			v.Unsupported = true
			continue
		}
		v.Fields = append(v.Fields, SpeciesRightsField{Key: pair.Key, Value: pair.Value.Scalar})
	}
	return nil
}
