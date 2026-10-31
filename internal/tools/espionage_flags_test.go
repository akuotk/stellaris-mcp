package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestOperationUnknownFlagTriggersRawFallbackWarning(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	for _, flags := range []string{`{example={unknown=1}}`, `{example={flag_days=0 flag_days=1}}`, `{example=0 example=1}`, `{example={anonymous_value}}`} {
		gs, err := gamestate.Load([]byte(`spy_networks={0={owner=0 active_operations={0}}} espionage_operations={operations={0={spy_network=0 flags=` + flags + `}}}`))
		if err != nil {
			t.Fatal(err)
		}
		id := 0
		text := resultText(renderEspionageOperations(gs, 0, &gamestate.Country{}, &id))
		if !strings.Contains(text, "raw tree/byte tools") || !strings.Contains(text, "Record decoding incomplete") {
			t.Fatal("unsupported flag did not surface raw fallback warning")
		}
	}
}
