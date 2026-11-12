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

// maxSaveRecordID is the largest saved ID a request can name: it must fit the
// platform int and survive a JSON number exactly (2^53-1).
const maxSaveRecordID = min(int64(^uint(0)>>1), 1<<53-1)

// maxRecordID is maxSaveRecordID as an int, for rawInteger bounds.
const maxRecordID = int(maxSaveRecordID)

// requireStringArgs rejects provided arguments that are not JSON strings.
// GetString would otherwise silently fall back to its default for them.
func requireStringArgs(req mcp.CallToolRequest, keys ...string) error {
	args := req.GetArguments()
	for _, key := range keys {
		if value, ok := args[key]; ok {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("%s must be a string", key)
			}
		}
	}
	return nil
}

// savedInt and savedFloat print optional saved numbers without inventing a zero.
func savedInt(value *int) string {
	if value == nil {
		return "not recorded"
	}
	return fmt.Sprint(*value)
}

func savedFloat(value *float64) string {
	if value == nil {
		return "not recorded"
	}
	return fmt.Sprintf("%g", *value)
}

func loadSave(req mcp.CallToolRequest) (*gamestate.GameState, error) {
	if err := requireStringArgs(req, "save_id"); err != nil {
		return nil, err
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
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > float64(maxSaveRecordID) || math.Trunc(n) != n {
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
	return fmt.Sprintf("%s used / %s maximum", savedFloat(c.UsedNavalCapRaw), savedFloat(c.NavalCap))
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
	return savedInt(ref.Value)
}

func compareSavedIDs(a, b int) string {
	if a == b {
		return "agrees with"
	}
	return "differs from"
}
