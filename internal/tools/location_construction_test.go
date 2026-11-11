package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

type locationCase struct {
	args  map[string]any
	want  []string
	avoid []string
}

func runLocationCases(t *testing.T, cases []locationCase) {
	t.Helper()
	for _, tc := range cases {
		result, err := handleLocationConstruction(context.Background(), rawRequest(tc.args))
		if err != nil || result.IsError {
			t.Fatalf("handler failed for %v: %v", tc.args, resultText(result))
		}
		text := resultText(result)
		for _, want := range tc.want {
			if !strings.Contains(text, want) {
				t.Fatalf("%v: missing %q in\n%s", tc.args, want, text)
			}
		}
		for _, avoid := range tc.avoid {
			if strings.Contains(text, avoid) {
				t.Fatalf("%v: unexpected %q in\n%s", tc.args, avoid, text)
			}
		}
	}
}

// Older saves: no colony table, queues and buildables name planet IDs.
func TestLocationConstructionLegacyPlanetQueues(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`planets={planet={
	 1={owner=3 build_queue=10 army_build_queue=11 externally_owned_build_queues={{key=4 value=12} {key=5 value=4294967295}}}
	 3={build_queue=30 build_queue=31}
	 4=none
	 6={build_queue=bad externally_owned_build_queues=oops}
	}}
	construction={queue_mgr={queues={
	 10={owner=3 location={type=2 id=1} simultaneous=1 type=planet items={100 101 102} disabled=no}
	 11={owner=3 location={type=2 id=1} type=army}
	 12={owner=4 location={type=2 id=1} type=planet items={}}
	 13={owner=6 location={type=2 id=1} type=planet items={103}}
	 14={owner=6 location={type=0 id=1} type=starbase items={105}}
	 30={owner=3 location={type=2 id=9} type=planet}
	 31=none
	}}
	item_mgr={items={
	 100={queue=10 paying_country=3 progress=0.5 progress_needed=100 buildable_district={district=district_city planet=1}}
	 101={queue=99 buildable_planet_building={building=b planet=2 zone=4 orbitable={starbase=1}}}
	 102=none
	 103={queue=13 buildable_planet_building={building=c planet=1}}
	 104={queue=55 buildable_district={district=d planet=1 planet=1}}
	 105={queue=14 buildable_district={district=e planet=1}}
	}}}`)}})
	s := server.NewMCPServer("test", "test")
	registerLocationConstruction(s)
	if _, ok := s.ListTools()["get_location_construction"]; !ok {
		t.Fatal("tool missing")
	}
	runLocationCases(t, []locationCase{
		{map[string]any{"planet_id": 1}, []string{
			"Planet 1 (planets.planet): unique saved record", "planet.colony=not recorded",
			"Item buildable_*.planet values are compared with planet 1 (no colony table in this save).",
			"planet 1 build_queue=10", "Queue 10: unique saved record", `type="planet" owner=3 location.type=2 location.id=1 simultaneous="1" disabled="no"`, "location.id equals planet 1",
			"items[0] Item 100: unique saved record", `item.queue=10 (item.queue back-reference agrees) paying_country=3 progress="0.5" progress_needed="100"`,
			`buildable_district: district="district_city" planet="1" (planet field equals planet 1)`,
			"item.queue=99 (item.queue back-reference differs or is unavailable)", `buildable_planet_building: building="b" planet="2" zone="4" orbitable=block(1 fields) (planet field does not equal planet 1)`,
			`items[2] Item 102: saved record is "none"`,
			"planet 1 army_build_queue=11", "items not recorded",
			"planet 1 externally_owned_build_queues[0].value=12 key=4", "items: empty",
			byIntWidth("planet 1 externally_owned_build_queues[1].value=4294967295 key=5", "planet 1 externally_owned_build_queues[1].value=unsupported or duplicate key=5"), byIntWidth("Queue 4294967295: record unavailable", "externally_owned_build_queues[1].value=unsupported or duplicate"),
			"Other queues whose saved location.id equals planet 1 with location.type 2 as used by the queues above (numeric candidates): 1", "Queue 13: unique saved record",
			"another location.type (likely another ID namespace; not listed): 1",
			"Items outside the queues above whose buildable_*.planet equals planet 1: 1", "Item 105",
		}, []string{"Queue 14:", "Colony ", "Item 104"}},
		{map[string]any{"planet_id": 3}, []string{"planet 3 build_queue [duplicate saved key]=30", "location.id does not equal planet 3", `Queue 31: saved record is "none"`, "with any location.type (no queue reached through saved fields matched this planet)"}, nil},
		{map[string]any{"planet_id": 4}, []string{`Planet 4 (planets.planet): saved record is "none"`, "None recorded", "equals planet 4: 0"}, nil},
		{map[string]any{"planet_id": 6}, []string{"planet 6 build_queue=unsupported or duplicate", "planet 6 externally_owned_build_queues=unsupported or duplicate"}, nil},
		{map[string]any{"planet_id": 77}, []string{"Planet 77 (planets.planet): record unavailable", "None recorded"}, nil},
		{map[string]any{"colony_id": 1}, []string{"Planet not resolved from the colony", "Colony 1 (colony table): table not recorded", "colony table: table not recorded"}, []string{"Other queues"}},
	})
	for _, args := range []map[string]any{nil, {"planet_id": 1, "colony_id": 7}, {"planet_id": -1}, {"colony_id": 1.5}, {"planet_id": 1, "save_id": 3}} {
		result, err := handleLocationConstruction(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Fatalf("invalid arguments accepted: %v", args)
		}
	}
}

