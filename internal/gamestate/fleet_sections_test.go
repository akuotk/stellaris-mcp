package gamestate

import (
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

func TestDesignReadsAllSections(t *testing.T) {
	var stage ShipGrowthStage
	data := []byte(`section={ component={ slot="SMALL_GUN_01" template="LASER" } } section={ component={ slot="MEDIUM_GUN_01" template="CANNON" } }`)
	if err := clausewitz.Unmarshal(data, &stage); err != nil {
		t.Fatal(err)
	}
	if len(stage.Sections) != 2 || stage.Sections[1].Component[0].Template != "CANNON" {
		t.Fatalf("sections lost: %#v", stage.Sections)
	}
}
