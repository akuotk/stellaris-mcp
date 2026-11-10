package tools

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestColonyPopulationRecordsGrowthAndBackReferences(t *testing.T) {
	const source = `player={{country=0} {country=0}} country={0={owned_planets={0}}}
colony={0={carrier={type=planet reference=8} current_month_growth_data={x={growth=01 growth=2 anon}}
 last_month_growth_data={} pop_groups={0 0 1 2 3 bad} pop_jobs={5 6}}}
planets={planet={8={colony=0 owner=0} 0={owner=0 migration=9}}}
pop_groups={0={planet=0 size=0 last_month_growth_details={growth=2 growth=3}} 1={planet=8}
 2=none 3={planet=0} 03={planet=0} 7={planet=0 current_month_growth_details={growth=0}} 4={planet=0 planet=8} 8={planet=9}}
pop_jobs={5={planet=0 max_workforce=-1 workforce_limit=0 bonus_workforce=1 automated_workforce=2}
 6={type=sample} 10={planet=0}}`
	syntheticSaveDir(t, []syntheticEntry{{"meta", []byte("sample=yes")}, {"gamestate", []byte(source)}})
	growth := rawCall(t, handleColonyPopulationRecords, map[string]any{"location_id": 0})
	if growth["country_id"] != float64(0) || growth["entry_index"] != float64(1) || growth["schema"] != "colony" ||
		growth["planet_link"] != "reciprocal" || growth["planet"].(map[string]any)["path"] != "/3/0/0" ||
		growth["owned_reference_paths_preview"].([]any)[0] != "/1/0/0/0" {
		t.Fatal("wrong colony selection or reciprocal planet identity")
	}
	rows := growth["rows"].([]any)
	for i, suffix := range []string{"1", "1/0", "1/0/0", "1/0/1", "1/0/2", "2"} {
		if rows[i].(map[string]any)["path"] != "/2/0/"+suffix {
			t.Fatalf("growth order lost at row %d", i)
		}
	}
	if len(rows) != 6 || rows[1].(map[string]any)["kind"] != "mixed" || rows[5].(map[string]any)["kind"] != "empty_block" {
		t.Fatal("mixed, duplicate or empty growth entries lost")
	}
	field := rows[2].(map[string]any)
	identity := map[string]any{"save_id": growth["save_id"], "revision": growth["revision"], "entry_index": growth["entry_index"], "path": field["path"]}
	tree := rawCall(t, handleReadSaveTree, identity)
	if tree["node"].(map[string]any)["scalar_preview"] != "01" {
		t.Fatal("growth raw path did not resolve")
	}
	delete(identity, "path")
	identity["offset"], identity["limit"] = field["entry_source_offset"], field["entry_source_bytes"]
	bytes := rawCall(t, handleReadSaveBytes, identity)
	if bytes["data"] != base64.StdEncoding.EncodeToString([]byte("growth=01")) {
		t.Fatal("growth source range did not preserve original bytes")
	}

	groups := rawCall(t, handleColonyPopulationRecords, map[string]any{"location_id": 0, "view": "pop_groups"})
	list := groups["rows"].([]any)
	if len(list) != 8 || groups["population_list"].(map[string]any)["status"] != "partial" ||
		groups["view_status"] != "partial" {
		t.Fatal("forward list or reverse-only candidates lost")
	}
	for i, want := range []struct{ role, state, backref string }{
		{"listed-reference", "numeric-candidate", "same-number"},
		{"listed-reference", "numeric-candidate", "same-number"},
		{"listed-reference", "numeric-candidate", "different-number"},
		{"listed-reference", "none", "unresolved"},
		{"listed-reference", "duplicate", "unresolved"},
		{"unsupported-reference", "", ""},
		{"reverse-only-candidate", "numeric-candidate", "same-number"},
		{"reverse-only-candidate", "numeric-candidate", "duplicate"},
	} {
		row := list[i].(map[string]any)
		if row["role"] != want.role {
			t.Fatalf("wrong reference role at row %d", i)
		}
		if want.state != "" && (row["reference"].(map[string]any)["status"] != want.state || row["record_planet_numeric_match"] != want.backref) {
			t.Fatalf("wrong candidate or back-reference state at row %d", i)
		}
	}
	if list[6].(map[string]any)["path"] != "/4/5" || list[7].(map[string]any)["path"] != "/4/6" {
		t.Fatal("reverse-only candidates lost root table order")
	}
	if list[0].(map[string]any)["list_reference_occurrences"] != float64(2) ||
		list[6].(map[string]any)["list_reference_occurrences"] != float64(0) {
		t.Fatal("repeated list occurrence or reverse-only scope lost")
	}
	for _, tc := range []struct {
		view, status string
		id, rows     int
	}{
		{"group_fields", "partial", 0, 5},
		{"group_fields", "record-duplicate", 3, 2},
		{"group_fields", "partial", 7, 3},
		{"group_fields", "record-not-observed-for-location", 8, 0},
		{"job_fields", "recorded", 5, 5},
	} {
		detail := rawCall(t, handleColonyPopulationRecords, map[string]any{"location_id": 0, "view": tc.view, "record_id": tc.id})
		if detail["view_status"] != tc.status || len(detail["rows"].([]any)) != tc.rows {
			t.Fatalf("wrong detail availability for %s", tc.view)
		}
		if tc.status == "record-duplicate" &&
			(detail["rows"].([]any)[1].(map[string]any)["path"] != "/4/4" || detail["record_planet_numeric_match"] != "unresolved") {
			t.Fatal("duplicate record paths were lost")
		}
		if tc.view == "group_fields" && tc.id == 7 &&
			(detail["record_list_occurrences"] != float64(0) || detail["record_reverse_observed"] != true) {
			t.Fatal("reverse-only details did not retain their scope")
		}
	}
	jobs := rawCall(t, handleColonyPopulationRecords, map[string]any{"location_id": 0, "view": "pop_jobs"})
	jobRows := jobs["rows"].([]any)
	if len(jobRows) != 3 || jobRows[1].(map[string]any)["record_planet_numeric_match"] != "not-recorded" ||
		jobRows[2].(map[string]any)["role"] != "reverse-only-candidate" {
		t.Fatal("job mismatch or reverse-only candidate lost")
	}
}

