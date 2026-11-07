package gamestate

import (
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type Starbases struct {
	Recorded     bool
	Items        map[int]Starbase
	DecodeErrors map[int]string
	TableIssue   bool
}

type Starbase struct {
	Station            *int           `clausewitz:"station"`
	BuildQueue         *int           `clausewitz:"build_queue"`
	ShipyardBuildQueue *int           `clausewitz:"shipyard_build_queue"`
	ShipDesign         ShipDesignImpl `clausewitz:"ship_design_implementation"`
	Level              *string        `clausewitz:"level"`
	Type               *string        `clausewitz:"type"`
	ConstructionType   *string        `clausewitz:"construction_type"`
	Modules            StarbaseSlots  `clausewitz:"modules"`
	Buildings          StarbaseSlots  `clausewitz:"buildings"`
	Orbitals           StarbaseSlots  `clausewitz:"orbitals"`
}

type StarbaseSlots struct {
	Recorded    bool
	Entries     []StarbaseSlot
	Unsupported bool
}

type StarbaseSlot struct {
	Key   string
	Value string
}

// Keep ordered entries and explicit "none" values rather than map-decoding them away.
func (s *StarbaseSlots) UnmarshalClausewitz(value clausewitz.Value) error {
	*s = StarbaseSlots{Recorded: true, Entries: []StarbaseSlot{}}
	if !value.IsObject() {
		s.Unsupported = true
		return nil
	}
	for _, pair := range value.Object.Pairs {
		if _, err := strconv.Atoi(pair.Key); err != nil {
			s.Unsupported = true
			continue
		}
		if !pair.Value.IsScalar() {
			s.Unsupported = true
			continue
		}
		s.Entries = append(s.Entries, StarbaseSlot{Key: pair.Key, Value: pair.Value.Scalar})
	}
	return nil
}

func (s *Starbases) UnmarshalClausewitz(value clausewitz.Value) error {
	*s = Starbases{Recorded: true, Items: map[int]Starbase{}, DecodeErrors: map[int]string{}}
	if !value.IsObject() {
		s.TableIssue = true
		return nil
	}
	found := false
	seen := map[int]bool{}
	for _, wrapper := range value.Object.Pairs {
		if wrapper.Key != "starbases" {
			continue
		}
		if found {
			s.TableIssue = true
		}
		found = true
		if !wrapper.Value.IsObject() {
			s.TableIssue = true
			continue
		}
		for _, pair := range wrapper.Value.Object.Pairs {
			id, err := strconv.Atoi(pair.Key)
			if err != nil {
				s.TableIssue = true
				continue
			}
			if seen[id] {
				s.DecodeErrors[id] = "duplicate starbase ID"
				continue
			}
			seen[id] = true
			if pair.Value.IsScalar() && pair.Value.Scalar == "none" {
				continue
			}
			var base Starbase
			err = clausewitz.DecodeValue(pair.Value, &base)
			s.Items[id] = base
			if err != nil {
				s.DecodeErrors[id] = err.Error()
			}
		}
	}
	if !found && len(value.Object.Pairs) > 0 {
		s.TableIssue = true
	}
	return nil
}
