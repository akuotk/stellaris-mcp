package tools

import (
	"fmt"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"sort"
	"strings"
)

func populationSpecies(gs *gamestate.GameState, id int) string {
	if issue := gs.Species.DecodeErrors[id]; issue != "" {
		return fmt.Sprintf("%d (%s)", id, diplomaticRecordIssue(issue))
	}
	s, ok := gs.Species.Items[id]
	if !ok || s.Name.Key == "" {
		return fmt.Sprintf("%d (identity unavailable)", id)
	}
	return fmt.Sprintf("%d (%s)", id, name(s.Name))
}

func writePopDistributions(b *strings.Builder, gs *gamestate.GameState, group gamestate.PopGroup) {
	if len(group.Ethos.Ethics) != len(group.Ethos.Pops) {
		fmt.Fprintf(b, "    Ethics: unsupported mismatched arrays (keys=%q populations=%v)\n", group.Ethos.Ethics, group.Ethos.Pops)
	} else if len(group.Ethos.Ethics) > 0 {
		fmt.Fprint(b, "    Ethics (raw population amounts):")
		for i, key := range group.Ethos.Ethics {
			fmt.Fprintf(b, " %s=%d", key, group.Ethos.Pops[i])
		}
		fmt.Fprintln(b)
	} else {
		fmt.Fprintln(b, "    Ethics: no distribution recorded")
	}
	if len(group.Factions.Objects) != len(group.Factions.Pops) {
		fmt.Fprintf(b, "    Factions: unsupported mismatched arrays (ids=%v populations=%v)\n", group.Factions.Objects, group.Factions.Pops)
	} else if len(group.Factions.Objects) > 0 {
		fmt.Fprintln(b, "    Faction membership (raw population amounts; excludes unassigned population):")
		for i, id := range group.Factions.Objects {
			f, ok := gs.PopFactions[id]
			if ok {
				country := "unknown"
				if f.Country != nil {
					country = fmt.Sprint(*f.Country)
				}
				fmt.Fprintf(b, "      %d (%s, type=%s, country=%s): %d\n", id, name(f.Name), f.Type, country, group.Factions.Pops[i])
			} else {
				fmt.Fprintf(b, "      %d (identity unavailable): %d\n", id, group.Factions.Pops[i])
			}
		}
	} else {
		fmt.Fprintln(b, "    Factions: no membership recorded")
	}
}

func writeCountryFactions(b *strings.Builder, gs *gamestate.GameState, countryID int) {
	ids := []int{}
	for id, f := range gs.PopFactions {
		if f.Country != nil && *f.Country == countryID {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	if len(ids) == 0 {
		fmt.Fprintln(b, "\nPolitical factions: no records")
		return
	}
	fmt.Fprintln(b, "\nPolitical factions (support_percent and approval as saved; population in raw save units):")
	for _, id := range ids {
		f := gs.PopFactions[id]
		fmt.Fprintf(b, "  %d: %s, type=%s support_percent=%g approval=%g last_month_population=%d change=%+d\n", id, name(f.Name), f.Type, f.SupportPercent, f.Approval, f.LastMonthPops, f.PopsChange)
	}
}
