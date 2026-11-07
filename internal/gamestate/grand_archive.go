package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type CountryGrandArchiveModules struct {
	Recorded    bool
	Archive     *GrandArchiveModule `clausewitz:"standard_grand_archive_module"`
	Unsupported bool
}

func (v *CountryGrandArchiveModules) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = CountryGrandArchiveModules{Recorded: true}
	type fields CountryGrandArchiveModules
	var decoded fields
	if uniqueSavedFields(value, "standard_grand_archive_module") != nil || clausewitz.DecodeValue(value, &decoded) != nil {
		v.Unsupported = true
		return nil
	}
	*v = CountryGrandArchiveModules(decoded)
	v.Recorded = true
	return nil
}

type GrandArchiveModule struct {
	Exhibits         *GrandArchiveValue `clausewitz:"exhibits"`
	AutoCulling      *GrandArchiveValue `clausewitz:"auto_culling_settings"`
	CullingValue     *GrandArchiveValue `clausewitz:"culling_value"`
	SpecimenSettings *GrandArchiveValue `clausewitz:"specimen_management_settings"`
	Unsupported      bool
}

func (v *GrandArchiveModule) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = GrandArchiveModule{}
	type fields GrandArchiveModule
	var decoded fields
	if uniqueSavedFields(value, "exhibits", "auto_culling_settings", "culling_value", "specimen_management_settings") != nil || clausewitz.DecodeValue(value, &decoded) != nil {
		v.Unsupported = true
		return nil
	}
	*v = GrandArchiveModule(decoded)
	return nil
}

// Keep ordered saved fields and list positions without assigning specimen roles.
type GrandArchiveValue struct{ clausewitz.Value }

func (v *GrandArchiveValue) UnmarshalClausewitz(value clausewitz.Value) error {
	v.Value = value
	return nil
}
