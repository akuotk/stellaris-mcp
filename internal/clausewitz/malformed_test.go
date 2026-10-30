package clausewitz

import (
	"strings"
	"testing"
)

func TestRejectUnterminatedQuotedScalar(t *testing.T) {
	_, err := Parse([]byte(`name="unfinished`))
	if err == nil || !strings.Contains(err.Error(), "unterminated") {
		t.Fatalf("expected unterminated string error, got %v", err)
	}
}
