package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
	"testing"
)

func TestToolsAdvertiseReadOnly(t *testing.T) {
	oldDir, oldLoc := saveDir, loc
	t.Cleanup(func() { saveDir, loc = oldDir, oldLoc })
	s := server.NewMCPServer("test", "test")
	Register(s, t.TempDir(), gamestate.NewLocalizer(t.TempDir()))
	for name, entry := range s.ListTools() {
		a := entry.Tool.Annotations
		if a.ReadOnlyHint == nil || !*a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint {
			t.Errorf("%s has incorrect read-only annotations", name)
		}
	}
}
