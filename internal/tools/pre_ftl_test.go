package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestPreFTLSavedCountryAndObservationReferences(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`country={
	 0={adjective={key="First country"} type=primitive preftl_age=0 capital=0 awareness=0.000001 old_awareness=0 fleets_manager={owned_fleets={{fleet=6}}}}
	 1={adjective={key="Observed country"} type=primitive preftl_age=example capital=2}
	 2={adjective={key="Observing country"} type=default fleets_manager={owned_fleets={{fleet=0} {fleet=6}}}}
	 3={type=default preftl_age=historical capital=0 capital=2}
	 4={type=primitive capital=99}
	 5={type=primitive type=default preftl_age={unexpected=yes} fleets_manager=unknown}
	 } planets={planet={
	 0={name={key="First planet"} owner=1 controller=0 shipclass_orbital_station=0}
	 2={owner=1 owner=2 shipclass_orbital_station=2}
	 4=none 04={owner=0}
	 5={owner=99 shipclass_orbital_station=99}
	 6={owner=1 shipclass_orbital_station=6}
	 7={shipclass_orbital_station=6}
	 }} fleet={
	 0={name={key="First fleet"} ship_class=shipclass_observation_station ships={0 1 2 3 3 4} movement_manager={coordinate={origin=0} orbit={orbitable={planet=0}}}}
	 1={ship_class=shipclass_observation_station ship_class=shipclass_military movement_manager={orbit={orbitable={planet=0}}}}
	 2=none 02={ship_class=shipclass_observation_station}
	 3={ship_class=shipclass_observation_station movement_manager={orbit={orbitable={planet=0 ship=0}}}}
	 4={ship_class=shipclass_observation_station movement_manager={orbit={orbitable={planet=4}}}}
	 5={ship_class=shipclass_observation_station movement_manager={orbit={orbitable={planet=5}}}}
	 6={ship_class=shipclass_observation_station movement_manager={orbit={orbitable={planet=6}}}}
	 } ships={0={fleet=0} 1={fleet=7} 2=none 3={} 4={fleet=0} 04=none}`)}})
	s := server.NewMCPServer("test", "test")
	Register(s, saveDir, loc)
	if _, ok := s.ListTools()["get_pre_ftl"]; !ok {
		t.Fatal("pre-FTL tool missing from normal registration")
	}
	result, err := handlePreFTL(context.Background(), rawRequest(nil))
	if err != nil || result.IsError {
		t.Fatal("normal pre-FTL handler failed")
	}
	text := resultText(result)
	for _, want := range []string{
		`Country 0 (First country) type="primitive" preftl_age="0" awareness="0.000001" old_awareness="0"`,
		"capital=0 (saved reference)", "Capital planet ID candidate 0 (First planet) owner=1 controller=0", "Planet owner numeric ID differs", "Planet controller numeric ID agrees",
		`type="default" preftl_age="historical"`, "capital=unsupported or duplicate", `type="primitive" preftl_age=not recorded`, "Capital planet ID candidate 99 (record unavailable)",
		"type=unsupported or duplicate (use raw tools) preftl_age=unsupported or duplicate", "Saved owned_fleets scope incomplete for 1", "owner=unsupported or duplicate",
		"Orbital-station fleet ID candidate 2 (record unsupported or duplicate)",
		"fleet table incomplete, unsupported or duplicate. Affected records=1", "ships table incomplete, unsupported or duplicate. Affected records=1", "planets.planet table incomplete, unsupported or duplicate. Affected records=1",
		`Fleet 0 (First fleet) ship_class="shipclass_observation_station"`, "Saved owned_fleets reference from country 2 (Observing country)", "movement_manager.coordinate.origin=0 orbit.orbitable.planet=0", "Planets with matching saved shipclass_orbital_station IDs: [0]",
		"Reciprocal saved fleet orbit and planet orbital-station IDs agree", "Orbit planet owner country ID candidate 1 (Observed country)", "Planet owner ID differs from the country ID with the saved owned_fleets reference",
		"ships[0]=0 ship.fleet=0 (agrees with the saved fleet ID)", "ships[1]=1 ship.fleet=7 (differs from the saved fleet ID)", "ships[2]=2 (record unavailable)", "ships[3]=3 (duplicate saved ship ID reference)", "ships[5]=4 (record unsupported or duplicate)",
		"Observation class evidence is ambiguous", "orbit.orbitable.planet=unsupported or duplicate", `key="ship": "0"`, "Orbit planet ID candidate 4 (record unsupported or duplicate)",
		"Saved planet orbital-station ID differs from the fleet ID", "Orbit planet owner country ID candidate 99 (record unavailable)", "Ambiguous saved owned_fleets references from country IDs [0 2]", "Ambiguous planet reverse references",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Count(text, "Reciprocal saved fleet orbit and planet orbital-station IDs agree") != 1 || strings.Contains(text, "Country 2 (Observing country) type=") {
		t.Fatal("an ambiguous orbit formed a reciprocal link or observer scope widened the country marker list")
	}
}

func TestPreFTLTablePresenceAndDuplicates(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"", "country table not recorded"},
		{"country={}", "No decoded country records with the selected markers"},
		{"country={0={type=primitive}} country={1={type=primitive}}", "country table incomplete, unsupported or duplicate"},
		{"planets={planet={} planet={0={owner=0}}}", "planets.planet table incomplete, unsupported or duplicate"},
		{"fleet={anonymous key=value}", "fleet table incomplete, unsupported or duplicate"},
	} {
		syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(tc.input)}})
		result, err := handlePreFTL(context.Background(), rawRequest(nil))
		if err != nil || result.IsError || !strings.Contains(resultText(result), tc.want) || strings.Contains(resultText(result), "Country 0") {
			t.Fatalf("table absence or ambiguity produced an unexpected result for %q", tc.input)
		}
	}
}
