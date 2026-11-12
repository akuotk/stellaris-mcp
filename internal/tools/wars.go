package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const warBattlePageSize = 20

func registerWars(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_wars", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read save-wide war records. war_id shows saved participants, goals, exhaustion, force-peace fields and a page of battle records. Battle country and location IDs are matched as candidates. Battle type codes, exhaustion and loss units, and current activity remain unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("war_id", mcp.Description("Saved war ID for details; omit for summary")),
		mcp.WithNumber("battle_offset", mcp.Description("First saved battle position (zero-based, 20 records per page; requires war_id)")),
		mcp.WithString("page_context", mcp.Description("Context token returned by the first page. Required for battle_offset > 0."))), handleWars)
}

func handleWars(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := requireStringArgs(req, "save_id", "page_context"); err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["war_id"]; ok {
		id, err := rawInteger(req, "war_id", 0, -maxRecordID, maxRecordID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &id
	}
	if _, hasOffset := req.GetArguments()["battle_offset"]; hasOffset && selected == nil {
		return toolError(fmt.Errorf("battle_offset requires war_id")), nil
	}
	if req.GetString("page_context", "") != "" && selected == nil {
		return toolError(fmt.Errorf("page_context requires war_id")), nil
	}
	offset, err := rawInteger(req, "battle_offset", 0, 0, 1<<31-1)
	if err != nil {
		return toolError(err), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	token := ""
	if selected != nil {
		token = rawPageToken(archive, -1, "get_wars", strconv.Itoa(*selected))
		verified, err := validateRawPageContext(req, offset, token)
		if err != nil {
			return toolError(err), nil
		}
		if !verified {
			return toolError(fmt.Errorf("page_context required for later battle pages")), nil
		}
	}
	gs, err := gamestate.LoadFromArchive(archive.zip)
	if err != nil {
		return toolError(err), nil
	}
	return renderWars(gs, selected, offset, archive.saveID, token), nil
}

func renderWars(gs *gamestate.GameState, selected *int, offset int, saveID, pageContext string) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintln(&b, "=== Saved war records (save-wide) ===")
	if saveID != "" {
		fmt.Fprintf(&b, "save_id=%s\n", saveID)
	}
	fmt.Fprintln(&b, "War activity, battle type codes, exhaustion and loss units remain unverified. Country and location links describe saved ID candidates.")
	writeSavedWarSummary(&b, gs, selected)
	if selected == nil {
		return mcp.NewToolResultText(b.String())
	}
	war, found := gs.War.Items[*selected]
	if !found {
		return mcp.NewToolResultText(b.String())
	}
	fields := &clausewitz.Object{}
	for _, pair := range war.Value.Object.Pairs {
		if pair.Key == "battles" || pair.Key == "attackers" && !war.Attackers.Unsupported || pair.Key == "defenders" && !war.Defenders.Unsupported {
			continue
		}
		fields.Pairs = append(fields.Pairs, pair)
	}
	writeSavedValue(&b, "\nWar fields (participant and battle entries displayed separately)", clausewitz.Value{Object: fields}, 0)
	for _, side := range []struct {
		key  string
		list gamestate.SavedWarList
	}{{"attackers", war.Attackers}, {"defenders", war.Defenders}} {
		for position, party := range side.list.Items {
			writeSavedValue(&b, fmt.Sprintf("%s[%d] (saved order)", side.key, position), party, 0)
		}
	}
	switch {
	case war.Battles.Unsupported:
		fmt.Fprintln(&b, "\nWar battles unsupported or duplicate. Use raw tree/byte tools.")
	case !war.Battles.Recorded:
		fmt.Fprintln(&b, "\nWar battles not recorded")
	default:
		total := len(war.Battles.Items)
		if offset > total {
			return toolError(fmt.Errorf("battle_offset %d exceeds %d saved battles", offset, total))
		}
		end := offset + min(warBattlePageSize, total-offset)
		fmt.Fprintf(&b, "\nSaved battle page offset=%d returned=%d total=%d", offset, end-offset, total)
		if end < total {
			fmt.Fprintf(&b, " next_battle_offset=%d", end)
		}
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "page_context=%s\n", pageContext)
		for position := offset; position < end; position++ {
			writeSavedWarBattle(&b, gs, position, war.Battles.Items[position])
		}
	}
	return mcp.NewToolResultText(b.String())
}

