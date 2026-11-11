package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSectors(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_sectors", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read sector records matching saved sector.owner or country.sectors.owned references. Shows missing/conflicting ownership paths. sector_id adds saved system order, reciprocal galactic_object.sector checks and local_capital ID candidates. Capital namespace, type and auto_expand meanings remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")),
		mcp.WithNumber("sector_id", mcp.Description("Matching sector reference for details; omit for summary"))), handleSectors)
}

func handleSectors(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["sector_id"]; ok {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			maxExact := int64(1<<53 - 1)
			maxID = int(maxExact)
		}
		sectorID, err := rawInteger(req, "sector_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &sectorID
	}
	return renderSectors(gs, id, country, selected), nil
}

func renderSectors(gs *gamestate.GameState, countryID int, country *gamestate.Country, selected *int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved sectors (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope uses saved sector.owner or country.sectors.owned. Type, auto_expand and local_capital namespace remain unverified. Use sector_id for system/reference details.")
	if !gs.Sectors.Recorded {
		fmt.Fprintln(&b, "Sector table not recorded")
	}
	if gs.Sectors.TableIssue || len(gs.Sectors.DecodeErrors) > 0 {
		fmt.Fprintln(&b, "Sector decoding incomplete; unsupported or duplicate records are excluded from owner paths. Use raw tree/byte tools.")
	}
	// Survey global saved references separately from the country-scoped results.
	maxUint32Refs, otherUnmatched, unsupportedRefs := 0, 0, 0
	for _, system := range gs.Systems {
		if system.SectorRef.Unsupported {
			unsupportedRefs++
			continue
		}
		if system.SectorRef.Value == nil {
			continue
		}
		id := *system.SectorRef.Value
		if id >= 0 && uint64(id) == uint64(4294967295) {
			maxUint32Refs++
			continue
		}
		if _, ok := gs.Sectors.Items[id]; !ok {
			otherUnmatched++
		}
	}
	fmt.Fprintf(&b, "System sector reference survey (all saved systems): raw 4294967295=%d (meaning unverified); other IDs without a decoded sector record=%d", maxUint32Refs, otherUnmatched)
	if unsupportedRefs > 0 {
		fmt.Fprintf(&b, "; undecodable references=%d (not included in the counts above)", unsupportedRefs)
	}
	fmt.Fprintln(&b)
	if !gs.Sectors.Recorded || gs.Sectors.TableIssue || len(gs.Sectors.DecodeErrors) > 0 || unsupportedRefs > 0 {
		fmt.Fprintln(&b, "  Survey coverage incomplete; use raw tools for unknown tables or references.")
	}
	refs := country.SectorRefs
	switch {
	case refs.Unsupported || refs.Owned.Unsupported:
		fmt.Fprintln(&b, "country.sectors.owned unsupported or duplicate; country reference coverage incomplete, use raw tools")
	case !refs.Recorded:
		fmt.Fprintln(&b, "country.sectors not recorded")
	case !refs.Owned.Recorded:
		fmt.Fprintln(&b, "country.sectors.owned not recorded")
	case len(refs.Owned.Values) == 0:
		fmt.Fprintln(&b, "country.sectors.owned empty")
	}
	scoped := map[int]bool{}
	if !refs.Unsupported && !refs.Owned.Unsupported {
		for _, id := range refs.Owned.Values {
			scoped[id] = true
		}
	}
	for id, sector := range gs.Sectors.Items {
		if sector.Owner != nil && *sector.Owner == countryID {
			scoped[id] = true
		}
	}
	ids := make([]int, 0, len(scoped))
	for id := range scoped {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if selected != nil {
		if !scoped[*selected] {
			return toolError(fmt.Errorf("sector %d has no matching country reference path", *selected))
		}
		ids = []int{*selected}
	}
	if len(ids) == 0 {
		fmt.Fprintln(&b, "No matching decoded sector records or country references")
	}
	for _, id := range ids {
		sector, ok := gs.Sectors.Items[id]
		if !ok {
			if _, bad := gs.Sectors.DecodeErrors[id]; bad {
				fmt.Fprintf(&b, "\nSector %d: referenced record unsupported or duplicate; use raw tools\n", id)
			} else {
				fmt.Fprintf(&b, "\nSector %d: referenced record unavailable\n", id)
			}
			continue
		}
		membership := sectorListMembership(refs.Owned, id)
		if refs.Unsupported {
			membership = "unsupported; use raw tools"
		}
		fmt.Fprintf(&b, "\nSector %d: %s; owner=%s; country.sectors.owned membership=%s\n", id, name(sector.Name), armyInt(sector.Owner), membership)
		if sector.Owner != nil && *sector.Owner != countryID {
			fmt.Fprintln(&b, "  Sector owner differs from selected country")
		}
		fmt.Fprintf(&b, "  type=%s auto_expand=%s local_capital=%s (saved values)\n", governanceValue(sector.Type), governanceValue(sector.AutoExpand), armyInt(sector.LocalCapital))
		switch {
		case sector.Systems.Unsupported:
			fmt.Fprintln(&b, "  sector.systems unsupported; use raw tools")
		case !sector.Systems.Recorded:
			fmt.Fprintln(&b, "  sector.systems not recorded")
		default:
			fmt.Fprintf(&b, "  sector.systems saved reference count=%d\n", len(sector.Systems.Values))
		}
		if selected == nil {
			continue
		}
		writeSectorSystems(&b, gs, id, sector)
		if sector.LocalCapital != nil {
			capital := *sector.LocalCapital
			fmt.Fprintln(&b, "  local_capital ID candidates (namespace unverified; ID equality does not establish capital identity)")
			colony, colonyOK := gs.Colonies[capital]
			if colonyOK {
				fmt.Fprintf(&b, "    Possible colony ID match: %d; %s\n", capital, colonyCarrier(gs, capital, colony))
			}
			planet, planetOK := gs.Planets.Planet[capital]
			if planetOK {
				fmt.Fprintf(&b, "    Possible planet ID match: %d %s; saved owner=%s\n", capital, name(planet.Name), armyInt(planet.OwnerID))
			}
			if !colonyOK && !planetOK {
				fmt.Fprintln(&b, "    No decoded colony/planet ID candidate; other namespace/sentinel meaning unverified")
			}
		}
	}
	return mcp.NewToolResultText(b.String())
}

func writeSectorSystems(b *strings.Builder, gs *gamestate.GameState, sectorID int, sector gamestate.Sector) {
	listed := map[int]bool{}
	for index, systemID := range sector.Systems.Values {
		listed[systemID] = true
		fmt.Fprintf(b, "  System position %d id=%d; sector.systems membership=%s\n", index, systemID, sectorListMembership(sector.Systems, systemID))
		system, ok := gs.Systems[systemID]
		if !ok {
			fmt.Fprintln(b, "    Referenced system record unavailable")
			continue
		}
		fmt.Fprintf(b, "    %s; system.sector=", name(system.Name))
		switch {
		case system.SectorRef.Unsupported:
			fmt.Fprintln(b, "unsupported; use raw tools")
		case system.SectorRef.Value == nil:
			fmt.Fprintln(b, "not recorded")
		case *system.SectorRef.Value != sectorID:
			fmt.Fprintf(b, "%d; sector/system reference mismatch\n", *system.SectorRef.Value)
		default:
			fmt.Fprintf(b, "%d; reciprocal sector/system IDs agree\n", *system.SectorRef.Value)
		}
	}
	reverse := []int{}
	unsupported := 0
	for id, system := range gs.Systems {
		if system.SectorRef.Unsupported {
			unsupported++
		}
		if system.SectorRef.Value != nil && !system.SectorRef.Unsupported && *system.SectorRef.Value == sectorID && !listed[id] {
			reverse = append(reverse, id)
		}
	}
	sort.Ints(reverse)
	for _, id := range reverse {
		fmt.Fprintf(b, "  System %d: system.sector points here; sector.systems membership=%s\n", id, sectorListMembership(sector.Systems, id))
	}
	if unsupported > 0 {
		fmt.Fprintf(b, "  Unsupported system.sector fields=%d; reverse-reference coverage incomplete\n", unsupported)
	}
}

func sectorListMembership(refs gamestate.OptionalIntList, id int) string {
	if refs.Unsupported {
		return "unsupported; use raw tools"
	}
	return armyMembership(refs.Values, id)
}
