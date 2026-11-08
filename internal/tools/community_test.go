package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestCommunityResolutionVotingPreservesAmbiguity(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={}} galactic_community={community_formed=yes members={0} proposed={0} failed={0} voting=0} resolution={0={type=test country=0 voting={0=yes 00=no invalid=unknown}}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := resultText(renderResolutionDetail(gs, 0))
	for _, want := range []string{"Saved resolution 0", "duplicate saved numeric key", "unsupported key", "Saved community voting=0"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
}
