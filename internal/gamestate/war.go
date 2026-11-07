package gamestate

import (
	"fmt"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type War struct {
	Value           clausewitz.Value
	Name            LocalizedName
	NameUnsupported bool
	StartDate       OptionalString
	EndDate         OptionalString
	Attackers       SavedWarList
	Defenders       SavedWarList
	Battles         SavedWarList
}

type SavedWarList struct {
	Items       []clausewitz.Value
	Recorded    bool
	Unsupported bool
}

func (w *War) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsObject() {
		return fmt.Errorf("unsupported saved war record shape")
	}
	*w = War{
		Value: value, StartDate: savedWarString(value, "start_date"), EndDate: savedWarString(value, "end_date"),
		Attackers: savedWarList(value, "attackers", true), Defenders: savedWarList(value, "defenders", true), Battles: savedWarList(value, "battles", false),
	}
	if field, recorded, duplicate := savedWarField(value, "name"); recorded {
		w.NameUnsupported = duplicate || uniqueSavedFields(field, "key", "variables") != nil || clausewitz.DecodeValue(field, &w.Name) != nil
	}
	return nil
}

func savedWarField(value clausewitz.Value, key string) (field clausewitz.Value, recorded, duplicate bool) {
	if !value.IsObject() {
		return field, false, false
	}
	for _, pair := range value.Object.Pairs {
		if pair.Key == key {
			if recorded {
				return field, true, true
			}
			field, recorded = pair.Value, true
		}
	}
	return field, recorded, false
}

func savedWarString(value clausewitz.Value, key string) OptionalString {
	field, recorded, duplicate := savedWarField(value, key)
	ref := OptionalString{Recorded: recorded, Unsupported: duplicate}
	if recorded && !duplicate {
		ref.UnmarshalClausewitz(field)
	}
	return ref
}

func savedWarList(value clausewitz.Value, key string, allowSingleObject bool) SavedWarList {
	field, recorded, duplicate := savedWarField(value, key)
	list := SavedWarList{Recorded: recorded}
	if recorded {
		if !duplicate && allowSingleObject && field.IsObject() && len(field.Object.Pairs) > 0 {
			// Older war-party decoding also accepted a single keyed record.
			list.Items = []clausewitz.Value{field}
			return list
		}
		list.Unsupported = duplicate || (!field.IsList() && (!field.IsObject() || len(field.Object.Pairs) != 0))
		if !list.Unsupported {
			list.Items = field.List
		}
	}
	return list
}

func SavedWarIntRef(value clausewitz.Value, key string) OptionalIntRef {
	field, recorded, duplicate := savedWarField(value, key)
	ref := OptionalIntRef{Recorded: recorded, Unsupported: duplicate}
	if recorded && !duplicate {
		ref.UnmarshalClausewitz(field)
	}
	return ref
}

func SavedWarBattleCountries(value clausewitz.Value, key string) OptionalIntList {
	list := savedWarList(value, key, false)
	ref := OptionalIntList{Recorded: list.Recorded, Unsupported: list.Unsupported}
	if !list.Unsupported && len(list.Items) > 0 {
		ref.UnmarshalClausewitz(clausewitz.Value{List: list.Items})
	}
	return ref
}
