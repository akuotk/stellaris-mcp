package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
)

func TestFleetTemplatesReferencesAndDetails(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={fleet_template_manager={fleet_template={0 0 1 99}} fleets_manager={owned_fleets={{fleet=0}}}}} fleet={0={fleet_template=0}} fleet_template={0={fleet=0 fleet_size=0 fleet_template_design={{design=0 count=0} {ship_design_implementation={design=1}}} home_base={orbitable={starbase=0}} all_queued={construction_queue_items={0 99}}} 1={fleet=7}} ship_design={0={name={key="Example"}}} construction={item_mgr={items={0={queue=0 paying_country=0}}}} starbase_mgr={starbases={0={level=outpost}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	summary := resultText(renderFleetTemplates(gs, 0, &c, nil))
	for _, want := range []string{"Template 0:", "reciprocal template/fleet IDs agree", "Template 1:", "Referenced fleet record unavailable", "Template 99: referenced record unavailable"} {
		if !strings.Contains(summary, want) {
			t.Errorf("missing %q in %s", want, summary)
		}
	}
	id := 0
	detail := resultText(renderFleetTemplates(gs, 0, &c, &id))
	for _, want := range []string{"Design entry 0", "design=0 count=\"0\"", "Design entry 1", "count=not recorded", "Starbase ID candidate", "item ID candidate 99 unavailable"} {
		if !strings.Contains(detail, want) {
			t.Errorf("missing %q in %s", want, detail)
		}
	}
	id = 8
	if !renderFleetTemplates(gs, 0, &c, &id).IsError {
		t.Fatal("unscoped template accepted")
	}
}
