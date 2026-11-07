package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestEmpireAssetsFiltersAndResolvesSystem(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Home"}} 2={adjective={key="Other"}}}
	galactic_object={7={name={key="Test System"} planet=9}}
	agreements={agreements={1={owner=0 target=2 active_status=active date_changed="2201.02.03" term_data={can_subject_vote=no joins_overlord_wars=defensive discrete_terms={{key=term_a value=option_a} {key=term_a value=option_b}} resource_terms={{key=resource_example value=-0.25}}} subject_specialization={level=2 experience=3.5 subject_conversion_process={progress=5 in_progress=yes done=no}}} 2={owner=2 target=3} 3={target=0} 4={owner=0 target=99}}}
	situations={situations={4={country=0 type="situation_test" progress=12.123456} 6={country=0 type="situation_zero" progress=0 last_month_progress=0} 5={country=2 type="other"}}}
	megastructures={6={owner=0 type="gateway" coordinate={origin=7}} 8={type="unowned"}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderEmpireAssets(gs, 0, &c))
	for _, want := range []string{"owner Home (0), target Other (2)", "progress=12.123456, last month=not recorded", "progress=0, last month=0", "possible system ID match Test System (system ID 7; reverse link unverified)", "ID 3: owner not recorded, target Home (0)", "target 99 (country record unavailable)", "date_changed=2201.02.03", "can_subject_vote=no", "joins_overlord_wars=defensive", "discrete_term term_a=option_a", "discrete_term term_a=option_b", "resource_term resource_example=-0.25", "specialization_level=2", "conversion_in_progress=yes"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "can_subject_be_integrated=") {
		t.Fatal("absent term presented as a default")
	}
	for _, bad := range []string{"ID 2:", "ID 5:", "ID 8:"} {
		if strings.Contains(text, bad) {
			t.Fatalf("unexpected %q: %s", bad, text)
		}
	}
}

func TestEmpireAssetsReportsSavedSubjectLinksAndAgreementEvidence(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	data := []byte(`player={{country=0}}
	country={0={adjective={key=Home} subjects={2 3 3 4 99}}
	2={adjective={key=Subject} overlord=0 subject_date=2201.02.03 subjects={}}
	3={overlord=8}
	4={overlord=0 overlord=7 subject_date=""}
	5={subjects={} subjects={2} subject_date=2201.01.01 subject_date=2202.01.01}}
	agreements={agreements={
	1={owner=0 target=2 term_data={forced_initial_loyalty=-0.000 has_cooldown_on_first_renegotiation=no} subject_specialization={subject_conversion_process={ignore=yes}}}
	2={owner=0 target=3}
	3={owner=0 target=4}
	4={owner=0 target=99}
	5={owner=0 target=5}
	6={owner=0 owner=7 target=2}
	7=none}}`)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", data}})
	gs, err := gamestate.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	result, err := handleEmpireAssets(context.Background(), mcp.CallToolRequest{})
	if err != nil || result.IsError {
		t.Fatalf("get_empire_assets returned an error: %v", err)
	}
	text := resultText(result)
	for _, want := range []string{
		"Agreement table has unsupported or duplicate data",
		"owner country.subjects=validated; target country.overlord=matches owner (0)",
		"owner country.subjects=2 duplicate references; target country.overlord=saved 8 differs from owner",
		"owner country.subjects=validated; target country.overlord=unsupported or duplicate",
		"owner country.subjects=validated; target country.overlord=target country record unavailable",
		"owner country.subjects=missing reverse reference; target country.overlord=not recorded",
		"forced_initial_loyalty=-0.000 (raw save value)",
		"has_cooldown_on_first_renegotiation=no (raw save value)",
		"subject_conversion_process.ignore=yes (raw save value)",
		"country.overlord=not recorded",
		"country.subject_date=not recorded",
		"country.subjects (saved order):\n    Subject (2)",
		"99 (country record unavailable)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in synthetic output: %s", want, text)
		}
	}
	for _, absent := range []string{"ID 6:", "ID 7:"} {
		if strings.Contains(text, absent) {
			t.Errorf("unexpected agreement %q in synthetic output", absent)
		}
	}
	for _, tc := range []struct {
		id   int
		want []string
	}{
		{2, []string{"country.overlord=Home (0)", `country.subject_date="2201.02.03"`, "country.subjects=empty"}},
		{4, []string{"country.overlord=unsupported or duplicate", `country.subject_date=""`, "country.subjects=not recorded"}},
		{5, []string{"country.subjects=unsupported or duplicate", "country.subject_date=unsupported or duplicate"}},
	} {
		c := gs.Country[tc.id]
		text := resultText(renderEmpireAssets(gs, tc.id, &c))
		for _, want := range tc.want {
			if !strings.Contains(text, want) {
				t.Errorf("missing %q for synthetic country %d: %s", want, tc.id, text)
			}
		}
	}
}
