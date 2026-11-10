package tools

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func popGroupNumberText(field gamestate.SavedPopGroupNumber, quoteValid bool) string {
	if len(field.Values) == 0 {
		return "not recorded"
	}
	if field.Valid && len(field.Values[0].Scalar) <= 128 {
		value := field.Values[0].Scalar
		if quoteValid {
			return strconv.Quote(value)
		}
		return value
	}
	var parts []string
	for _, value := range field.Values[:min(4, len(field.Values))] {
		switch {
		case value.IsScalar():
			preview := strconv.Quote(value.Scalar[:min(128, len(value.Scalar))])
			if len(value.Scalar) > 128 {
				preview += " (truncated; use raw save tools)"
			}
			parts = append(parts, preview)
		case value.Mixed:
			parts = append(parts, "mixed block")
		case value.IsList():
			parts = append(parts, "list")
		case value.IsObject() && len(value.Object.Pairs) == 0:
			parts = append(parts, "empty block")
		default:
			parts = append(parts, "object")
		}
	}
	if len(field.Values) == 1 {
		if field.Valid {
			return parts[0]
		}
		return "unsupported saved value " + parts[0]
	}
	if extra := len(field.Values) - len(parts); extra > 0 {
		parts = append(parts, fmt.Sprintf("%d more; use raw save tools", extra))
	}
	return "duplicate saved fields [" + strings.Join(parts, ", ") + "]"
}
