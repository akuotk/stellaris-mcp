package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type SavedBudgetCategories struct {
	Recorded    bool
	Duplicate   bool
	Unsupported bool
	Items       []SavedBudgetCategory
}

type SavedBudgetCategory struct {
	Key       string
	Resources SavedResourceAmounts
}

func (c *SavedBudgetCategories) UnmarshalClausewitz(value clausewitz.Value) error {
	*c = SavedBudgetCategories{Recorded: true}
	if !value.IsObject() {
		c.Unsupported = true
		return nil
	}
	for _, pair := range value.Object.Pairs {
		entry := SavedBudgetCategory{Key: pair.Key}
		entry.Resources.UnmarshalClausewitz(pair.Value)
		c.Items = append(c.Items, entry)
	}
	return nil
}

func (c *SavedBudgetCategories) rejectDuplicate(value clausewitz.Value, key string) {
	if uniqueSavedFields(value, key) != nil {
		*c = SavedBudgetCategories{Recorded: true, Duplicate: true}
	}
}

type IncomeHighWaterMark struct {
	Length      OptionalString        `clausewitz:"length"`
	Current     SavedResourceAmounts  `clausewitz:"current"`
	History     SavedBudgetCategories `clausewitz:"history"`
	Recorded    bool                  `clausewitz:"-"`
	Duplicate   bool                  `clausewitz:"-"`
	Unsupported bool                  `clausewitz:"-"`
}

func (h *IncomeHighWaterMark) UnmarshalClausewitz(value clausewitz.Value) error {
	*h = IncomeHighWaterMark{Recorded: true}
	if uniqueSavedFields(value, "length", "current", "history") != nil {
		h.Duplicate = true
		return nil
	}
	type fields IncomeHighWaterMark
	if err := clausewitz.DecodeValue(value, (*fields)(h)); err != nil {
		h.Unsupported = true
	}
	return nil
}
