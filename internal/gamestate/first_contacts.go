package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type FirstContacts struct {
	ConnectionRecordTable[FirstContact]
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
	*c = FirstContacts{}
	if !value.IsObject() || uniqueSavedFields(value, "contacts") != nil {
		c.TableIssue = true
		return nil
	}
	for _, pair := range value.Object.Pairs {
		if pair.Key == "contacts" {
			return c.ConnectionRecordTable.UnmarshalClausewitz(pair.Value)
		}
	}
	c.TableIssue = len(value.Object.Pairs) != 0
	return nil
}

func (c *FirstContact) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "owner", "country", "name", "location", "leader", "date", "last_roll", "days_left", "difficulty", "clues", "stage", "status", "event", "flags"); err != nil {
		return err
	}
	type fields FirstContact
	return clausewitz.DecodeValue(value, (*fields)(c))
}
