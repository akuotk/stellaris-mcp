package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type CountryDiplomaticRefs struct {
	OptionalIntList
	Duplicate bool
}

type CountrySpyNetworkRefs struct{ CountryDiplomaticRefs }
type CountryFirstContactRefs struct{ CountryDiplomaticRefs }

func (r *CountrySpyNetworkRefs) UnmarshalClausewitz(value clausewitz.Value) error {
	r.CountryDiplomaticRefs = decodeCountryDiplomaticRefs(value, "spy_networks")
	return nil
}

func (r *CountryFirstContactRefs) UnmarshalClausewitz(value clausewitz.Value) error {
	r.CountryDiplomaticRefs = decodeCountryDiplomaticRefs(value, "contacts")
	return nil
}

func decodeCountryDiplomaticRefs(value clausewitz.Value, key string) CountryDiplomaticRefs {
	var refs CountryDiplomaticRefs
	if !value.IsObject() {
		refs.OptionalIntList = OptionalIntList{Recorded: true, Unsupported: true}
		return refs
	}
	for _, pair := range value.Object.Pairs {
		if pair.Key != key {
			continue
		}
		if refs.Recorded {
			return duplicateCountryDiplomaticRefs()
		}
		if pair.Value.IsList() || (pair.Value.IsObject() && len(pair.Value.Object.Pairs) == 0) {
			refs.OptionalIntList.UnmarshalClausewitz(pair.Value)
		} else {
			refs.OptionalIntList = OptionalIntList{Recorded: true, Unsupported: true}
		}
	}
	return refs
}

func duplicateCountryDiplomaticRefs() CountryDiplomaticRefs {
	return CountryDiplomaticRefs{OptionalIntList: OptionalIntList{Recorded: true}, Duplicate: true}
}

type SpyNetwork struct {
	Owner            *int                     `clausewitz:"owner"`
	Target           *int                     `clausewitz:"target"`
	Leader           *int                     `clausewitz:"leader"`
	Power            *string                  `clausewitz:"power"`
	ActiveOperations []int                    `clausewitz:"active_operations"`
	Formed           string                   `clausewitz:"formed"`
	TimedModifiers   SpyNetworkTimedModifiers `clausewitz:"timed_modifiers"`
}

func (n *SpyNetwork) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "owner", "target", "leader", "power", "formed"); err != nil {
		return err
	}
	type fields SpyNetwork
	if err := clausewitz.DecodeValue(value, (*fields)(n)); err != nil {
		return err
	}
	if uniqueSavedFields(value, "timed_modifiers") != nil {
		n.TimedModifiers = SpyNetworkTimedModifiers{Recorded: true, Duplicate: true}
	}
	return nil
}

type SpyNetworkTimedModifiers struct {
	Items       []SpyNetworkTimedModifier `clausewitz:"items"`
	Recorded    bool
	Unsupported bool
	Duplicate   bool
}

func (m *SpyNetworkTimedModifiers) UnmarshalClausewitz(value clausewitz.Value) error {
	*m = SpyNetworkTimedModifiers{Recorded: true}
	if uniqueSavedFields(value, "items") != nil {
		m.Duplicate = true
		return nil
	}
	if value.IsObject() {
		for _, pair := range value.Object.Pairs {
			if pair.Key == "items" && !pair.Value.IsList() && (!pair.Value.IsObject() || len(pair.Value.Object.Pairs) != 0) {
				m.Unsupported = true
				return nil
			}
		}
	}
	type fields SpyNetworkTimedModifiers
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		m.Unsupported = true
		return nil
	}
	m.Items = decoded.Items
	return nil
}

type SpyNetworkTimedModifier struct {
	Modifier    *string `clausewitz:"modifier"`
	Days        *string `clausewitz:"days"`
	Duplicate   bool
	Unsupported bool
}

func (m *SpyNetworkTimedModifier) UnmarshalClausewitz(value clausewitz.Value) error {
	if uniqueSavedFields(value, "modifier", "days") != nil {
		*m = SpyNetworkTimedModifier{Duplicate: true}
		return nil
	}
	type fields SpyNetworkTimedModifier
	if err := clausewitz.DecodeValue(value, (*fields)(m)); err != nil {
		*m = SpyNetworkTimedModifier{Unsupported: true}
	}
	return nil
}
