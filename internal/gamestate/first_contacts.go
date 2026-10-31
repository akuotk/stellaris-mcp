package gamestate

import (
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type FirstContacts struct {
	Items        map[int]FirstContact
	DecodeErrors map[int]string
	TableIssue   bool
}

type FirstContact struct {
	Owner      *int                     `clausewitz:"owner"`
	Country    *int                     `clausewitz:"country"`
	Name       LocalizedName            `clausewitz:"name"`
	Location   *int                     `clausewitz:"location"`
	Leader     *int                     `clausewitz:"leader"`
	Date       *string                  `clausewitz:"date"`
	LastRoll   *float64                 `clausewitz:"last_roll"`
	DaysLeft   *float64                 `clausewitz:"days_left"`
	Difficulty *float64                 `clausewitz:"difficulty"`
	Clues      *float64                 `clausewitz:"clues"`
	Stage      *string                  `clausewitz:"stage"`
	Status     *string                  `clausewitz:"status"`
	Event      *ArchaeologyEvent        `clausewitz:"event"`
	Events     []ArchaeologyEvent       `clausewitz:"events"`
	Completed  []FirstContactCompletion `clausewitz:"completed"`
	Flags      EspionageOperationFlags  `clausewitz:"flags"`
}

type FirstContactCompletion struct {
	Date  *string `clausewitz:"date"`
	Stage *string `clausewitz:"stage"`
}

func (c *FirstContacts) UnmarshalClausewitz(value clausewitz.Value) error {
	*c = FirstContacts{Items: map[int]FirstContact{}, DecodeErrors: map[int]string{}}
	if !value.IsObject() {
		c.TableIssue = true
		return nil
	}
	found := false
	for _, wrapper := range value.Object.Pairs {
		if wrapper.Key != "contacts" {
			continue
		}
		if found {
			c.TableIssue = true
		}
		found = true
		if !wrapper.Value.IsObject() {
			c.TableIssue = true
			continue
		}
		for _, pair := range wrapper.Value.Object.Pairs {
			id, err := strconv.Atoi(pair.Key)
			if err != nil {
				c.TableIssue = true
				continue
			}
			if pair.Value.IsScalar() && pair.Value.Scalar == "none" {
				continue
			}
			var contact FirstContact
			err = clausewitz.DecodeValue(pair.Value, &contact)
			if _, duplicate := c.Items[id]; duplicate {
				err = fmt.Errorf("duplicate first contact ID")
			}
			c.Items[id] = contact
			if err != nil {
				c.DecodeErrors[id] = err.Error()
			}
		}
	}
	if !found && len(value.Object.Pairs) > 0 {
		c.TableIssue = true
	}
	return nil
}
