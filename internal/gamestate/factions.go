package gamestate

import (
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type CountryFactionModules struct {
	Module      *CountryFactionModule `clausewitz:"standard_pop_factions_module"`
	Recorded    bool                  `clausewitz:"-"`
	Duplicate   bool                  `clausewitz:"-"`
	Unsupported bool                  `clausewitz:"-"`
}

func (m *CountryFactionModules) UnmarshalClausewitz(value clausewitz.Value) error {
	*m = CountryFactionModules{Recorded: true}
	if uniqueSavedFields(value, "standard_pop_factions_module") != nil {
		m.Duplicate = true
		return nil
	}
	type fields CountryFactionModules
	if err := clausewitz.DecodeValue(value, (*fields)(m)); err != nil {
		*m = CountryFactionModules{Recorded: true, Unsupported: true}
	}
	return nil
}

type CountryFactionModule struct {
	FactionRefs SurfaceRefList    `clausewitz:"pop_factions"`
	SavedFields []clausewitz.Pair `clausewitz:"-"`
}

func (m *CountryFactionModule) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields CountryFactionModule
	if err := clausewitz.DecodeValue(value, (*fields)(m)); err != nil {
		return err
	}
	m.FactionRefs.rejectDuplicate(value, "pop_factions")
	for _, pair := range value.Object.Pairs {
		switch pair.Key {
		case "factions", "potential", "actual_count", "potential_count", "last_created",
			"total_faction_members", "total_faction_members_power", "total_potential_members":
			m.SavedFields = append(m.SavedFields, pair)
		}
	}
	return nil
}

type PopFaction struct {
	Country         *int                 `clausewitz:"country"`
	Type            string               `clausewitz:"type"`
	Name            LocalizedName        `clausewitz:"name"`
	SupportPercent  *string              `clausewitz:"support_percent"`
	Approval        *string              `clausewitz:"faction_approval"`
	LastMonthPops   *string              `clausewitz:"last_month_pops"`
	PopsChange      *string              `clausewitz:"pops_change"`
	SupportPower    *string              `clausewitz:"support_power"`
	UnformableSince *string              `clausewitz:"unformable_since"`
	Members         SurfaceRefList       `clausewitz:"members"`
	ColonyHistory   FactionColonyHistory `clausewitz:"last_month_colony_pops"`
	SavedDetails    []clausewitz.Pair    `clausewitz:"-"`
}

func (f *PopFaction) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "country", "type", "name", "support_percent", "faction_approval", "last_month_pops", "pops_change", "support_power", "unformable_since"); err != nil {
		return err
	}
	type fields PopFaction
	if err := clausewitz.DecodeValue(value, (*fields)(f)); err != nil {
		return err
	}
	f.Members.rejectDuplicate(value, "members")
	if uniqueSavedFields(value, "last_month_colony_pops") != nil {
		f.ColonyHistory = FactionColonyHistory{Recorded: true, Duplicate: true}
	}
	for _, pair := range value.Object.Pairs {
		if pair.Key == "parameters" || pair.Key == "timed_modifier" {
			f.SavedDetails = append(f.SavedDetails, pair)
		}
	}
	return nil
}

type FactionColonyHistory struct {
	Items       []FactionColonyHistoryRow
	Recorded    bool
	Duplicate   bool
	Unsupported bool
}

func (h *FactionColonyHistory) UnmarshalClausewitz(value clausewitz.Value) error {
	*h = FactionColonyHistory{Recorded: true}
	if !value.IsList() && (!value.IsObject() || len(value.Object.Pairs) != 0) {
		h.Unsupported = true
		return nil
	}
	return clausewitz.DecodeValue(value, &h.Items)
}

type FactionColonyHistoryRow struct {
	Colony      OptionalIntRef `clausewitz:"colony"`
	NumPops     *string        `clausewitz:"num_pops"`
	Change      *string        `clausewitz:"change"`
	Duplicate   bool           `clausewitz:"-"`
	Unsupported bool           `clausewitz:"-"`
}

func (r *FactionColonyHistoryRow) UnmarshalClausewitz(value clausewitz.Value) error {
	if uniqueSavedFields(value, "colony", "num_pops", "change") != nil {
		*r = FactionColonyHistoryRow{Duplicate: true}
		return nil
	}
	type fields FactionColonyHistoryRow
	if err := clausewitz.DecodeValue(value, (*fields)(r)); err != nil {
		*r = FactionColonyHistoryRow{Unsupported: true}
	}
	return nil
}

type LegacyFactionPop struct {
	FactionRef OptionalIntRef `clausewitz:"pop_faction"`
	Duplicate  bool           `clausewitz:"-"`
}

func (p *LegacyFactionPop) UnmarshalClausewitz(value clausewitz.Value) error {
	if uniqueSavedFields(value, "pop_faction") != nil {
		*p = LegacyFactionPop{Duplicate: true}
		return nil
	}
	type fields LegacyFactionPop
	return clausewitz.DecodeValue(value, (*fields)(p))
}

func (gs *GameState) markAmbiguousPopGroupFactions(value clausewitz.Value) {
	mark := func(id int) {
		if group, ok := gs.PopGroups[id]; ok {
			group.Factions = PopFactionDistribution{Duplicate: true}
			gs.PopGroups[id] = group
		}
	}
	if uniqueSavedFields(value, "pop_groups") != nil {
		for id := range gs.PopGroups {
			mark(id)
		}
		return
	}
	for _, pair := range value.Object.Pairs {
		if pair.Key != "pop_groups" || !pair.Value.IsObject() {
			continue
		}
		seen := map[int]bool{}
		for _, row := range pair.Value.Object.Pairs {
			id, err := strconv.Atoi(row.Key)
			if err != nil {
				continue
			}
			if seen[id] {
				mark(id)
			}
			seen[id] = true
		}
	}
}