// Newer saves: colony records carry army queues and buildables name colony IDs.
func TestLocationConstructionColonyFormat(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`planets={planet={
	 2={owner=3 colony=7 build_queue=20}
	 5={colony=8}
	 7={build_queue=70}
	}}
	colony={7={owner=3 carrier={type=planet reference=2} army_build_queue=21 externally_owned_build_queues={}} 8={carrier={type=ship reference=5}} 9={carrier={type=planet reference=99}}}
	construction={queue_mgr={queues={
	 20={owner=3 location={type=2 id=2} type=planet items={200}}
	 21={owner=3 location={type=2 id=2} type=army}
	 70={owner=3 location={type=2 id=7} type=planet items={700}}
	}}
	item_mgr={items={
	 200={queue=20 buildable_district={district=d planet=7}}
	 201={queue=88 buildable_district={district=d planet=7}}
	 700={queue=70 buildable_district={district=d planet=12}}
	 800={queue=88 buildable_district={district=d planet=8}}
	}}}`)}})
	runLocationCases(t, []locationCase{
		{map[string]any{"planet_id": 2}, []string{
			"planet.colony=7", "Colony 7 (colony table): unique saved record", `colony.carrier.type="planet" colony.carrier.reference=2`, "both saved sides agree",
			"compared with colony 7: this save has a colony table",
			"planet 2 build_queue=20", "colony 7 army_build_queue=21", `buildable_district: district="d" planet="7" (planet field equals colony 7)`,
			"Items outside the queues above whose buildable_*.planet equals colony 7: 1", "Item 201",
		}, []string{"Item 700"}},
		{map[string]any{"colony_id": 7}, []string{"Planet 2 (planets.planet)", "both saved sides agree", "planet 2 build_queue=20", "colony 7 army_build_queue=21"}, nil},
		{map[string]any{"planet_id": 7}, []string{"Item buildable_*.planet values are not compared", `planet="12"`}, []string{"Items outside", "Item 200", "Item 201"}},
		{map[string]any{"colony_id": 8}, []string{"Planet not resolved from the colony", `colony.carrier.type="ship"`, "None recorded", "Items outside the queues above whose buildable_*.planet equals colony 8: 1", "Item 800"}, []string{"Other queues"}},
		{map[string]any{"colony_id": 9}, []string{"Planet 99 (planets.planet): record unavailable", "Colony 9"}, []string{"Reciprocal"}},
		{map[string]any{"planet_id": 5}, []string{"planet.colony=8", "Colony 8", "only planet.colony names this colony"}, nil},
	})
}

func TestLocationConstructionReportsMissingTables(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`planets={planet={1={build_queue=10}}}`)}})
	result, err := handleLocationConstruction(context.Background(), rawRequest(map[string]any{"planet_id": 1}))
	if err != nil || result.IsError {
		t.Fatal("handler failed")
	}
	text := resultText(result)
	for _, want := range []string{"construction.queue_mgr.queues: table not recorded", "construction.item_mgr.items: table not recorded", "Queue 10: table not recorded"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	if strings.Contains(text, "Other queues") || strings.Contains(text, "Items outside") {
		t.Fatalf("candidate scan should be skipped without tables:\n%s", text)
	}
}
