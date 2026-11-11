package tools

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// byIntWidth selects an expectation by the platform int size. Saved IDs above
// 2147483647 (such as the frequent 4294967295) do not fit a 32-bit int; typed decoding
// then reports the field or record as unsupported instead of wrapping the value.
func byIntWidth(wide, narrow string) string {
	if strconv.IntSize == 64 {
		return wide
	}
	return narrow
}

func TestToolErrorExplainsIntWidthOverflow(t *testing.T) {
	text := resultText(toolError(errors.New(`parse gamestate: value "3000000000" overflows int`)))
	explained := strings.Contains(text, "this 32-bit build cannot represent saved IDs above 2147483647")
	if explained != (strconv.IntSize == 32) {
		t.Fatalf("int width %d explanation=%v: %s", strconv.IntSize, explained, text)
	}
	if other := resultText(toolError(errors.New("record unavailable"))); other != "record unavailable" {
		t.Fatalf("unrelated error changed: %s", other)
	}
}
