package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func countryObservationCall(t *testing.T, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := handleCountryObservationRefs(context.Background(), rawRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCountryObservationRefsOrderAndCandidates(t *testing.T) {
	const fixture = `
player={{country=0} {country=0}}
country={0={
	sensor_range_fleets={0 00 bad {key=x anonymous}}
	sensor_range_fleets={}
	sensor_range_fleets=unknown
	surveyed_deposit_holders={{type=0 id=0} {id=00 type=0} {id=0 type=1} {id=1 id=0 type=0}
		{id=2 type=unknown} {type=0} {id=1 type=0 type=1} malformed}
	surveyed_deposit_holders={field=1 anonymous}
} 1={}}
fleet={0={} 1=none}
planets={planet={0={surveyed_by=0} 1={surveyed_by=1} 2={surveyed_by=0 surveyed_by=1}}}
deposit={0={} 1=none 2={} 02={}}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	sensors := countryObservationCall(t, nil)
	if sensors.IsError {
		t.Fatal("sensor read failed")
	}
	technologyContains(t, resultText(sensors),
		"kind=sensor_range_fleets saved_entries=6 offset=0",
		"field_status=duplicate field_occurrences=3 unsupported_entries=3",
		`entry[0] path="/1/0/0/0" kind=sensor-reference id_occurrences=2`,
		`id_status=integer id_path="/1/0/0/0" id_saved="0"`,
		`candidate_table=fleet status=numeric-candidate occurrences=1 path="/2/0"`,
		`candidate_table=planet status=numeric-candidate occurrences=1 path="/3/0/0"`,
		`candidate_table=deposit status=numeric-candidate occurrences=1 path="/4/0"`,
		`candidate_table=country status=numeric-candidate occurrences=1 path="/1/0"`,
		`entry[1] path="/1/0/0/1" kind=sensor-reference id_occurrences=2`,
		`id_saved="00"`,
		`entry[2] path="/1/0/0/2" kind=unsupported-item: "bad"`,
		`entry[3] path="/1/0/0/3" kind=unsupported-item: mixed saved block`,
		`entry[4] path="/1/0/1" kind=empty-list: empty object`,
		`entry[5] path="/1/0/2" kind=unsupported-list: "unknown"`, "next_offset=none")
	if strings.Contains(resultText(sensors), "planet_candidate_surveyed_by") {
		t.Fatal("sensor candidates included an unrelated planet field")
	}
	holders := countryObservationCall(t, map[string]any{"kind": gamestate.SurveyedDepositHoldersKind})
	if holders.IsError {
		t.Fatal("holder read failed")
	}
	technologyContains(t, resultText(holders),
		"kind=surveyed_deposit_holders saved_entries=9 offset=0",
		"field_status=duplicate field_occurrences=2 unsupported_entries=6",
		`entry[0] path="/1/0/3/0" kind=holder-reference id_occurrences=3 id_type_pair_occurrences=2`,
		`id_status=integer id_path="/1/0/3/0/1" id_saved="0"`,
		`type_status=integer type_path="/1/0/3/0/0" type_saved="0"`,
		`planet_candidate_surveyed_by_status=integer planet_candidate_surveyed_by_path="/3/0/0/0" planet_candidate_surveyed_by_saved="0" planet_candidate_surveyed_by_country_match=same-number`,
		`entry[1] path="/1/0/3/1" kind=holder-reference id_occurrences=3 id_type_pair_occurrences=2`,
		`entry[2] path="/1/0/3/2" kind=holder-reference id_occurrences=3 id_type_pair_occurrences=1`,
		`entry[3] path="/1/0/3/3" kind=holder-reference id_occurrences=0`,
		`id_status=duplicate id_path="/1/0/3/3"`,
		`entry[4] path="/1/0/3/4" kind=holder-reference id_occurrences=1`,
		`type_status=unsupported type_path="/1/0/3/4/1" type_saved="unknown"`,
		`candidate_table=deposit status=duplicate occurrences=2 duplicate_paths=["/4/2" "/4/3"]`,
		`planet_candidate_surveyed_by_status=duplicate planet_candidate_surveyed_by_path="/3/0/2"`,
		`entry[5] path="/1/0/3/5" kind=holder-reference id_occurrences=0`, "id_status=not-recorded",
		`entry[6] path="/1/0/3/6" kind=holder-reference id_occurrences=1`, "type_status=duplicate",
		`candidate_table=fleet status=unsupported occurrences=1 path="/2/1"`,
		"planet_candidate_surveyed_by_country_match=different-number",
		`entry[7] path="/1/0/3/7" kind=unsupported-item: "malformed"`,
		`entry[8] path="/1/0/4" kind=unsupported-list: mixed saved block`, "next_offset=none")
}

func TestCountryObservationRefsCoverage(t *testing.T) {
	for _, fixture := range []struct{ name, save, want, extra string }{
		{"absent list", `country={0={}}`, "field_status=not-recorded field_occurrences=0", "next_offset=none"},
		{"empty list", `country={0={sensor_range_fleets={}}}`, "field_status=empty field_occurrences=1", "kind=empty-list"},
		{"duplicate root", `country={0={sensor_range_fleets={0}}} fleet={0={}} fleet={}`, `table=fleet table_status=duplicate table_path="" unindexed_keys=0 duplicate_paths=["/1" "/2"]`, "candidate_table=fleet status=table-duplicate"},
		{"duplicate nested table", `country={0={sensor_range_fleets={0}}} planets={planet={} planet={0={}}}`, `table=planet table_status=duplicate table_path="/1" unindexed_keys=0 duplicate_paths=["/1/0" "/1/1"]`, "candidate_table=planet status=table-duplicate"},
		{"unsupported table", `country={0={sensor_range_fleets={0}}} fleet={0 1}`, `table=fleet table_status=unsupported table_path="/1"`, "candidate_table=fleet status=table-unsupported"},
		{"partial table", `country={0={sensor_range_fleets={0}}} fleet={unexpected={}}`, `table=fleet table_status=partial table_path="/1" unindexed_keys=1`, "candidate_table=fleet status=unresolved"},
		{"missing table key", `country={0={sensor_range_fleets={0}}} fleet={1={}}`, "table=fleet table_status=recorded", "candidate_table=fleet status=missing"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture.save)}})
			result := countryObservationCall(t, map[string]any{"country_id": 0})
			if result.IsError {
				t.Fatal("lookup failed")
			}
			technologyContains(t, resultText(result), fixture.want, fixture.extra)
		})
	}
}

func TestCountryObservationRefsPagination(t *testing.T) {
	var sensors, holders strings.Builder
	for i := 0; i < 23; i++ {
		fmt.Fprintf(&sensors, " %d", i)
		fmt.Fprintf(&holders, " {id=%d type=0}", i)
	}
	fixture := "player={country=0} player={country=0} country={0={sensor_range_fleets={" + sensors.String() + "} surveyed_deposit_holders={" + holders.String() + "}} 1={}}"
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	s := server.NewMCPServer("test", "test")
	registerCountryObservationRefs(s)
	entry, ok := s.ListTools()["get_country_observation_refs"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only registration")
	}
	var continuation map[string]any
	for _, kind := range []string{gamestate.SensorRangeFleetsKind, gamestate.SurveyedDepositHoldersKind} {
		first := countryObservationCall(t, map[string]any{"kind": kind})
		if first.IsError {
			t.Fatal("first page failed")
		}
		got := resultText(first)
		technologyContains(t, got, "saved_entries=23 offset=0", "entry[19]", "next_offset=20")
		if strings.Contains(got, "entry[20]") {
			t.Fatal("page exceeds 20 entries")
		}
		continuation = map[string]any{"country_id": 0, "kind": kind, "offset": 20}
		for _, line := range strings.Split(got, "\n") {
			key, value, ok := strings.Cut(line, "=")
			if ok && (key == "save_id" || key == "page_context") {
				continuation[key] = value
			}
		}
		second := countryObservationCall(t, continuation)
		if second.IsError {
			t.Fatal("continuation failed")
		}
		technologyContains(t, resultText(second), "offset=20", "entry[20]", "entry[21]", "entry[22]", "next_offset=none")
	}
	for _, invalid := range []map[string]any{
		{"kind": gamestate.SensorRangeFleetsKind, "offset": 20, "page_context": continuation["page_context"]},
		{"kind": gamestate.SurveyedDepositHoldersKind, "country_id": 1, "offset": 20, "page_context": continuation["page_context"]},
		{"kind": gamestate.SurveyedDepositHoldersKind, "offset": 24, "page_context": continuation["page_context"]},
		{"offset": 20}, {"offset": 0.5}, {"save_id": 0}, {"country_id": 0.5}, {"kind": "invalid"}, {"kind": 0},
	} {
		if !countryObservationCall(t, invalid).IsError {
			t.Fatalf("accepted invalid synthetic request %v", invalid)
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(fixture + " changed=yes")}})
	if stale := countryObservationCall(t, continuation); !stale.IsError || !strings.Contains(resultText(stale), "page_context") {
		t.Fatal("changed save revision accepted")
	}
}