func writeSavedWarSummary(b *strings.Builder, gs *gamestate.GameState, selected *int) {
	table := gs.War
	switch {
	case table.TableIssue:
		fmt.Fprintln(b, "  War table decoding incomplete or duplicate. Use raw tree/byte tools.")
	case !table.Recorded:
		fmt.Fprintln(b, "  not recorded")
	case len(table.Items) == 0 && len(table.DecodeErrors) == 0:
		fmt.Fprintln(b, "  empty")
	}
	var ids []int
	if selected != nil {
		ids = []int{*selected}
	} else {
		for id := range table.Items {
			ids = append(ids, id)
		}
		for id := range table.DecodeErrors {
			ids = append(ids, id)
		}
		sort.Ints(ids)
	}
	for _, id := range ids {
		if _, bad := table.DecodeErrors[id]; bad {
			fmt.Fprintf(b, "  - ID %d: war record unsupported or duplicate. Use raw tree/byte tools.\n", id)
			continue
		}
		war, ok := table.Items[id]
		if !ok {
			fmt.Fprintf(b, "  - ID %d: war record unavailable\n", id)
			continue
		}
		label := "(name unsupported or duplicate)"
		if !war.NameUnsupported {
			label = name(war.Name)
		}
		fmt.Fprintf(b, "  - ID %d: %s (saved start=%s, saved end=%s)\n", id, label, councilSavedString(war.StartDate), councilSavedString(war.EndDate))
		for _, side := range []struct {
			label string
			list  gamestate.SavedWarList
		}{{"Attackers", war.Attackers}, {"Defenders", war.Defenders}} {
			if side.list.Unsupported || len(side.list.Items) == 0 {
				fmt.Fprintf(b, "    %s: %s\n", side.label, savedWarListStatus(side.list))
				continue
			}
			var names []string
			seen := map[int]bool{}
			for _, party := range side.list.Items {
				ref := gamestate.SavedWarIntRef(party, "country")
				switch {
				case !party.IsObject() || ref.Unsupported:
					names = append(names, "country reference unsupported or duplicate")
				case ref.Value == nil:
					names = append(names, "country reference not recorded")
				default:
					names = append(names, warCountryReference(gs, *ref.Value, seen))
				}
			}
			fmt.Fprintf(b, "    %s: %s\n", side.label, strings.Join(names, ", "))
		}
		fmt.Fprintf(b, "    Battles: %s\n", savedWarListStatus(war.Battles))
	}
}

func savedWarListStatus(list gamestate.SavedWarList) string {
	switch {
	case list.Unsupported:
		return "unsupported or duplicate"
	case !list.Recorded:
		return "not recorded"
	case len(list.Items) == 0:
		return "empty"
	default:
		return fmt.Sprintf("%d saved entries", len(list.Items))
	}
}

func warCountryReference(gs *gamestate.GameState, id int, seen map[int]bool) string {
	label := fmt.Sprintf("Country %d (record unavailable)", id)
	if _, ok := gs.Country[id]; ok {
		label = fmt.Sprintf("%s (ID %d)", countryName(gs, id), id)
	}
	if seen[id] {
		label += " [duplicate saved reference]"
	}
	seen[id] = true
	return label
}

func writeSavedWarBattle(b *strings.Builder, gs *gamestate.GameState, position int, battle clausewitz.Value) {
	writeSavedValue(b, fmt.Sprintf("\nBattle[%d] (saved order)", position), battle, 0)
	if !battle.IsObject() {
		fmt.Fprintln(b, "  Battle record shape unsupported. Use raw tree/byte tools.")
		return
	}
	for _, side := range []string{"attackers", "defenders"} {
		refs := gamestate.SavedWarBattleCountries(battle, side)
		switch {
		case refs.Unsupported:
			fmt.Fprintf(b, "  %s country ID candidates unresolved (unsupported or duplicate)\n", side)
		case !refs.Recorded:
			fmt.Fprintf(b, "  %s not recorded\n", side)
		case len(refs.Values) == 0:
			fmt.Fprintf(b, "  %s empty\n", side)
		default:
			seen := map[int]bool{}
			var names []string
			for _, id := range refs.Values {
				names = append(names, warCountryReference(gs, id, seen))
			}
			fmt.Fprintf(b, "  %s country ID candidates: %s\n", side, strings.Join(names, ", "))
		}
	}
	for _, key := range []string{"system", "planet", "colony"} {
		ref := gamestate.SavedWarIntRef(battle, key)
		if ref.Unsupported {
			fmt.Fprintf(b, "  %s ID candidate unresolved (unsupported or duplicate)\n", key)
			continue
		}
		if ref.Value == nil {
			continue
		}
		id := *ref.Value
		label := "record unavailable"
		switch key {
		case "system":
			if system, ok := gs.Systems[id]; ok {
				label = name(system.Name)
			}
		case "planet":
			if planet, ok := gs.Planets.Planet[id]; ok {
				label = name(planet.Name)
			}
		case "colony":
			if _, ok := gs.Colonies[id]; ok {
				label = "saved colony record exists"
			}
		}
		fmt.Fprintf(b, "  %s ID candidate %d (%s)\n", key, id, label)
	}
}
