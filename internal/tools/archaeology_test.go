package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestArchaeologyScopeCandidatesAndRawHistory(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Synthetic nation"}}} planets={planet={0={name={key="Candidate planet"}}}} fleet={0={name={key="Candidate fleet"}}}
 archaeological_sites={sites={
  0={type="synthetic_site" visible_to={0} location={type=0 id=0} excavator_fleet=0 index=0 days_left=0 clues=0 last_roll=0 difficulty=0 locked=no
    completed={{country=0 date="2201.01.01"} {country=0 date="2201.01.02"}}
    events={{event_id="synthetic_event" expired=0 owner=0 fleet=0 index=0 player_event=0 scope={opaque_field=0}}}
    log={{title="first" roll=0 bonus=0 total=0 clues=0 difficulty=0} {title="second"}}}
  1={last_excavator_country=0 excavator_fleet=999}
  2={completed={{country=0}}}
  3={visible_to={1}}
 }}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	summary := resultText(renderArchaeology(gs, 0, &c, nil))
	for _, want := range []string{"Site 0:", "Site 1:", "Site 2:", "Possible planet ID match: Candidate planet", "Possible fleet ID match: Candidate fleet", "no reverse site link validated", "Excavator fleet reference unresolved"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in %s", want, summary)
		}
	}
	if strings.Contains(summary, "Site 3:") {
		t.Fatal("unreferenced country site included")
	}
	id := 0
	detail := resultText(renderArchaeology(gs, 0, &c, &id))
	for _, want := range []string{"index=0 days_left=0", "clues=0 last_roll=0 difficulty=0", "locked: false", "global site completion not inferred", "completed entry 0", "completed entry 1", "event entry 0: event_id=synthetic_event expired=0 owner=0 fleet=0 index=0 player_event=0", "scope: preserved opaque object", "title=first", "title=second"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("missing %q in %s", want, detail)
		}
	}
	if strings.Index(detail, "title=first") > strings.Index(detail, "title=second") || strings.Contains(detail, "%") || strings.Contains(detail, "Site completed: true") {
		t.Fatal("raw history or unverified meaning changed")
	}
	id = 3
	if !renderArchaeology(gs, 0, &c, &id).IsError {
		t.Fatal("unreferenced country site accepted")
	}
}

