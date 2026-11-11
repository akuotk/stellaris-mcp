package tools

import (
	"fmt"
	"math"
	"sort"
	"strconv"
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
	var requested *int
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
		id := int(n)
		requested = &id
	}
	return gs.SelectCountry(requested)
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
	return fmt.Sprintf("%s used / %s maximum", armyFloat(c.UsedNavalCapRaw), armyFloat(c.NavalCap))
}

func toolError(err error) *mcp.CallToolResult {
	message := err.Error()
	if strconv.IntSize == 32 && strings.Contains(message, "overflows int") {
		message += " (this 32-bit build cannot represent saved IDs above 2147483647; use the 64-bit build; the raw tree, byte, scalar and search tools still read every saved value)"
	}
	return mcp.NewToolResultError(message)
}

func writeSavedTableStatus[T any](b *strings.Builder, label string, table gamestate.ConnectionRecordTable[T]) {
	if !table.Recorded {
		fmt.Fprintf(b, "%s table not recorded\n", label)
	}
	if table.TableIssue || len(table.DecodeErrors) > 0 {
		fmt.Fprintf(b, "%s table incomplete, unsupported or duplicate. Affected records=%d. Use raw tree/byte tools.\n", label, len(table.DecodeErrors))
	}
}

func optionalIntRefText(ref gamestate.OptionalIntRef) string {
	if ref.Unsupported {
		return "unsupported or duplicate"
	}
	return armyInt(ref.Value)
}

func compareSavedIDs(a, b int) string {
	if a == b {
		return "agrees with"
	}
	return "differs from"
}
