package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type CountrySpecialProjects struct {
	Recorded      bool
	Items         []SavedSpecialProject
	SituationRefs OptionalIntList
	EventChains   []SavedEventChainEntry
	Unsupported   bool
}

type SavedEventChainEntry struct {
	Kind  string
	Value clausewitz.Value
}

type SavedSpecialProject struct {
	Value       clausewitz.Value
	ID          OptionalIntRef
	Planet      OptionalIntRef
	Debris      OptionalIntRef
	Unsupported bool
}

func (v *CountrySpecialProjects) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = CountrySpecialProjects{Recorded: true}
	if !value.IsObject() {
		v.Unsupported = true
		return nil
	}
	for _, pair := range value.Object.Pairs {
		switch pair.Key {
		case "special_project":
			v.Items = append(v.Items, decodeSavedSpecialProject(pair.Value))
		case "situations":
			if v.SituationRefs.Recorded {
				v.SituationRefs = OptionalIntList{Recorded: true, Unsupported: true}
			} else {
				v.SituationRefs.UnmarshalClausewitz(pair.Value)
			}
		case "event_chain", "completed_event_chain":
			v.EventChains = append(v.EventChains, SavedEventChainEntry{Kind: pair.Key, Value: pair.Value})
		}
	}
	return nil
}

func decodeSavedSpecialProject(value clausewitz.Value) SavedSpecialProject {
	project := SavedSpecialProject{Value: value}
	if !value.IsObject() {
		project.Unsupported = true
		return project
	}
	for _, pair := range value.Object.Pairs {
		var ref *OptionalIntRef
		switch pair.Key {
		case "id":
			ref = &project.ID
		case "planet":
			ref = &project.Planet
		case "debris":
			ref = &project.Debris
		default:
			continue
		}
		if ref.Recorded {
			*ref = OptionalIntRef{Recorded: true, Unsupported: true}
		} else {
			ref.UnmarshalClausewitz(pair.Value)
		}
	}
	return project
}
