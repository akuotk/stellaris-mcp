package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestSpecialProjectsRawRecordsAndSameCountryCandidates(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`country={0={events={
	 special_project={id=0 special_project=example planet=0 days_left=-1 status=0 scope={from={id=0}}}
	 special_project={id=0 debris=0}
	 special_project={id=2 planet=99}
	 special_project={id=0 id=2 planet=0 planet=2}
	 special_project=none
	 } last_completed_special_project=example tech_status={physics_queue={{special_project=0 progress=0 date=0} {special_project=2} {special_project=99} {special_project=0 special_project=1}}}}
	 1={events={special_project={id=99 special_project=foreign_record}}}}
	 planets={planet={0={name={key="Planet candidate"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderSpecialProjects(gs, 0, &c))
	for _, want := range []string{
		`last_completed_special_project="example" (saved field)`, "country.events.special_project[0] (saved order)",
		`key="days_left": "-1" (saved scalar)`, `key="status": "0" (saved scalar)`,
		"nested saved block", "Multiple saved project records share this ID",
		"Project ID unsupported or duplicate", "Planet field unsupported or duplicate",
		"Unsupported project record shape", "Saved planet ID candidate 0 Planet candidate",
		"Saved planet ID candidate 99 unavailable",
		`physics_queue[0] special_project=0 progress=0 date="0" (saved values)`,
		"Ambiguous ID with 2 decoded same-country project candidates", "One decoded same-country project ID candidate",
		"No decoded same-country project ID candidate", "physics_queue[3].special_project unsupported or duplicate",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "foreign_record") {
		t.Fatal("another country's ID match escaped the selected scope")
	}
}

func TestSpecialProjectsRegisteredHandlerAndEmptyStates(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={events={}}
	1={events={special_project={id=0}} events={special_project={id=1}} last_completed_special_project=one last_completed_special_project=two} 2={}}`)}})
	s := server.NewMCPServer("test", "test")
	registerSpecialProjects(s)
	if _, ok := s.ListTools()["get_special_projects"]; !ok {
		t.Fatal("special project tool missing")
	}
	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{nil, "No saved country.events.special_project records"},
		{map[string]any{"country_id": 1}, "country.events unsupported or duplicate"},
		{map[string]any{"country_id": 2}, "country.events not recorded"},
	} {
		result, err := handleSpecialProjects(context.Background(), rawRequest(tc.args))
		if err != nil || result.IsError || !strings.Contains(resultText(result), tc.want) || strings.Contains(resultText(result), "country.events.special_project[0]") {
			t.Fatal("normal handler failed or ambiguous records escaped")
		}
	}
}
