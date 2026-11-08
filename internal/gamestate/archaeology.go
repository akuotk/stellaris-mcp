package gamestate

import (
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type ArchaeologySites struct {
	Items         map[int]ArchaeologySite
	DecodeErrors  map[int]string
	SitesRecorded bool
	TableIssue    bool
}

const duplicateArchaeologySiteID = "duplicate archaeology site ID"

func (s ArchaeologySites) HasAmbiguousID(id int) bool {
	return s.TableIssue || s.DecodeErrors[id] == duplicateArchaeologySiteID
}

type ArchaeologySite struct {
	Location struct {
		Type *string `clausewitz:"type"`
		ID   *int    `clausewitz:"id"`
	} `clausewitz:"location"`
	LastExcavatorCountry     *int                    `clausewitz:"-"`
	LastExcavatorUnsupported bool                    `clausewitz:"-"`
	ExcavatorFleet           *int                    `clausewitz:"excavator_fleet"`
	ExcavatorFleetAmbiguous  bool                    `clausewitz:"-"`
	Type                     *string                 `clausewitz:"type"`
	VisibleTo                []int                   `clausewitz:"visible_to"`
	Completed                []ArchaeologyCompletion `clausewitz:"completed"`
	Index                    *int                    `clausewitz:"index"`
	Clues                    *float64                `clausewitz:"clues"`
	LastRoll                 *float64                `clausewitz:"last_roll"`
	DaysLeft                 *float64                `clausewitz:"days_left"`
	Difficulty               *float64                `clausewitz:"difficulty"`
	Locked                   *bool                   `clausewitz:"locked"`
	Events                   []SavedArchaeologyEvent `clausewitz:"-"`
	EventsUnsupported        bool                    `clausewitz:"-"`
	Log                      []ArchaeologyLog        `clausewitz:"log"`
	Flags                    EspionageOperationFlags `clausewitz:"flags"`
}

type ArchaeologyCompletion struct {
	Country *int    `clausewitz:"country"`
	Date    *string `clausewitz:"date"`
}

func (s *ArchaeologySite) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields ArchaeologySite
	err := clausewitz.DecodeValue(value, (*fields)(s))
	lastExcavator := archaeologyIntRef(value, "last_excavator_country")
	s.LastExcavatorCountry = lastExcavator.Value
	s.LastExcavatorUnsupported = lastExcavator.Unsupported
	if uniqueSavedFields(value, "excavator_fleet") != nil {
		s.ExcavatorFleet = nil
		s.ExcavatorFleetAmbiguous = true
	}
	if !value.IsObject() {
		return err
	}
	if uniqueSavedFields(value, "events") != nil {
		s.EventsUnsupported = true
		return fmt.Errorf("duplicate archaeology events field")
	}
	for _, pair := range value.Object.Pairs {
		if pair.Key != "events" {
			continue
		}
		if decodeErr := clausewitz.DecodeValue(pair.Value, &s.Events); decodeErr != nil {
			s.EventsUnsupported = true
			return decodeErr
		}
		for _, event := range s.Events {
			if event.Unsupported || event.Scope != nil && event.Scope.Unsupported {
				return fmt.Errorf("unsupported archaeology event record or scope shape")
			}
		}
	}
	return err
}

type ArchaeologyEvent struct {
	Expired     *string                `clausewitz:"expired"`
	EventID     *string                `clausewitz:"event_id"`
	Scope       *ArchaeologyEventScope `clausewitz:"scope"`
	Fleet       *int                   `clausewitz:"fleet"`
	Owner       *int                   `clausewitz:"owner"`
	Effect      *string                `clausewitz:"effect"`
	Picture     *string                `clausewitz:"picture"`
	Index       *int                   `clausewitz:"index"`
	PlayerEvent *string                `clausewitz:"player_event"`
}

// Archaeology details retain individual events when an optional field is unsupported.
type SavedArchaeologyEvent struct {
	ArchaeologyEvent
	Value       clausewitz.Value
	OwnerRef    OptionalIntRef
	FleetRef    OptionalIntRef
	Unsupported bool
}

