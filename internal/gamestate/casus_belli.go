package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type CountryDiplomaticModules struct {
	Recorded    bool
	Standard    CasusBelliModule `clausewitz:"standard_diplomacy_module"`
	Exclusive   CasusBelliModule `clausewitz:"exclusive_diplomacy_module"`
	Unsupported bool
}

func (v *CountryDiplomaticModules) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = CountryDiplomaticModules{Recorded: true}
	if uniqueSavedFields(value, "standard_diplomacy_module", "exclusive_diplomacy_module") != nil {
		v.Unsupported = true
		return nil
	}
	type fields CountryDiplomaticModules
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		v.Unsupported = true
		return nil
	}
	*v = CountryDiplomaticModules(decoded)
	v.Recorded = true
	return nil
}

type CasusBelliModule struct {
	Recorded    bool
	CasusBelli  CasusBelliList `clausewitz:"casus_belli"`
	Unsupported bool
}

func (v *CasusBelliModule) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = CasusBelliModule{Recorded: true}
	if uniqueSavedFields(value, "casus_belli") != nil {
		v.Unsupported = true
		return nil
	}
	type fields CasusBelliModule
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		v.Unsupported = true
		return nil
	}
	*v = CasusBelliModule(decoded)
	v.Recorded = true
	return nil
}

type CasusBelliList struct {
	Recorded    bool
	Items       []CasusBelli
	Unsupported bool
}

type CasusBelli struct {
	Owner    *int    `clausewitz:"owner"`
	Country  *int    `clausewitz:"country"`
	Type     *string `clausewitz:"type"`
	DaysLeft *string `clausewitz:"days_left"`
}

func (v *CasusBelli) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "owner", "country", "type", "days_left"); err != nil {
		return err
	}
	type fields CasusBelli
	return clausewitz.DecodeValue(value, (*fields)(v))
}

func (v *CasusBelliList) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = CasusBelliList{Recorded: true}
	if value.IsObject() && len(value.Object.Pairs) > 0 {
		v.Unsupported = true
		return nil
	}
	if err := clausewitz.DecodeValue(value, &v.Items); err != nil {
		v.Items = nil
		v.Unsupported = true
	}
	return nil
}