func TestArchaeologyMissingValuesAndUnsupportedWarning(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`archaeological_sites={sites={0={last_excavator_country=0} 1={days_left={unexpected=yes}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	id := 0
	text := resultText(renderArchaeology(gs, 0, &gamestate.Country{}, &id))
	for _, want := range []string{"Archaeology decoding incomplete", "raw tree/byte tools", "index=not recorded days_left=not recorded", "type=not recorded id=not recorded", "locked: not recorded", "completed entries: not recorded", "events: not recorded", "log: not recorded"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}

func TestArchaeologyChecksReciprocalCurrentOrder(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`fleet={
	 0={name={key="Matching fleet"} current_order={excavate_archaeological_site_fleet_order={archaeological_site=0}}}
	 1={name={key="Different fleet"} current_order={excavate_archaeological_site_fleet_order={archaeological_site=2}}}
	 2={name={key="Duplicate site fleet"} current_order={excavate_archaeological_site_fleet_order={archaeological_site=2}}}
	 4={name={key="Duplicate order fleet"} current_order={excavate_archaeological_site_fleet_order={archaeological_site=4 archaeological_site=1}}}
	 } archaeological_sites={sites={
	 0={visible_to={0} excavator_fleet=0}
	 1={visible_to={0} excavator_fleet=1}
	 2=none 2={visible_to={0} excavator_fleet=2}
	 3={visible_to={0} excavator_fleet=0 excavator_fleet=1}
	 4={visible_to={0} excavator_fleet=4}
	 }}`))
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(renderArchaeology(gs, 0, &gamestate.Country{}, nil))
	for _, want := range []string{
		"Matching fleet; current_order.excavate_archaeological_site_fleet_order.archaeological_site=0 (reciprocal saved IDs agree)",
		"Different fleet; current_order.excavate_archaeological_site_fleet_order.archaeological_site=2 (saved ID differs)",
		"Duplicate site fleet; site or fleet order unsupported or duplicate",
		"Duplicate order fleet; site or fleet order unsupported or duplicate",
		"Duplicate excavator_fleet fields. Fleet candidate unresolved.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Count(text, "reciprocal saved IDs agree") != 1 {
		t.Fatal("ambiguous or mismatching order formed a reciprocal site link")
	}
}

func TestArchaeologyReportsOneWayFleetOrders(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	data := `country={0={fleets_manager={owned_fleets={{fleet=0} {fleet=1} {fleet=2} {fleet=3} {fleet=4}}}} 1={fleets_manager={owned_fleets={{fleet=9} {fleet=10}}}}}
	 fleet={
	 0={name={key="Visible site order"} current_order={excavate_archaeological_site_fleet_order={archaeological_site=0}}}
	 1={name={key="Missing site order"} current_order={excavate_archaeological_site_fleet_order={archaeological_site=1}}}
	 2={name={key="Unscoped site order"} current_order={excavate_archaeological_site_fleet_order={archaeological_site=2}}}
	 3={current_order={excavate_archaeological_site_fleet_order={archaeological_site=0 archaeological_site=2}}}
	 9={name={key="Foreign order"} current_order={excavate_archaeological_site_fleet_order={archaeological_site=1}}}
	 }`
	for _, tc := range []struct {
		table string
		want  []string
	}{
		{"", []string{"archaeological_sites.sites table not recorded", "Fleet 0 (Visible site order)", "Site record unavailable"}},
		{` archaeological_sites={sites={0={visible_to={0}} 2={visible_to={1}}}}`, []string{
			"Fleet 0 (Visible site order) current_order.excavate_archaeological_site_fleet_order.archaeological_site=0 (saved reference)\n    Site excavator_fleet not recorded",
			"Fleet 1 (Missing site order) current_order.excavate_archaeological_site_fleet_order.archaeological_site=1 (saved reference)\n    Site record unavailable",
			"Fleet 2 (Unscoped site order) current_order.excavate_archaeological_site_fleet_order.archaeological_site=2 (saved reference)\n    Site has no matching saved country references",
			"Fleet 3 current_order.excavate_archaeological_site_fleet_order.archaeological_site unsupported or duplicate",
		}},
	} {
		gs, err := gamestate.Load([]byte(data + tc.table))
		if err != nil {
			t.Fatal(err)
		}
		c := gs.Country[0]
		text := resultText(renderArchaeology(gs, 0, &c, nil))
		for _, want := range tc.want {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in %s", want, text)
			}
		}
		if !strings.Contains(text, "Fleet 4 referenced by owned_fleets has no decoded fleet record") || strings.Contains(text, "Fleet 10") {
			t.Fatal("unavailable fleet record was omitted or reported outside the selected country")
		}
		if strings.Contains(text, "Foreign order") || strings.Contains(text, "reciprocal saved IDs agree") {
			t.Fatal("foreign order leaked or a one-way order was treated as reciprocal")
		}
		siteID := 0
		detail := renderArchaeology(gs, 0, &c, &siteID)
		if tc.table == "" {
			if !detail.IsError || !strings.Contains(resultText(detail), "archaeological_sites.sites table not recorded") {
				t.Fatal("missing table was not reported for a specific site request")
			}
		} else if detail.IsError || !strings.Contains(resultText(detail), "Site excavator_fleet not recorded") || strings.Contains(resultText(detail), "Fleet 1") || strings.Contains(resultText(detail), "Fleet 4") {
			t.Fatal("specific site request did not keep the one-way order scoped to that site")
		}
	}
}
