package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func controlledPlanetCall(t *testing.T, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := handleControlledPlanets(context.Background(), rawRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestControlledPlanetsSavedOrderAndLinks(t *testing.T) {
	const fixture = `
player={{country=0} {country=0}}
country={0={controlled_planets={00 1 2 3 4 0 bad {key=x anonymous}} controlled_planets={4} controlled_planets=unknown}}
planets={planet={
	0={controller=0 owner=1 colony=10}
	1={controller=1 owner=0 colony=11}
	2=none
	3={}
	03={controller=0}
	5={controller=0 colony=12}
	6={controller=0 controller=1 owner={unexpected=yes}}
	7={owner=0}
	invalid_key={controller=0}
}}
colony={
	10={carrier={type=planet reference=0}}
	11={carrier={type=fleet reference=1}}
	12={carrier={type=planet reference=1}}
}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	listed := controlledPlanetCall(t, nil)
	if listed.IsError {
		t.Fatal("listed read failed")
	}
	technologyContains(t, resultText(listed),
		"view=listed saved_entries=10 offset=0",
		"controlled_planets_status=duplicate list_occurrences=3 unsupported_list_entries=3",
		`planet_table_status=partial planet_table_path="/2/0" planet_unindexed_keys=1 colony_table_status=recorded`,
		"reverse_coverage=partial controller_match_records=4 unresolved_planet_records=5 unlisted_candidate_records=3",
		`entry[0] path="/1/0/0/0" kind=planet-reference planet_id=0 saved_reference="00" list_membership=listed list_occurrences=2 planet_status=unique planet_candidates=1 planet_path="/2/0/0"`,
		`controller_status=integer controller_path="/2/0/0/0" controller_saved="0" controller_country_match=same-number`,
		`owner_status=integer owner_path="/2/0/0/1" owner_saved="1" owner_country_match=different-number`,
		`colony_status=integer colony_path="/2/0/0/2" colony_saved="10"`,
		`colony_link=reciprocal colony_path="/3/0" carrier_type="planet"`,
		`carrier_reference_status=integer carrier_reference_path="/3/0/0/1" carrier_reference_saved="0"`,
		`entry[1] path="/1/0/0/1" kind=planet-reference planet_id=1`,
		"controller_country_match=different-number", "owner_country_match=same-number",
		`colony_link=other-carrier-type colony_path="/3/1" carrier_type="fleet"`,
		`entry[2] path="/1/0/0/2" kind=planet-reference planet_id=2`, "planet_status=unsupported",
		`entry[3] path="/1/0/0/3" kind=planet-reference planet_id=3`,
		`planet_status=duplicate planet_candidates=2 planet_candidate_paths=["/2/0/3" "/2/0/4"]`,
		`entry[4] path="/1/0/0/4" kind=planet-reference planet_id=4`, "planet_status=unresolved",
		`entry[5] path="/1/0/0/5" kind=planet-reference planet_id=0`,
		`entry[6] path="/1/0/0/6" kind=unsupported-reference: "bad"`,
		`entry[7] path="/1/0/0/7" kind=unsupported-reference: mixed saved block`,
		`entry[8] path="/1/0/1/0" kind=planet-reference planet_id=4`,
		`entry[9] path="/1/0/2" kind=unsupported-list: "unknown"`, "next_offset=none")

	unlisted := controlledPlanetCall(t, map[string]any{"view": "unlisted"})
	if unlisted.IsError {
		t.Fatal("reverse read failed")
	}
	technologyContains(t, resultText(unlisted),
		"view=unlisted saved_entries=3 offset=0",
		`entry[0] path="/2/0/5" kind=controller-record planet_id=5 list_membership=not-observed planet_status=unique`,
		"owner_status=not-recorded",
		`colony_link=carrier-reference-differs colony_path="/3/2"`,
		`entry[1] path="/2/0/6" kind=controller-record planet_id=6 list_membership=not-observed`,
		`controller_status=duplicate controller_path="/2/0/6"`,
		`owner_status=unsupported owner_path="/2/0/6/2"`,
		`entry[2] path="/2/0/8" kind=controller-record list_membership=unresolved planet_status=unsupported-key`,
		"next_offset=none")
}

func TestControlledPlanetsCoverageStates(t *testing.T) {
	for _, fixture := range []struct{ name, save, view, want, extra string }{
		{"missing country list", `country={0={}} planets={planet={0={controller=0}}}`, "unlisted", "controlled_planets_status=not-recorded", "planet_id=0 list_membership=not-observed"},
		{"empty country list", `country={0={controlled_planets={}}} planets={planet={0={controller=0}}}`, "unlisted", "controlled_planets_status=empty", "planet_id=0 list_membership=outside-list"},
		{"missing planet record", `country={0={controlled_planets={0}}} planets={planet={1={}}}`, "listed", "planet_table_status=recorded", "planet_status=missing"},
		{"missing planet table", `country={0={controlled_planets={0}}}`, "listed", "planet_table_status=not-recorded", "planet_status=table-not-recorded"},
		{"duplicate planet wrapper", `country={0={controlled_planets={0}}} planets={planet={0={controller=0}} planet={}}`, "listed", `planet_table_status=duplicate planet_table_path="/1"`, "planet_status=table-duplicate"},
		{"unsupported planet wrapper", `country={0={controlled_planets={0}}} planets={planet={0 1}}`, "listed", `planet_table_status=unsupported planet_table_path="/1/0"`, "planet_status=table-unsupported"},
		{"duplicate unlisted planet", `country={0={controlled_planets={}}} planets={planet={0={controller=0 colony=4} 00={controller=0}}} colony={4={carrier={type=planet reference=0}}}`, "unlisted", "unlisted_candidate_records=2", "colony_link=planet-identity-unresolved"},
		{"duplicate colony records", `country={0={controlled_planets={0}}} planets={planet={0={colony=4}}} colony={4={} 04={}}`, "listed", "planet_status=unique", `colony_link=duplicate colony_candidate_paths=["/2/0" "/2/1"]`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture.save)}})
			result := controlledPlanetCall(t, map[string]any{"country_id": 0, "view": fixture.view})
			if result.IsError {
				t.Fatal("read failed")
			}
			technologyContains(t, resultText(result), fixture.want, fixture.extra)
		})
	}
}

func TestControlledPlanetsPagination(t *testing.T) {
	var list, planets strings.Builder
	for i := 0; i < 46; i++ {
		if i < 23 {
			fmt.Fprintf(&list, " %d", i)
		}
		fmt.Fprintf(&planets, " %d={controller=0}", i)
	}
	fixture := "player={country=0} player={country=0} country={0={controlled_planets={" + list.String() + "}} 1={}} planets={planet={" + planets.String() + "}}"
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	s := server.NewMCPServer("test", "test")
	registerControlledPlanets(s)
	entry, ok := s.ListTools()["get_controlled_planets"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only registration")
	}
	var continuation map[string]any
	for _, view := range []string{"listed", "unlisted"} {
		first := controlledPlanetCall(t, map[string]any{"view": view})
		if first.IsError {
			t.Fatal("first page failed")
		}
		got := resultText(first)
		technologyContains(t, got, "saved_entries=23 offset=0", "entry[19]", "next_offset=20")
		if strings.Contains(got, "entry[20]") {
			t.Fatal("page exceeds 20 entries")
		}
		continuation = map[string]any{"country_id": 0, "view": view, "offset": 20}
		for _, line := range strings.Split(got, "\n") {
			key, value, ok := strings.Cut(line, "=")
			if ok && (key == "save_id" || key == "page_context") {
				continuation[key] = value
			}
		}
		second := controlledPlanetCall(t, continuation)
		if second.IsError {
			t.Fatal("continuation failed")
		}
		technologyContains(t, resultText(second), "offset=20", "entry[20]", "entry[21]", "entry[22]", "next_offset=none")
	}
	for _, invalid := range []map[string]any{
		{"view": "listed", "offset": 20, "page_context": continuation["page_context"]},
		{"view": "unlisted", "country_id": 1, "offset": 20, "page_context": continuation["page_context"]},
		{"view": "unlisted", "offset": 24, "page_context": continuation["page_context"]},
		{"offset": 20}, {"offset": 0.5}, {"save_id": 0}, {"country_id": 0.5}, {"view": "invalid"}, {"view": 0},
	} {
		if !controlledPlanetCall(t, invalid).IsError {
			t.Fatalf("accepted invalid synthetic request %v", invalid)
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(fixture + " changed=yes")}})
	if stale := controlledPlanetCall(t, continuation); !stale.IsError || !strings.Contains(resultText(stale), "page_context") {
		t.Fatal("changed save revision accepted")
	}
}
