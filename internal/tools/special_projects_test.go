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

func TestFleetSpecialProjectOrdersUseSameCountryRecords(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={
	 0={events={special_project={id=0} special_project={id=2} special_project={id=2}} fleets_manager={owned_fleets={{fleet=0} {fleet=1} {fleet=2} {fleet=3} {fleet=4} {fleet=5} {fleet=5} {fleet=6} {fleet=7} {fleet=8} {fleet=9} {}}}}
	 1={events={special_project={id=99 special_project=foreign_record}} fleets_manager={owned_fleets={{fleet=4}}}}
	 2={fleets_manager={owned_fleets={{fleet=8 fleet=9}}}}
	 }
	 fleet={
	 0={name={key="Direct fleet"} current_order={collect_data_fleet_order={special_project=0}}}
	 1={name={key="Auto fleet"} current_order={automate_fleet_order={sub_order={collect_data_fleet_order={special_project=2}}}}}
	 2={current_order={collect_data_fleet_order={special_project=99}}}
	 3={current_order={collect_data_fleet_order={special_project=0 special_project=2}}}
	 4={current_order={collect_data_fleet_order={special_project=0}}}
	 5={current_order={collect_data_fleet_order={special_project=0}}}
	 6={current_order={collect_data_fleet_order={special_project=0}}} 6=none
	 8={current_order={research_anomaly_order={special_project=0}}}
	 9={current_order={collect_data_fleet_order={special_project=0}} current_order={}}
	 }`)}})
	result, err := handleSpecialProjects(context.Background(), rawRequest(nil))
	if err != nil || result.IsError {
		t.Fatal("normal special project handler failed")
	}
	text := resultText(result)
	for _, want := range []string{
		"Fleet 0 (Direct fleet) current_order.collect_data_fleet_order.special_project=0 (saved reference)\n    One decoded same-country project ID candidate at country.events.special_project[0]",
		"Fleet 1 (Auto fleet) current_order.automate_fleet_order.sub_order.collect_data_fleet_order.special_project=2 (saved reference)\n    Ambiguous ID with 2 decoded same-country project candidates at saved positions [1 2]",
		"current_order.collect_data_fleet_order.special_project=99 (saved reference)\n    No decoded same-country project ID candidate",
		"Fleet 3 current_order.collect_data_fleet_order.special_project unsupported or duplicate",
		"Fleet 6 current_order unsupported or duplicate", "Fleet 9 current_order unsupported or duplicate",
		"Owned fleet entries without IDs=1; referenced fleet records unavailable=1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "foreign_record") || strings.Contains(text, "Fleet 8") || strings.Count(text, "Ambiguous fleet scope with 2 saved owned_fleets references") != 2 {
		t.Fatalf("unexpected cross-country, unrelated order, or duplicate ownership result: %s", text)
	}
	result, err = handleSpecialProjects(context.Background(), rawRequest(map[string]any{"country_id": 2}))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "Candidate scope unresolved") || strings.Contains(resultText(result), "Fleet 8") {
		t.Fatal("duplicate country-owned fleet fields formed a scoped reference")
	}
}

func TestDuplicateOwnedFleetListsRemainReadable(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	data := []byte(`player={{country=0}} country={0={events={special_project={id=0}} fleets_manager={owned_fleets={{fleet=0}} owned_fleets={{fleet=1}}}}}
	 fleet={0={current_order={collect_data_fleet_order={special_project=0}}} 1={current_order={collect_data_fleet_order={special_project=0}}}}`)
	gs, err := gamestate.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	entries := c.FleetsManager.OwnedFleets
	if len(entries) != 2 || entries[0].FleetID == nil || *entries[0].FleetID != 0 || entries[1].FleetID == nil || *entries[1].FleetID != 1 {
		t.Fatal("saved fleet entries were discarded or reordered")
	}
	text := resultText(renderSpecialProjects(gs, 0, &c))
	if !strings.Contains(text, "Candidate scope unresolved") || !strings.Contains(text, "country.events.special_project[0] (saved order)") || strings.Contains(text, "(saved reference)") {
		t.Fatalf("duplicate fleet lists lost neighboring data or formed a scoped reference: %s", text)
	}
}