func (e *SavedArchaeologyEvent) UnmarshalClausewitz(value clausewitz.Value) error {
	*e = SavedArchaeologyEvent{Value: value}
	if !value.IsObject() {
		e.Unsupported = true
		return nil
	}
	fields := &clausewitz.Object{}
	for _, pair := range value.Object.Pairs {
		if pair.Key != "scope" && pair.Key != "fleet" && pair.Key != "owner" {
			fields.Pairs = append(fields.Pairs, pair)
		}
	}
	e.Unsupported = clausewitz.DecodeValue(clausewitz.Value{Object: fields}, &e.ArchaeologyEvent) != nil
	e.FleetRef = archaeologyIntRef(value, "fleet")
	e.OwnerRef = archaeologyIntRef(value, "owner")
	e.Fleet, e.Owner = e.FleetRef.Value, e.OwnerRef.Value
	if uniqueSavedFields(value, "scope") != nil {
		e.Scope = &ArchaeologyEventScope{Unsupported: true}
	} else {
		for _, pair := range value.Object.Pairs {
			if pair.Key == "scope" {
				e.Scope = &ArchaeologyEventScope{}
				e.Scope.UnmarshalClausewitz(pair.Value)
				break
			}
		}
	}
	return nil
}

// Preserve arbitrary scope fields and duplicate pairs without interpreting event scopes.
type ArchaeologyEventScope struct {
	Value       clausewitz.Value
	ID          OptionalIntRef
	FromID      OptionalIntRef
	Unsupported bool
}

func (s *ArchaeologyEventScope) UnmarshalClausewitz(value clausewitz.Value) error {
	*s = ArchaeologyEventScope{Value: value}
	if !value.IsObject() {
		s.Unsupported = true
		return fmt.Errorf("unsupported archaeology event scope shape")
	}
	stack := []clausewitz.Value{value}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.Mixed {
			s.Unsupported = true
			return fmt.Errorf("unsupported mixed archaeology event scope")
		}
		if node.IsObject() {
			for _, pair := range node.Object.Pairs {
				stack = append(stack, pair.Value)
			}
		} else if node.IsList() {
			stack = append(stack, node.List...)
		}
	}
	s.ID = archaeologyIntRef(value, "id")
	if uniqueSavedFields(value, "from") != nil {
		s.FromID = OptionalIntRef{Recorded: true, Unsupported: true}
	} else {
		for _, pair := range value.Object.Pairs {
			if pair.Key == "from" {
				if pair.Value.IsObject() {
					s.FromID = archaeologyIntRef(pair.Value, "id")
				} else {
					s.FromID = OptionalIntRef{Recorded: true, Unsupported: true}
				}
			}
		}
	}
	return nil
}

func archaeologyIntRef(value clausewitz.Value, key string) OptionalIntRef {
	var ref OptionalIntRef
	if value.IsObject() {
		for _, pair := range value.Object.Pairs {
			if pair.Key != key {
				continue
			}
			if ref.Recorded {
				return OptionalIntRef{Recorded: true, Unsupported: true}
			}
			ref.UnmarshalClausewitz(pair.Value)
		}
	}
	return ref
}

type ArchaeologyLog struct {
	Title      *string  `clausewitz:"title"`
	Tooltip    *string  `clausewitz:"tooltip"`
	Clues      *float64 `clausewitz:"clues"`
	Total      *float64 `clausewitz:"total"`
	Roll       *float64 `clausewitz:"roll"`
	Bonus      *float64 `clausewitz:"bonus"`
	Difficulty *float64 `clausewitz:"difficulty"`
	Date       *string  `clausewitz:"date"`
}

func (s *ArchaeologySites) UnmarshalClausewitz(value clausewitz.Value) error {
	*s = ArchaeologySites{Items: map[int]ArchaeologySite{}, DecodeErrors: map[int]string{}}
	if !value.IsObject() {
		s.TableIssue = true
		return nil
	}
	seen := map[int]bool{}
	for _, wrapper := range value.Object.Pairs {
		if wrapper.Key != "sites" {
			continue
		}
		if s.SitesRecorded {
			s.TableIssue = true
		}
		s.SitesRecorded = true
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
			duplicate := seen[id]
			seen[id] = true
			if duplicate {
				s.DecodeErrors[id] = duplicateArchaeologySiteID
			}
			if pair.Value.IsScalar() && pair.Value.Scalar == "none" {
				continue
			}
			var site ArchaeologySite
			err = clausewitz.DecodeValue(pair.Value, &site)
			s.Items[id] = site
			if err != nil && !duplicate {
				s.DecodeErrors[id] = err.Error()
			}
		}
	}
	if !s.SitesRecorded && len(value.Object.Pairs) > 0 {
		s.TableIssue = true
	}
	return nil
}
