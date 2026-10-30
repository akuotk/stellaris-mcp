package tools

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
)

var (
	saveDir string
	loc     *gamestate.Localizer
)

func loadSave(req mcp.CallToolRequest) (*gamestate.GameState, error) {
	if value, ok := req.GetArguments()["save_id"]; ok {
		if _, ok := value.(string); !ok {
			return nil, fmt.Errorf("save_id must be a string")
		}
	}
	path, err := gamestate.FindSaveByID(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return nil, err
	}
	return gamestate.LoadFromZip(path)
}

// l resolves a game key to its localized display name.
func l(key string) string {
	return loc.Resolve(key)
}

// ll resolves a slice of game keys.
func ll(keys []string) []string {
	return loc.ResolveAll(keys)
}

func name(n gamestate.LocalizedName) string { return loc.ResolveName(n) }

func getCountry(gs *gamestate.GameState, req mcp.CallToolRequest) (int, *gamestate.Country, error) {
	var id int
	if value, provided := req.GetArguments()["country_id"]; provided {
		var n float64
		switch v := value.(type) {
		case float64:
			n = v
		case int:
			n = float64(v)
		default:
			return 0, nil, fmt.Errorf("country_id must be an integer")
		}
		max := float64(^uint(0) >> 1)
		if max > 9007199254740991 {
			max = 9007199254740991
		}
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > max || math.Trunc(n) != n {
			return 0, nil, fmt.Errorf("country_id must be a nonnegative exact integer")
		}
		id = int(n)
	} else {
		if len(gs.Player) != 1 || gs.Player[0].CountryID == nil {
			return 0, nil, fmt.Errorf("cannot determine a unique player country; specify country_id")
		}
		id = *gs.Player[0].CountryID
	}
	c, ok := gs.Country[id]
	if !ok {
		return 0, nil, fmt.Errorf("country %d not found", id)
	}
	return id, &c, nil
}

func countryName(gs *gamestate.GameState, id int) string {
	if c, ok := gs.Country[id]; ok {
		return name(c.Adjective)
	}
	return fmt.Sprintf("Country %d", id)
}

func formatResources(r map[string]float64) string {
	if len(r) == 0 {
		return "(none)"
	}
	keys := make([]string, 0, len(r))
	for k := range r {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %.1f", l(k), r[k]))
	}
	return strings.Join(parts, ", ")
}

func formatNavalCapacity(c *gamestate.Country) string {
	if c.NavalCap == nil {
		return fmt.Sprintf("%.0f used; maximum unavailable", c.UsedNavalCap)
	}
	return fmt.Sprintf("%.0f / %.0f", c.UsedNavalCap, *c.NavalCap)
}

func toolError(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}
