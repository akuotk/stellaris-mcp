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

func registerEspionageOperations(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_espionage_operations", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithDescription("Read espionage operation records linked to matching spy networks or a saved country target. Validates active_operations and operation.spy_network links; displays raw target scope, index, days_left, rolls, difficulty and info. operation_id selects saved logs; opener_id and progress meaning are not inferred."), mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")), mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player")), mcp.WithNumber("operation_id", mcp.Description("Matching operation ID for full details; omit for summary"))), handleEspionageOperations)
}

func handleEspionageOperations(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	countryID, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	var selected *int
	if _, ok := req.GetArguments()["operation_id"]; ok {
		maxID := int(^uint(0) >> 1)
		if strconv.IntSize == 64 {
			maxExact := int64(1<<53 - 1)
			maxID = int(maxExact)
		}
		id, err := rawInteger(req, "operation_id", 0, 0, maxID)
		if err != nil {
			return toolError(err), nil
		}
		selected = &id
	}
	return renderEspionageOperations(gs, countryID, country, selected), nil
}

func renderEspionageOperations(gs *gamestate.GameState, countryID int, country *gamestate.Country, selected *int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Espionage operations (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Scope matches saved network owner/target or target.type=country with matching target.id. Index, days_left, rolls, difficulty and info are raw; phase/ETA/infiltration and opener_id semantics are not inferred.")
	if gs.EspionageOperations.TableIssue || len(gs.EspionageOperations.DecodeErrors) > 0 {
		fmt.Fprintln(&b, "Operation decoding incomplete for some saved records; use raw tree/byte tools for complete data.")
	}
	networks := map[int]bool{}
	refs := map[int][]int{}
	scoped := map[int]bool{}
	for id, network := range gs.SpyNetworks {
		if (network.Owner != nil && *network.Owner == countryID) || (network.Target != nil && *network.Target == countryID) {
			networks[id] = true
			for _, operationID := range network.ActiveOperations {
				refs[operationID] = append(refs[operationID], id)
				scoped[operationID] = true
			}
		}
	}
	for id, operation := range gs.EspionageOperations.Items {
		if operation.SpyNetwork != nil && networks[*operation.SpyNetwork] || operation.Target.Type == "country" && operation.Target.ID != nil && *operation.Target.ID == countryID {
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
			return toolError(fmt.Errorf("operation %d does not match selected country/network references", *selected))
		}
		ids = []int{*selected}
	}
	if len(ids) == 0 {
		fmt.Fprintln(&b, "No matching decoded operation records.")
	}
	for _, id := range ids {
		operation, ok := gs.EspionageOperations.Items[id]
		if !ok {
			fmt.Fprintf(&b, "\nOperation %d: referenced record unavailable\n", id)
			continue
		}
		fmt.Fprintf(&b, "\nOperation %d: type=%s spy_network=%s index=%s days_left=%s\n", id, operation.Type, armyInt(operation.SpyNetwork), armyInt(operation.Index), armyFloat(operation.DaysLeft))
		if _, bad := gs.EspionageOperations.DecodeErrors[id]; bad {
			fmt.Fprintln(&b, "  Record decoding incomplete; displayed fields may be partial")
		}
		referencing := append([]int(nil), refs[id]...)
		sort.Ints(referencing)
		for _, networkID := range referencing {
			if operation.SpyNetwork == nil {
				fmt.Fprintf(&b, "  Network %d lists this operation, but operation.spy_network is not recorded\n", networkID)
			} else if *operation.SpyNetwork != networkID {
				fmt.Fprintf(&b, "  Network %d active_operations disagrees with operation.spy_network=%d\n", networkID, *operation.SpyNetwork)
			}
		}
		if len(referencing) > 1 {
			fmt.Fprintf(&b, "  Multiple active_operations references in selected networks: %v\n", referencing)
		}
		if operation.SpyNetwork == nil {
			fmt.Fprintln(&b, "  Network reference: not recorded")
		} else if network, exists := gs.SpyNetworks[*operation.SpyNetwork]; exists {
			fmt.Fprintf(&b, "  Network %d: owner=%s target=%s; active_operations membership: %s\n", *operation.SpyNetwork, armyInt(network.Owner), armyInt(network.Target), armyMembership(network.ActiveOperations, id))
			if operation.Target.Type == "country" && operation.Target.ID != nil && network.Target != nil {
				if *operation.Target.ID == *network.Target {
					fmt.Fprintln(&b, "  operation target.id and network.target agree for saved country scope")
				} else {
					fmt.Fprintln(&b, "  Target mismatch: operation country target.id and network.target disagree")
				}
			}
		} else {
			fmt.Fprintln(&b, "  Referenced network record unavailable")
		}
		fmt.Fprintf(&b, "  Target (as saved): type=%s id=%s opener_id=%s (opener meaning unverified)\n", operation.Target.Type, armyInt(operation.Target.ID), armyInt(operation.Target.OpenerID))
		if selected == nil {
			continue
		}
		fmt.Fprintf(&b, "  last_roll=%s difficulty=%s info=%s\n", armyFloat(operation.LastRoll), armyFloat(operation.Difficulty), armyFloat(operation.Info))
		if operation.Log == nil {
			fmt.Fprintln(&b, "  Log: not recorded")
		} else if len(operation.Log) == 0 {
			fmt.Fprintln(&b, "  Log: empty")
		}
		for index, entry := range operation.Log {
			fmt.Fprintf(&b, "  Log entry %d (saved order): date=%s title=%s tooltip=%s roll=%s skill=%s info=%s difficulty=%s\n", index, entry.Date, entry.Title, entry.Tooltip, armyFloat(entry.Roll), armyFloat(entry.Skill), armyFloat(entry.Info), armyFloat(entry.Difficulty))
		}
		for _, key := range federationSortedKeys(operation.Flags) {
			flag := operation.Flags[key]
			if flag.Scalar != nil {
				fmt.Fprintf(&b, "  Flag %s=%s\n", key, *flag.Scalar)
			} else {
				fmt.Fprintf(&b, "  Flag %s: flag_date=%s flag_days=%s (as saved)\n", key, operationFlagText(flag.FlagDate), operationFlagText(flag.FlagDays))
			}
		}
	}
	return mcp.NewToolResultText(b.String())
}

func operationFlagText(value *string) string {
	if value == nil {
		return "not recorded"
	}
	return *value
}
