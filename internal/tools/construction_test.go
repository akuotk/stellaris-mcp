package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestConstructionOutputReferencesOrderAndOrphans(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Synthetic nation"}}}
 construction={queue_mgr={queues={
  0={owner=0 location={type=0 id=0} simultaneous=0 disabled=no items={0 1 1 99 3}}
  1={owner=0 items={}}
  3={owner=1 items={8}}
  4={owner=1 items={4}}
  5={owner=0}
 }} item_mgr={items={
  0={queue=0 paying_country=0 progress=0 progress_needed=5 resources={energy=0.0001234} buildable_planet_building={building="example_building" planet=0 zone=0}}
  1={queue=7 paying_country=0}
  2={queue=88 paying_country=0}
  3={paying_country=0}
  4={queue=4 paying_country=0 buildable_ship={ship_design_implementation={design=7 upgrade=0 growth_stage=0} orbitable={starbase=0}}}
  5={queue=5 paying_country=0}
  6={paying_country=0}
  8={queue=3 paying_country=1}
 }}}
 colony={0={carrier={type=planet reference=3}}} planets={planet={3={colony=0 name={key="Candidate planet"}}}}
 ship_design={7={name={key="Example design"}}}
 `))
	if err != nil {
		t.Fatal(err)
	}
	country := gs.Country[0]
	text := resultText(renderConstruction(gs, 0, &country))
	for _, want := range []string{"Queue 0:", "owner=0 simultaneous=0 disabled=false", "type=0 id=0", "Position 0, item 0", "Position 2, item 1 [duplicate queue reference]", "item.queue=7", "referenced item unavailable", "back-reference: queue not recorded", "item.queue points here but queue.items does not contain it", "Unresolved item 2: queue=88", "Unresolved item 6: queue=not recorded", "energy=0.0001234", "progress=0 progress_needed=5", "Possible colony ID match", "Ship design: Example design", "ship_design.upgrade=0", "No recognized buildable payload"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "Queue 3:") {
		t.Fatal("unrelated foreign queue included")
	}
	if strings.Index(text, "Queue 0:") > strings.Index(text, "Queue 1:") {
		t.Fatal("queue IDs not stable")
	}
	if strings.Contains(text, "%") || strings.Contains(text, "ETA=") {
		t.Fatal("unverified progress units interpreted")
	}
}

func TestConstructionIncompleteDecodeWarning(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`construction={item_mgr={items={0={resources={unsupported={}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(renderConstruction(gs, 0, &gamestate.Country{}))
	if !strings.Contains(text, "Construction decoding incomplete") || !strings.Contains(text, "raw tree/byte tools") {
		t.Fatal("unsupported shape silently presented as complete")
	}
}
