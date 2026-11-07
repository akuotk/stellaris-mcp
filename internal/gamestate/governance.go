package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type ActivePolicyList struct {
	Recorded    bool
	Items       []ActivePolicy
	Unsupported bool
}

type ActivePolicy struct {
	Policy   *string `clausewitz:"policy"`
	Selected *string `clausewitz:"selected"`
	Date     *string `clausewitz:"date"`
}

func (v *ActivePolicyList) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = ActivePolicyList{Recorded: true}
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

type EdictList struct {
	Recorded    bool
	Items       []Edict
	Unsupported bool
}

type Edict struct {
	Edict     *string `clausewitz:"edict"`
	Date      *string `clausewitz:"date"`
	StartDate *string `clausewitz:"start_date"`
	Perpetual *string `clausewitz:"perpetual"`
}

func (v *EdictList) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = EdictList{Recorded: true}
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
