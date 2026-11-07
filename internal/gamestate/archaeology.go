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

type ArchaeologySite struct {
	Location struct {
		Type *string `clausewitz:"type"`
		ID   *int    `clausewitz:"id"`
	} `clausewitz:"location"`
	LastExcavatorCountry    *int                    `clausewitz:"last_excavator_country"`
	ExcavatorFleet          *int                    `clausewitz:"excavator_fleet"`
	ExcavatorFleetAmbiguous bool                    `clausewitz:"-"`
	Type                    *string                 `clausewitz:"type"`
	VisibleTo               []int                   `clausewitz:"visible_to"`
	Completed               []ArchaeologyCompletion `clausewitz:"completed"`
	Index                   *int                    `clausewitz:"index"`
	Clues                   *float64                `clausewitz:"clues"`
	LastRoll                *float64                `clausewitz:"last_roll"`
	DaysLeft                *float64                `clausewitz:"days_left"`
	Difficulty              *float64                `clausewitz:"difficulty"`
	Locked                  *bool                   `clausewitz:"locked"`
	Events                  []ArchaeologyEvent      `clausewitz:"events"`
	Log                     []ArchaeologyLog        `clausewitz:"log"`
	Flags                   EspionageOperationFlags `clausewitz:"flags"`
}

type ArchaeologyCompletion struct {
	Country *int    `clausewitz:"country"`
	Date    *string `clausewitz:"date"`
}

func (s *ArchaeologySite) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields ArchaeologySite
	err := clausewitz.DecodeValue(value, (*fields)(s))
	if uniqueSavedFields(value, "excavator_fleet") != nil {
		s.ExcavatorFleet = nil
		s.ExcavatorFleetAmbiguous = true
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

// Preserve arbitrary scope fields and duplicate pairs without interpreting event scopes.
type ArchaeologyEventScope struct{ Value clausewitz.Value }

func (s *ArchaeologyEventScope) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsObject() {
		return fmt.Errorf("unsupported archaeology event scope shape")
	}
	stack := []clausewitz.Value{value}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.Mixed {
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
	s.Value = value
	return nil
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
				s.DecodeErrors[id] = "duplicate archaeology site ID"
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
