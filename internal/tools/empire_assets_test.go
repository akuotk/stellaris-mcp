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

func TestEmpireAssetsSituationScopeStagesAndTypedPlanetTargets(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	data := []byte(`player={{country=0}}
	country={0={adjective={key=Home} track_all_situations=no events={situations={5 2 1 0 0 4 7 9}}}
	2={adjective={key=Foreign} events={situations={2 0 0}}}}
	planets={planet={0={name={key="Target Body"}}}}
	situations={situations={
	0={country=0 type=demo progress=00.1200 last_month_progress=-0.00 approach=hold
	target={type=planet id=0 opener_id=4294967295}
	stage=first stage_durations={{key=first value=0.00} {key=first value=1.00}}
	stage_flags={first_flag first_flag} flags={flag_name={flag_date=2201.01.01 flag_days=0}}}
	1={country=0 type=demo target={type=none id=0 opener_id=4294967295}}
	2={country=2 target={type=planet type=none id=0}}
	3={country=0 target={type=planet}}
	4=none
	5={country=0} 5=none
	6={country=0 target={type=planet id=99}}
	7={type=orphan}
	8={country=2 type=hidden}
	9={country=0 target={type=planet id=0} target={type=none id=0}}
	}}`)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", data}})
	result, err := handleEmpireAssets(context.Background(), mcp.CallToolRequest{})
	if err != nil || result.IsError {
		t.Fatalf("get_empire_assets returned an error: %v", err)
	}
	text := resultText(result)
	for _, want := range []string{
		`country.track_all_situations="no" (saved field)`,
		"Situation table has unsupported or duplicate data",
		"country.events.situations (saved order)=[5 2 1 0 0 4 7 9]",
		"progress=00.1200, last month=-0.00, approach=hold",
		"situation.country=Home (0); country.events.situations membership=2 duplicate references",
		"Other countries' saved situation references (country IDs, including repeated entries)=[2 2]",
		`stage: "first" (saved scalar)`,
		"stage_durations: list with 2 saved entries",
		`Field 1 key="value": "1.00" (saved scalar)`,
		"stage_flags: list with 2 saved entries",
		`Item 1: "first_flag" (saved scalar)`,
		`Field 0 key="flag_date": "2201.01.01" (saved scalar)`,
		`Field 2 key="opener_id": "4294967295" (saved scalar)`,
		"Saved planet target 0: Target Body",
		"Saved planet target 99: planet record unavailable.",
		"Saved situation.country differs from selected country.",
		"Situation target unsupported or duplicate",
		"Planet target ID not recorded.",
		"ID 4: referenced record unavailable",
		"ID 5: referenced record unsupported or duplicate",
		"situation.country=not recorded; country.events.situations membership=validated",
		"target [duplicate saved key]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in synthetic output: %s", want, text)
		}
	}
	if strings.Contains(text, "ID 8:") || strings.Count(text, "Saved planet target 0:") != 1 {
		t.Fatal("unreferenced foreign situation or ambiguous target incorrectly resolved")
	}
}

func TestEmpireAssetsMegastructureReferencesAndRawDetails(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	data := []byte(`player={{country=0}}
	country={0={adjective={key=Home} owned_megastructures={0 1 1 2 4 6}} 2={adjective={key=Foreign}}}
	galactic_object={0={name={key="Origin System"} megastructures={0 1 1} planet=0}
	1={name={key="Other System"} megastructures={1 3}}
	2={megastructures={3} megastructures={3}}}
	planets={planet={0={name={key="Orbit Body"}}}}
	bypasses={0={type=gate owner={type=unknown id=0}} 2={type=gate owner={id=99}}}
	megastructures={
	0={owner=0 type=gate coordinate={origin=0 x=-0.0000 y=0.00 randomized=no visual_height=5.123456} planet=0 bypass=0
	build_queue=4294967295 dismantle_progress=00.125 dismantle_finish_date=2201.01.01 graphical_culture=style
	upgrade={upgrade_to=gate_2 progress=-0.000 halted=no indefinitely_halted=yes}}
	1={owner=2 type=habitat coordinate={origin=0} planet=4294967295 bypass=2}
	2={type=unknown coordinate={origin=99} bypass=9}
	3={owner=0 type=unknown}
	4={owner=0} 4=none
	5={owner=2 type=hidden}
	6=none}`)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", data}})
	result, err := handleEmpireAssets(context.Background(), mcp.CallToolRequest{})
	if err != nil || result.IsError {
		t.Fatalf("get_empire_assets returned an error: %v", err)
	}
	text := resultText(result)
	for _, want := range []string{
		"Megastructure table has unsupported or duplicate data",
		"System reverse-reference survey incomplete",
		"ID 0: Gate [gate], Origin System (system ID 0; coordinate.origin and galactic_object.megastructures membership=validated)",
		"owner=Home (0); country.owned_megastructures membership=validated",
		`coordinate.x="-0.0000" coordinate.y="0.00" coordinate.randomized="no" coordinate.visual_height="5.123456"`,
		`dismantle_progress="00.125" dismantle_finish_date="2201.01.01" graphical_culture="style"`,
		`upgrade.upgrade_to="gate_2" upgrade.progress="-0.000" upgrade.halted="no" upgrade.indefinitely_halted="yes"`,
		"planet=0 (planet record Orbit Body); coordinate.origin system planet membership=validated",
		`bypass=0; bypass.type="gate"; bypass.owner.type="unknown" bypass.owner.id=0 (owner namespace unverified)`,
		"build_queue=4294967295 (raw save value; queue reference unverified)",
		byIntWidth("owner=Foreign (2); country.owned_megastructures membership=2 duplicate references", "ID 1: referenced record unsupported or duplicate"),
		byIntWidth("Saved owner differs from selected country.", "ID 1: referenced record unsupported or duplicate"),
		byIntWidth("galactic_object.megastructures reverse system IDs=[0 0 1]", "ID 1: referenced record unsupported or duplicate"),
		byIntWidth("Duplicate system membership entries recorded.", "ID 1: referenced record unsupported or duplicate"),
		byIntWidth("Saved system references disagree or span multiple systems.", "ID 1: referenced record unsupported or duplicate"),
		byIntWidth("planet=4294967295 (planet record unavailable; raw value)", "ID 1: referenced record unsupported or duplicate"),
		byIntWidth("saved owner ID differs from megastructure ID", "ID 1: referenced record unsupported or duplicate"),
		"coordinate.origin=99 (system record unavailable)",
		"bypass=9 (bypass record unavailable)",
		"ID 3: Unknown [unknown], coordinate.origin not recorded",
		"ID 4: referenced record unsupported or duplicate",
		"ID 6: referenced record unavailable",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in synthetic output: %s", want, text)
		}
	}
	if strings.Contains(text, "ID 5:") {
		t.Fatal("foreign megastructure without a saved reference entered selected country scope")
	}
}
