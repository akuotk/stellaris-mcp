package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestDebrisSavedFieldsAndProjectReferences(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={
	 0={events={special_project={id=0 debris=0} special_project={debris=1} special_project={debris=2} special_project={debris=3} special_project={debris=4} special_project={debris=99} special_project={debris=5 debris=6} special_project={}}}
	 7={events={special_project={id=0 id=1 debris=0}}}
	 } debris={
	 0={country=0 from_country=7 date=2201.01.01 coordinate={type=2 id=0} ship_size=first ship_size=second component=first component=second resources={alloys=0} must_reanimate=no must_research=yes must_scavenge=no payload={known=zero anonymous}}
	 1={country=0 country=7} 2=none 3=none 03={country=7} 4=unexpected 5={from_country=7} 6={} bad_key={}
	 }`)}})
	s := server.NewMCPServer("test", "test")
	Register(s, saveDir, loc)
	if _, ok := s.ListTools()["get_debris"]; !ok {
		t.Fatal("debris tool missing from normal registration")
	}
	for _, tc := range []struct {
		args map[string]any
		want []string
	}{
		{nil, []string{"Debris 0 debris.country=0", "Country 0 country.events.special_project[0] id=0 debris=0", "Country 7 country.events.special_project[0] id=unsupported or duplicate debris=0", "numeric ID agrees", "numeric ID differs", "Debris 1 debris.country=unsupported or duplicate", `Debris 2 saved entry is "none"`, "Debris 3 has a duplicate saved ID (2 entries)", "Debris 4 record shape unsupported", "Debris 5 debris.country=not recorded (saved field)\n  No decoded saved project debris references", "Debris 99 record unavailable", "1 nonnumeric keys", "Project reference coverage incomplete for 1"}},
		{map[string]any{"debris_id": 0}, []string{`key="from_country": "7"`, `key="date": "2201.01.01"`, `key="coordinate": object`, `key="id": "0"`, `key="ship_size" [duplicate saved key]: "second"`, `key="component" [duplicate saved key]: "second"`, `key="alloys": "0"`, `key="must_research": "yes"`, `key="payload": mixed saved block`}},
		{map[string]any{"debris_id": 3}, []string{"Debris 3 has a duplicate saved ID", `Saved debris entry 0: "none"`, "Saved debris entry 1: object"}},
	} {
		result, err := handleDebris(context.Background(), rawRequest(tc.args))
		if err != nil || result.IsError {
			t.Fatal("normal debris handler failed")
		}
		output := resultText(result)
		for _, want := range tc.want {
			if !strings.Contains(output, want) {
				t.Fatalf("missing %q in %s", want, output)
			}
		}
		if strings.Contains(output, "country.events.special_project[6]") {
			t.Fatal("duplicate project debris field became an authoritative reference")
		}
	}
	result, err := handleDebris(context.Background(), rawRequest(map[string]any{"debris_id": 1.25}))
	if err != nil || !result.IsError {
		t.Fatal("non-integer debris ID accepted")
	}
	projects, err := handleSpecialProjects(context.Background(), rawRequest(nil))
	if err != nil || projects.IsError {
		t.Fatal("normal special projects handler failed")
	}
	output := resultText(projects)
	for _, want := range []string{"Saved debris ID candidate 0 debris.country=0", "Saved debris ID candidate 1 debris.country=unsupported or duplicate", "Saved debris reference 3 unresolved (has a duplicate saved ID", "Saved debris reference 99 unresolved (record unavailable)", "Debris field unsupported or duplicate", "Debris reference not recorded"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in %s", want, output)
		}
	}
}

func TestDebrisAbsentAndDuplicateTablesStayIsolated(t *testing.T) {
	for _, tc := range []struct{ table, want string }{
		{"", "table not recorded"},
		{"debris={}", "record unavailable"},
		{"debris={0={country=0}} debris={0={country=7}}", "table unsupported or duplicate"},
		{"debris={anonymous field=zero}", "table unsupported or duplicate"},
	} {
		gs, err := gamestate.Load([]byte("date=2201.01.01 country={0={events={special_project={debris=0}}}} " + tc.table))
		if err != nil || gs.Date != "2201.01.01" {
			t.Fatal("optional debris shape prevented unrelated save decoding")
		}
		output := resultText(renderDebris(gs, nil))
		if !strings.Contains(output, "Debris 0 "+tc.want) || !strings.Contains(output, "debris=0 (saved reference)") || strings.Contains(output, "numeric ID agrees") {
			t.Fatalf("table shape produced an authoritative link or lost a project reference: %s", output)
		}
	}
}
