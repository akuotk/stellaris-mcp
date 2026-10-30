package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestGalacticCommunityLinksRecordsAndPreservesMissing(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Home"}} 2={adjective={key="Other"}}}
	galactic_community={members={0 2} passed={7} proposed={8 9} community_formed="2201.01.01"}
	resolution={7={type="resolution_example" country=0 supporters={0 2} opponents={2} pass_date="2202.01.01"} 8={type="resolution_new" country=2}}`))
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(renderCommunity(gs))
	for _, want := range []string{"Home (ID 0)", "[resolution_example]", "supporters: Home (0), Other (2)", "opponents: Other (2)", "ID 9: record unavailable"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	if !strings.Contains(resultText(renderCommunity(&gamestate.GameState{})), "absent") {
		t.Fatal("missing community presented as empty")
	}
}
