package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGovernance(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_policies_edicts", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read a country's saved active_policies, edicts and policy_flags in saved order. Keeps raw keys, selected options, dates and perpetual values. Date semantics, cooldowns, effects and current edict activity are not inferred."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("country_id", mcp.Description("Country ID; omit for sole player"))), handleGovernance)
}

func handleGovernance(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadSave(req)
	if err != nil {
		return toolError(err), nil
	}
	id, country, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}
	return renderGovernance(id, country), nil
}

func renderGovernance(countryID int, country *gamestate.Country) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved policies and edicts (%s, country %d) ===\n", name(country.Adjective), countryID)
	fmt.Fprintln(&b, "Date, start_date and perpetual are saved values. Their semantics, cooldowns, effects and current edict activity remain unverified.")
	policies := country.ActivePolicies
	fmt.Fprintln(&b, "\nactive_policies (saved order)")
	switch {
	case policies.Unsupported:
		fmt.Fprintln(&b, "  Unsupported saved shape; use raw tree/byte tools for complete policy data")
	case !policies.Recorded:
		fmt.Fprintln(&b, "  Not recorded")
	case len(policies.Items) == 0:
		fmt.Fprintln(&b, "  Empty")
	}
	for index, policy := range policies.Items {
		fmt.Fprintf(&b, "  Entry %d policy=%s selected=%s date=%s\n", index, governanceValue(policy.Policy), governanceValue(policy.Selected), governanceValue(policy.Date))
	}
	edicts := country.Edicts
	fmt.Fprintln(&b, "\nedicts (saved order)")
	switch {
	case edicts.Unsupported:
		fmt.Fprintln(&b, "  Unsupported saved shape; use raw tree/byte tools for complete edict data")
	case !edicts.Recorded:
		fmt.Fprintln(&b, "  Not recorded")
	case len(edicts.Items) == 0:
		fmt.Fprintln(&b, "  Empty")
	}
	for index, edict := range edicts.Items {
		fmt.Fprintf(&b, "  Entry %d edict=%s date=%s start_date=%s perpetual=%s\n", index, governanceValue(edict.Edict), governanceValue(edict.Date), governanceValue(edict.StartDate), governanceValue(edict.Perpetual))
	}
	flags := country.PolicyFlags
	fmt.Fprintln(&b, "\npolicy_flags (raw values in saved order; relationship to policy selections unverified)")
	switch {
	case flags.Unsupported:
		fmt.Fprintln(&b, "  Unsupported saved shape; use raw tree/byte tools for complete policy flags")
	case !flags.Recorded:
		fmt.Fprintln(&b, "  Not recorded")
	case len(flags.Values) == 0:
		fmt.Fprintln(&b, "  Empty")
	}
	for index, flag := range flags.Values {
		fmt.Fprintf(&b, "  Entry %d value=%q\n", index, flag)
	}
	return mcp.NewToolResultText(b.String())
}

func governanceValue(value *string) string {
	if value == nil {
		return "not recorded"
	}
	return strconv.Quote(*value)
}
