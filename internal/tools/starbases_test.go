package tools

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestStarbasesCandidateReferencesAndDetails(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Synthetic country"} fleets_manager={owned_fleets={{fleet=0} {}}}}}
	 starbase_mgr={starbases={
	 0={station=0 build_queue=0 shipyard_build_queue=1 ship_design_implementation={design=0 upgrade=0 growth_stage=0} level=outpost
	 modules={2=none 0=shipyard 0=anchorage} buildings={} orbitals={4=0}}
	 1={station=7 shipyard_build_queue=9 modules={unknown=bad 0=none} buildings=unsupported}
	 2={station=99 build_queue=0}
	 3={} 4={build_queue=3} 5={station=invalid build_queue=0} 6={build_queue=0} 6=none
	 }}
	 construction={queue_mgr={queues={0={owner=0 location={type=0 id=0} items={2 2}} 1={owner=1} 3={}}}}
	 ships={0={name={key="Candidate station"} fleet=0 coordinate={origin=4} ship_design_implementation={design=1}} 7={fleet=0 coordinate={unexpected={}}}}
	 fleet={0={name={key="Referenced fleet"} ships={7}}}
	 ship_design={0={name={key="Saved design"}}}
	 galactic_object={4={name={key="System four"} starbases={0 0 4294967295 987654}} 5={starbases={0 4294967295}} 6={starbases=unsupported} 7={}}
	 `))
	if err != nil {
		t.Fatal(err)
	}
	country := gs.Country[0]
	summary := resultText(renderStarbases(gs, 0, &country, nil))
	for _, want := range []string{"Starbase 0:", "Starbase 1:", "Starbase 2:", "fleet ID: 1", "Base/ship saved design IDs disagree", "fleet.ships membership: missing reverse reference", byIntWidth("2 duplicate references", "Unsupported system.starbases fields: 3"), byIntWidth("coordinate ID agrees", "ship.coordinate.origin=4"), byIntWidth("coordinate ID differs", "reference coverage incomplete"), byIntWidth("undecodable starbase lists=1 (their", "undecodable starbase lists=3 (their"), byIntWidth("Unsupported system.starbases fields: 1", "Unsupported system.starbases fields: 3"), "Systems without recorded starbase references: 1", "station ID has no decoded ship candidate", "Slot decoding incomplete", "Starbase decoding incomplete", "do not prove starbase ownership", "Saved design ID record: Saved design", byIntWidth("raw 4294967295=2 (meaning unverified); other IDs without a decoded base record=1", "raw 4294967295=0 (meaning unverified); other IDs without a decoded base record=0"), "Survey coverage incomplete"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in %s", want, summary)
		}
	}
	for _, excluded := range []string{"Starbase 3:", "Starbase 4:", "Starbase 5:", "Starbase 6:", "modules[0]"} {
		if strings.Contains(summary, excluded) {
			t.Fatalf("summary incorrectly contains %q", excluded)
		}
	}
	if strings.Index(summary, "Starbase 0:") > strings.Index(summary, "Starbase 1:") {
		t.Fatal("starbase order is unstable")
	}
	selected := 0
	details := resultText(renderStarbases(gs, 0, &country, &selected))
	for _, want := range []string{"build_queue 0: owner=0", "shipyard_build_queue 1: owner=1", "Queue owner differs from selected country", "location.type=0 location.id=0", "key=2 value=none", "key=0 value=anchorage [duplicate slot key]", "buildings: empty", "orbitals (raw key/value; namespace unverified)[0] (saved order): key=4 value=0"} {
		if !strings.Contains(details, want) {
			t.Fatalf("missing detail %q in %s", want, details)
		}
	}
	selected = 1
	details = resultText(renderStarbases(gs, 0, &country, &selected))
	for _, want := range []string{"shipyard_build_queue 9: referenced queue unavailable", "build_queue: not recorded", "buildings: unsupported saved entries", "orbitals (raw key/value; namespace unverified): not recorded", "ship.coordinate.origin=not recorded"} {
		if !strings.Contains(details, want) {
			t.Fatalf("missing unresolved detail %q in %s", want, details)
		}
	}
	if strings.Contains(details, "buildings: empty") {
		t.Fatal("unsupported slots presented as empty")
	}
	selected = 3
	if !renderStarbases(gs, 0, &country, &selected).IsError {
		t.Fatal("unmatched starbase selected")
	}
}

func TestStarbasesIncompleteConstructionCannotCreateCountryPath(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`starbase_mgr={starbases={0={build_queue=0} 1={station=0 build_queue=0}}}
	 construction={queue_mgr={queues={0={owner=invalid}}}}
	 ships={0={fleet=0 coordinate=unsupported}}
	 country={0={fleets_manager={owned_fleets={{fleet=0}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	country := gs.Country[0]
	summary := resultText(renderStarbases(gs, 0, &country, nil))
	if !strings.Contains(summary, "Construction decoding incomplete") || strings.Contains(summary, "Starbase 0:") || !strings.Contains(summary, "Starbase 1:") || !strings.Contains(summary, "ship.coordinate: unsupported") {
		t.Fatal("partial queue parsing created a false country path or hid warnings")
	}
	selected := 1
	if !strings.Contains(resultText(renderStarbases(gs, 0, &country, &selected)), "queue decoding incomplete") {
		t.Fatal("partial queue displayed as complete")
	}
	if !strings.Contains(resultText(renderStarbases(&gamestate.GameState{}, 0, &country, nil)), "Starbase table not recorded") {
		t.Fatal("missing table presented as empty")
	}
}

func TestStarbasesRegisteredHandlerAndInputValidation(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={}} starbase_mgr={starbases={0={build_queue=0 modules={0=none}}}} construction={queue_mgr={queues={0={owner=0}}}}`)}})
	s := server.NewMCPServer("test", "test")
	registerStarbases(s)
	entry, ok := s.ListTools()["get_starbases"]
	if !ok {
		t.Fatal("get_starbases not registered")
	}
	if entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint || entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("incorrect read-only annotations")
	}
	for _, key := range []string{"save_id", "country_id", "starbase_id"} {
		if _, ok := entry.Tool.InputSchema.Properties[key]; !ok {
			t.Fatalf("missing input %q", key)
		}
	}
	for _, value := range []any{"0", nil, true, -1, 0.5, math.NaN(), math.Inf(1), float64(1 << 53)} {
		result, err := handleStarbases(context.Background(), rawRequest(map[string]any{"starbase_id": value}))
		if err != nil || result == nil || !result.IsError {
			t.Fatalf("invalid starbase ID accepted: %v", value)
		}
	}
	for _, args := range []map[string]any{nil, {"starbase_id": 0}, {"country_id": 0, "starbase_id": float64(0)}} {
		result, err := handleStarbases(context.Background(), rawRequest(args))
		if err != nil || result.IsError || !strings.Contains(resultText(result), "Starbase 0:") {
			t.Fatal("valid normal handler request failed")
		}
	}
}
