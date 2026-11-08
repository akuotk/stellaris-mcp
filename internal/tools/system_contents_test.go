package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestSystemContentsCoordinatesMoonsAndDeposits(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`galactic_object={
	 0={name={key="First system"} star_class=sc_example coordinate={x=0 y=-0.00001 origin=0} asteroid_belts={{type=example radius=0.123456789}} ambient_object={{type=ambient}} planet=0 planet=1 planet=2 planet=3 planet=3 planet=4 planet=5 planet=6 planet=7 planet=8 planet=9 planet={unexpected=planet}}
	 1=none 01={name={key="Ambiguous system"} planet=0} 2={planet={}} 3={} 4={planet={0 1}}
	 } planets={planet={
	 0={name={key="Primary body"} planet_class=pc_example coordinate={origin=0 x=0 y=0} orbit=999 is_moon=no moons={1 2} deposits={0 1 2 3 3} surveyed=no surveyed_by=0 owner=0}
	 1={name={key="Moon body"} coordinate={origin=0} moon_of=0 orbit=42}
	 2={coordinate={origin=8} moon_of=6}
	 3={coordinate={origin=0}}
	 4=none 04={}
	 5={coordinate={origin=0 origin=1} moon_of=77}
	 6={moons={2 2} deposits={0} deposits={1}}
	 7={moon_of=7}
	 8={moon_of=6 moons=unsupported}
	 }} deposit={0={type=d_example deposit_holder={type=planet id=0}} 1=none 01={type=d_other} 3={type=d_repeat deposit_holder={type=unknown id=9}}}`)}})
	s := server.NewMCPServer("test", "test")
	Register(s, saveDir, loc)
	if _, ok := s.ListTools()["get_system_contents"]; !ok {
		t.Fatal("system contents tool missing from normal registration")
	}
	result, err := handleSystemContents(context.Background(), rawRequest(map[string]any{"system_id": 0}))
	if err != nil || result.IsError {
		t.Fatal("normal system contents handler failed")
	}
	text := resultText(result)
	for _, want := range []string{
		"System name: First system", `key="star_class": "sc_example"`, `key="y": "-0.00001"`, `key="radius": "0.123456789"`, `key="type": "ambient"`,
		"System planet reference[0]=0", "Body 0 (Primary body)", `planet_class="pc_example" owner=0 surveyed="no" surveyed_by=0`, "coordinate.origin=0 (agrees with the saved system ID)", `orbit="999"`,
		"moons[0]=1: reciprocal saved moon_of and moons IDs agree", "moon_of 0: reciprocal saved moon_of and moons IDs agree", "moons[1]=2: child moon_of=6 differs", "coordinate.origin=8 (differs from the saved system ID)",
		"Duplicate saved planet ID reference. Body link unresolved", "Body 4: duplicate saved record ID", "coordinate.origin=unsupported or duplicate", "moon_of 77: body 77 absent from this system", "moon_of 6: duplicate child ID references", "moon_of 7: same saved parent and child ID", "parent moons list has no matching child ID", "moons=unsupported saved shape",
		"Body 9: referenced record unavailable", "System planet reference[11] unsupported", `key="unexpected": "planet"`,
		"Deposit 0:", "holder ID matches referencing planet ID", "Deposit 1: duplicate saved record ID", "Deposit 2: referenced record unavailable", "holder ID differs from referencing planet ID", "Duplicate deposit reference 3", "Deposit references: duplicate saved fields",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Count(text, "reciprocal saved moon_of and moons IDs agree") != 2 || strings.Contains(text, "Body 3 (") || strings.Contains(text, "body 999") {
		t.Fatal("duplicate or raw orbit values formed body links")
	}
	for id, want := range map[int]string{1: "System 1: duplicate saved record ID", 2: "System planet references empty", 3: "System planet references not recorded", 4: "System planet reference[1]=1", 99: "System 99: referenced record unavailable"} {
		result, err := handleSystemContents(context.Background(), rawRequest(map[string]any{"system_id": id}))
		if err != nil || result.IsError || !strings.Contains(resultText(result), want) {
			t.Fatalf("unexpected response for synthetic system %d", id)
		}
	}
	for _, args := range []map[string]any{nil, {"system_id": -1}, {"system_id": 0.1}, {"system_id": 0, "save_id": 0}} {
		result, err := handleSystemContents(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Fatal("invalid system selection accepted")
		}
	}
}

func TestSystemContentsMissingAndDuplicateTables(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	for _, tc := range []struct{ input, want string }{
		{"", "galactic_object table not recorded"},
		{"galactic_object={0={planet=0}}", "planets.planet table not recorded"},
		{"galactic_object={0={planet=0}} galactic_object={0={planet=1}}", "galactic_object table incomplete, unsupported or duplicate"},
		{"galactic_object={0={planet=0}} planets={planet={} planet={0={}}}", "planets.planet table incomplete, unsupported or duplicate"},
		{"galactic_object={0={planet=0}} planets={planet={0={deposits={0}}}}", "deposit table not recorded"},
		{"galactic_object={0={planet=0}} planets={planet={0={deposits={0}}}} deposit={anonymous key=value}", "Deposit table decoding incomplete"},
	} {
		syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(tc.input)}})
		result, err := handleSystemContents(context.Background(), rawRequest(map[string]any{"system_id": 0}))
		if err != nil || result.IsError || !strings.Contains(resultText(result), tc.want) {
			t.Fatalf("missing %q for synthetic table", tc.want)
		}
	}
}

func TestSystemContentsNebulaCandidatesPreserveUncertainty(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`galactic_object={0={name={key=system}} 1={}} nebula={name={key=first} galactic_object=0 galactic_object=0 galactic_object=1 radius=0.125} nebula=bad nebula={galactic_object={invalid=0}} nebula={galactic_object=1}`)}})
	result, err := handleSystemContents(context.Background(), rawRequest(map[string]any{"system_id": 0}))
	if err != nil || result.IsError {
		t.Fatal("system contents failed")
	}
	got := resultText(result)
	for _, want := range []string{"Nebula[0]: 2 saved galactic_object ID match(es)", "candidate only", "Nebula candidate coverage incomplete: 2", `key="radius": "0.125"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "Nebula[3]") {
		t.Fatal("unmatched nebula included")
	}
}

func TestSystemContentsAmbientReferences(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`galactic_object={0={ambient_object={0 1 1 2 3 4 5}} 1={ambient_object={0} ambient_object={2}}} ambient_object={0={type=synthetic coordinate={origin=0} properties={flag=yes}} 1={type=duplicate_ref} 2={coordinate={origin=9}} 3={type=first} 03={type=duplicate_record} 4=bad}`)}})
	result, err := handleSystemContents(context.Background(), rawRequest(map[string]any{"system_id": 0}))
	if err != nil || result.IsError {
		t.Fatal("system contents failed")
	}
	got := resultText(result)
	for _, want := range []string{"Ambient object 0 candidate: coordinate.origin=0 (agrees with the saved system ID)", "2 duplicate saved references; candidate ambiguous", "coordinate.origin=9 (differs from the saved system ID)", "Ambient object 3: duplicate", "Ambient object 4: referenced record has an unsupported saved shape", "Ambient object 5: referenced record unavailable", `key="type": "synthetic"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	result, err = handleSystemContents(context.Background(), rawRequest(map[string]any{"system_id": 1}))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "Ambient object references: duplicate saved fields") {
		t.Fatal("duplicate list not reported")
	}
}
