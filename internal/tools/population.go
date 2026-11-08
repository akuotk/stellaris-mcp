package tools

import (
	"fmt"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"sort"
	"strconv"
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
	if group.Factions.Duplicate {
		fmt.Fprintln(b, "    Factions: duplicate saved fields or group records")
	} else if group.Factions.Unsupported {
		fmt.Fprintln(b, "    Factions: unsupported saved shape")
	} else if len(group.Factions.Objects) != len(group.Factions.Pops) {
		fmt.Fprintf(b, "    Factions: unsupported mismatched arrays (ids=%v populations=%v)\n", group.Factions.Objects, group.Factions.Pops)
	} else if len(group.Factions.Objects) > 0 {
		fmt.Fprintln(b, "    Faction membership (raw population amounts; excludes unassigned population):")
		for i, id := range group.Factions.Objects {
			f, ok := gs.PopFactions.Items[id]
			if ok {
				country := "unknown"
				if f.Country != nil {
					country = fmt.Sprint(*f.Country)
				}
				fmt.Fprintf(b, "      %d (%s, type=%s, country=%s): %d\n", id, name(f.Name), f.Type, country, group.Factions.Pops[i])
			} else {
				if issue := gs.PopFactions.DecodeErrors[id]; issue != "" {
					fmt.Fprintf(b, "      %d (%s): %d\n", id, diplomaticRecordIssue(issue), group.Factions.Pops[i])
				} else {
					fmt.Fprintf(b, "      %d (identity unavailable): %d\n", id, group.Factions.Pops[i])
				}
			}
		}
	} else {
		fmt.Fprintln(b, "    Factions: no membership recorded")
	}
}

func writeCountryFactions(b *strings.Builder, gs *gamestate.GameState, countryID int) {
	ids := []int{}
	for id, f := range gs.PopFactions.Items {
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
		f := gs.PopFactions.Items[id]
		change := operationFlagText(f.PopsChange)
		if n, err := strconv.ParseInt(change, 10, 64); err == nil && n > 0 && !strings.HasPrefix(change, "+") {
			change = "+" + change
		}
		fmt.Fprintf(b, "  %d: %s, type=%s support_percent=%s approval=%s last_month_population=%s change=%s\n", id, name(f.Name), f.Type, operationFlagText(f.SupportPercent), operationFlagText(f.Approval), operationFlagText(f.LastMonthPops), change)
	}
}
