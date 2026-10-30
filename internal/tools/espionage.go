package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerEspionage(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("get_espionage",
			mcp.WithDescription("Get spy networks and raw network power, spymasters and active operation counts; infiltration conversion is not verified"),
			mcp.WithNumber("country_id", mcp.Description("Country ID (default 0 = player)"), mcp.DefaultNumber(0)),
		),
		handleEspionage,
	)
}

func handleEspionage(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gs, err := loadLatestSave()
	if err != nil {
		return toolError(err), nil
	}
	countryID, c, err := getCountry(gs, req)
	if err != nil {
		return toolError(err), nil
	}

	type networkEntry struct {
		id  int
		net gamestate.SpyNetwork
	}

	// Our networks (where we spy on others)
	var ourNets []networkEntry
	// Networks targeting us
	var theirNets []networkEntry

	for id, net := range gs.SpyNetworks {
		if net.Owner == countryID {
			ourNets = append(ourNets, networkEntry{id, net})
		}
		if net.Target == countryID {
			theirNets = append(theirNets, networkEntry{id, net})
		}
	}

	sort.Slice(ourNets, func(i, j int) bool { return ourNets[i].net.Power > ourNets[j].net.Power })
	sort.Slice(theirNets, func(i, j int) bool { return theirNets[i].net.Power > theirNets[j].net.Power })

	var b strings.Builder
	fmt.Fprintf(&b, "=== Espionage (%s) ===\n", name(c.Adjective))

	// Our spy networks
	fmt.Fprintf(&b, "\nOur Spy Networks (%d):\n", len(ourNets))
	activeCount := 0
	for _, e := range ourNets {
		if e.net.Power > 0 || e.net.Leader != 0 {
			activeCount++
		}
	}
	if activeCount == 0 {
		fmt.Fprintf(&b, "  No networks with nonzero power or leader recorded.\n")
	}
	for _, e := range ourNets {
		target := countryName(gs, e.net.Target)
		if e.net.Power == 0 && e.net.Leader == 0 {
			continue // Skip inactive networks
		}
		fmt.Fprintf(&b, "\n  Target: %s (Country %d)\n", target, e.net.Target)
		fmt.Fprintf(&b, "    Network power (raw save value; infiltration conversion unknown): %.1f\n", e.net.Power)

		// Resolve spymaster
		if e.net.Leader != 0 {
			if ldr, ok := gs.Leaders[e.net.Leader]; ok {
				fmt.Fprintf(&b, "    Spymaster: %s (Level %d)\n", name(ldr.Name.FullNames), ldr.Level)
			}
		}

		if len(e.net.ActiveOperations) > 0 {
			fmt.Fprintf(&b, "    Active Operations: %d\n", len(e.net.ActiveOperations))
		}
		if e.net.Formed != "" {
			fmt.Fprintf(&b, "    Established: %s\n", e.net.Formed)
		}

	}

	// Show inactive networks as a compact list
	inactiveCount := len(ourNets) - activeCount
	if inactiveCount > 0 {
		fmt.Fprintf(&b, "\n  Networks without nonzero power or leader (%d): ", inactiveCount)
		var names []string
		for _, e := range ourNets {
			if e.net.Power == 0 && e.net.Leader == 0 {
				names = append(names, countryName(gs, e.net.Target))
			}
		}
		fmt.Fprintf(&b, "%s\n", strings.Join(names, ", "))
	}

	// Networks targeting us
	fmt.Fprintf(&b, "\nHostile Spy Networks Targeting Us (%d):\n", len(theirNets))
	hostileActive := 0
	for _, e := range theirNets {
		if e.net.Power == 0 && e.net.Leader == 0 {
			continue
		}
		hostileActive++
		owner := countryName(gs, e.net.Owner)
		fmt.Fprintf(&b, "  %s: network power (raw save value) %.1f", owner, e.net.Power)
		if len(e.net.ActiveOperations) > 0 {
			fmt.Fprintf(&b, " (%d active ops!)", len(e.net.ActiveOperations))
		}
		fmt.Fprintf(&b, "\n")
	}
	if hostileActive == 0 {
		fmt.Fprintf(&b, "  No hostile networks with nonzero power or leader recorded.\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}