func TestColonyPopulationRecordsLegacyAndUnresolved(t *testing.T) {
	const planet = `planets={planet={0={owner=0 migration=-01 growth=0 growth_species_ref=0 pop_assembly=2 pop={0 1} jobs_cache={{synthetic=yes}}} 1={owner=0 colony=9}}}`
	const country = `player={country=0} country={0={owned_planets={0 1}}} `
	for _, tc := range []struct {
		name, extra, schema, status, link string
		id, count                         int
	}{
		{"legacy", "", "legacy-planet", "recorded", "owner-same-number", 0, 10},
		{"missing colony table for linked planet", "", "unresolved", "colony-reference-without-colony-table", "not-checked", 1, 1},
		{"present empty colony table", " colony={}", "colony", "location-missing", "not-checked", 0, 0},
		{"duplicate colony table", " colony={} colony={}", "unresolved", "location-table-duplicate", "not-checked", 0, 0},
		{"duplicate colony key", " colony={0={carrier={type=planet reference=0}} 00={}}", "colony", "location-duplicate", "not-checked", 0, 2},
		{"nonplanet carrier", " colony={0={carrier={type=ship reference=0}}}", "colony", "growth-fields-not-recorded", "nonplanet-carrier", 0, 0},
		{"different reverse link", " colony={0={carrier={type=planet reference=1}}}", "colony", "growth-fields-not-recorded", "different-number", 0, 0},
		{"not owned", " colony={2={current_month_growth_data={secret=synthetic}}}", "colony", "location-not-observed-in-owned-planets", "not-checked", 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(country + planet + tc.extra)}})
			result := rawCall(t, handleColonyPopulationRecords, map[string]any{"location_id": tc.id})
			if result["schema"] != tc.schema || result["view_status"] != tc.status ||
				result["planet_link"] != tc.link || len(result["rows"].([]any)) != tc.count {
				t.Fatal("wrong schema or scope status")
			}
		})
	}
}

func TestColonyPopulationRecordsPagination(t *testing.T) {
	group := "{planet=0 " + strings.Repeat("max_workforce=-1 ", 23) + "}"
	colony := "{carrier={type=ship reference=0} pop_groups={0 1} pop_jobs={0}}"
	source := "player={{country=0} {country=0}} country={0={owned_planets={0 1}} 1={owned_planets={0 1}}}" +
		" colony={0=" + colony + " 1=" + colony + "} pop_groups={0=" + group + " 1=" + group + "} pop_jobs={0=" + group + "}"
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	s := server.NewMCPServer("test", "test")
	registerColonyPopulationRecords(s)
	tool := s.ListTools()["get_colony_population_records"].Tool
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint ||
		tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only annotations")
	}
	first := rawCall(t, handleColonyPopulationRecords, map[string]any{"location_id": 0, "view": "group_fields", "record_id": 0})
	if len(first["rows"].([]any)) != 20 || first["row_count"] != float64(24) || first["next_offset"] != float64(20) {
		t.Fatal("large saved record did not paginate its fields")
	}
	args := map[string]any{"location_id": 0, "view": "group_fields", "record_id": 0, "country_id": 0,
		"save_id": first["save_id"], "page_context": first["page_context"], "offset": 20}
	second := rawCall(t, handleColonyPopulationRecords, args)
	if rows := second["rows"].([]any); len(rows) != 4 || second["next_offset"] != nil || rows[0].(map[string]any)["path"] != "/3/0/20" {
		t.Fatal("continuation lost its saved raw position")
	}
	for key, bad := range map[string]any{"location_id": 1, "view": "job_fields", "record_id": 1, "country_id": 1, "offset": 25, "page_context": ""} {
		saved := args[key]
		args[key] = bad
		expectColonyPopulationRecordsError(t, args)
		args[key] = saved
	}
	for _, bad := range []map[string]any{
		nil, {"location_id": 0.5}, {"location_id": -1}, {"location_id": 0, "view": "unknown"},
		{"location_id": 0, "view": "group_fields"}, {"location_id": 0, "record_id": 0},
	} {
		expectColonyPopulationRecordsError(t, bad)
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(source + " ")}})
	expectColonyPopulationRecordsError(t, args)
}

func expectColonyPopulationRecordsError(t *testing.T, args map[string]any) {
	t.Helper()
	result, err := handleColonyPopulationRecords(context.Background(), rawRequest(args))
	if err != nil || !result.IsError {
		t.Fatal("invalid input accepted")
	}
}
