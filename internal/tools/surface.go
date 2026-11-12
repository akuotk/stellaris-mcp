package tools

import (
	"fmt"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func surfaceRefText(refs gamestate.SurfaceRefList) string {
	switch {
	case refs.Duplicate:
		return "duplicate saved fields"
	case refs.Unsupported:
		return "unsupported saved shape"
	default:
		return colonyReferenceList(refs.Values)
	}
}

func writeSurfaceLayout(b *strings.Builder, gs *gamestate.GameState, districts, cache gamestate.SurfaceRefList) {
	if gs.Districts.TableIssue || gs.Zones.TableIssue || gs.Buildings.TableIssue {
		fmt.Fprintln(b, "  Surface table decoding incomplete. Use raw tools.")
	}
	fmt.Fprintf(b, "  District references: %s\n  Building cache references: %s\n", surfaceRefText(districts), surfaceRefText(cache))
	placed := map[int]bool{}
	seenDistricts := map[int]bool{}
	for _, id := range districts.Values {
		if seenDistricts[id] {
			fmt.Fprintf(b, "    Duplicate district reference %d\n", id)
			continue
		}
		seenDistricts[id] = true
		d, ok := gs.Districts.Items[id]
		if !ok {
			fmt.Fprintf(b, "    District %d: %s\n", id, diplomaticRecordIssue(gs.Districts.DecodeErrors[id]))
			continue
		}
		fmt.Fprintf(b, "    District %d: type=%s level=%s zones=%s [%s]\n", id, d.Type, governanceValue(d.LevelRaw), surfaceRefText(d.ZoneRefs), l(d.Type))
		seenZones := map[int]bool{}
		for _, zid := range d.ZoneRefs.Values {
			if seenZones[zid] {
				fmt.Fprintf(b, "      Duplicate zone reference %d\n", zid)
				continue
			}
			seenZones[zid] = true
			z, ok := gs.Zones.Items[zid]
			if !ok {
				fmt.Fprintf(b, "      Zone %d: %s\n", zid, diplomaticRecordIssue(gs.Zones.DecodeErrors[zid]))
				continue
			}
			fmt.Fprintf(b, "      Zone %d: type=%s buildings=%s [%s]\n", zid, z.Type, surfaceRefText(z.BuildingRefs), l(z.Type))
			for _, bid := range z.BuildingRefs.Values {
				placed[bid] = true
			}
			writeSurfaceBuildings(b, gs, "        ", z.BuildingRefs.Values, &cache)
		}
	}
	var cacheOnly []int
	for _, id := range cache.Values {
		if !placed[id] {
			cacheOnly = append(cacheOnly, id)
		}
	}
	if len(cacheOnly) != 0 {
		fmt.Fprintln(b, "  Building cache references without a decoded zone path:")
		writeSurfaceBuildings(b, gs, "    ", cacheOnly, nil)
	}
}

func writeSurfaceBuildings(b *strings.Builder, gs *gamestate.GameState, indent string, ids []int, cache *gamestate.SurfaceRefList) {
	seen := map[int]bool{}
	for _, id := range ids {
		if seen[id] {
			fmt.Fprintf(b, "%sDuplicate building reference %d\n", indent, id)
			continue
		}
		seen[id] = true
		if building, ok := gs.Buildings.Items[id]; ok {
			fmt.Fprintf(b, "%sBuilding %d: type=%s position=%s ruined=%s (saved values) [%s]", indent, id, building.Type, councilSavedString(building.Position), councilSavedString(building.Ruined), l(building.Type))
		} else {
			fmt.Fprintf(b, "%sBuilding %d: %s", indent, id, diplomaticRecordIssue(gs.Buildings.DecodeErrors[id]))
		}
		if cache != nil {
			membership := armyMembership(cache.Values, id)
			if cache.Duplicate || cache.Unsupported {
				membership = surfaceRefText(*cache)
			}
			fmt.Fprintf(b, ", buildings_cache membership=%s", membership)
		}
		fmt.Fprintln(b)
	}
}

func writeExternalSurfaceBuildings(b *strings.Builder, gs *gamestate.GameState, label string, external gamestate.ExternalSurfaceBuildings) {
	switch {
	case external.Duplicate:
		fmt.Fprintf(b, "  %s: duplicate saved fields\n", label)
	case external.Unsupported:
		fmt.Fprintf(b, "  %s: unsupported saved shape\n", label)
	case external.Recorded:
		fmt.Fprintf(b, "  %s: %d saved groups\n", label, len(external.Items))
	}
	for index, group := range external.Items {
		switch {
		case group.Duplicate:
			fmt.Fprintf(b, "    Group %d: duplicate saved owner fields\n", index)
		case group.Unsupported:
			fmt.Fprintf(b, "    Group %d: unsupported saved shape\n", index)
		default:
			fmt.Fprintf(b, "    Group %d: building_owner=%s owner_type=%s (saved values), buildings=%s\n", index, councilSavedString(group.Owner), councilSavedString(group.OwnerType), surfaceRefText(group.BuildingRefs))
			writeSurfaceBuildings(b, gs, "      ", group.BuildingRefs.Values, nil)
		}
	}
}

func writePlanetDeposits(b *strings.Builder, table gamestate.ConnectionRecordTable[gamestate.Deposit], planetID int, refs gamestate.SurfaceRefList) {
	fmt.Fprintf(b, "\nDeposit references: %s\n", surfaceRefText(refs))
	if len(refs.Values) > 0 {
		fmt.Fprintln(b, "  Holder types retain their saved values. ID comparisons use the referencing planet ID.")
	}
	if table.TableIssue {
		fmt.Fprintln(b, "  Deposit table decoding incomplete. Use raw tools.")
	}
	seen := map[int]bool{}
	for _, id := range refs.Values {
		if seen[id] {
			fmt.Fprintf(b, "  Duplicate deposit reference %d\n", id)
			continue
		}
		seen[id] = true
		deposit, ok := table.Items[id]
		if !ok {
			fmt.Fprintf(b, "  Deposit %d: %s\n", id, diplomaticRecordIssue(table.DecodeErrors[id]))
			continue
		}
		swap := "not recorded"
		if deposit.SwapType != "" {
			swap = deposit.SwapType
		}
		fmt.Fprintf(b, "  Deposit %d: %s [%s], swap_type=%s\n", id, l(deposit.Type), deposit.Type, swap)
		h := deposit.Holder
		switch {
		case h.Duplicate:
			fmt.Fprintln(b, "    deposit_holder: duplicate saved fields")
		case h.Unsupported || h.ID.Unsupported:
			fmt.Fprintln(b, "    deposit_holder: unsupported saved shape")
		case !h.Recorded:
			fmt.Fprintln(b, "    deposit_holder: not recorded")
		default:
			fmt.Fprintf(b, "    deposit_holder.type=%s id=%s (saved values)", governanceValue(h.Type), savedInt(h.ID.Value))
			if h.ID.Value != nil {
				comparison := "differs from"
				if *h.ID.Value == planetID {
					comparison = "matches"
				}
				fmt.Fprintf(b, ", holder ID %s referencing planet ID", comparison)
			}
			fmt.Fprintln(b)
		}
	}
}
