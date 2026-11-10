package tools

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestFleetOrdersScopeAndRawReferences(t *testing.T) {
	const source = `player={{country=0} {country=0}} country={
0={fleets_manager={owned_fleets={{fleet=0} {fleet=00} {fleet=1} {fleet=2} {fleet=3} {fleet=4 fleet=5} {} bad owned={fleet=8}}}
 events={special_project={id=0} special_project={id=1} special_project={id=1} special_project={id=2 id=3}}}
1={fleets_manager={owned_fleets={{fleet=4}}} events={special_project={id=9}}}}
fleet={0={current_order={collect_data_fleet_order={order_id=0 planet=0 special_project=0 unknown={army=0 system=0}
 anonymous fleet=2 archaeological_site=0} repeat={planet=0 planet=1 special_project=1}
 malformed={planet={x=0}}} current_order={} current_order=bad}
 1={current_order={other=yes}} 01={} 2=none 3={name={key=sample}} 4={current_order={special_project=9}}}
planets={planet={0={sample=yes}}} archaeological_sites={sites={0={sample=yes}}}`
	syntheticSaveDir(t, []syntheticEntry{{"meta", []byte("sample=yes")}, {"gamestate", []byte(source)}})
	fleets := rawCall(t, handleFleetOrders, nil)
	fleetRows := fleets["rows"].([]any)
	if fleets["view"] != "fleets" || fleets["entry_index"] != float64(1) || fleets["view_status"] != "partial" || len(fleetRows) != 9 {
		t.Fatal("saved fleet scope or malformed entries lost")
	}
	for i, want := range []string{"numeric-candidate", "numeric-candidate", "duplicate", "none", "numeric-candidate"} {
		if fleetRows[i].(map[string]any)["fleet"].(map[string]any)["status"] != want {
			t.Fatalf("wrong fleet table status at row %d", i)
		}
	}
	if fleetRows[0].(map[string]any)["owned_reference_occurrences"] != float64(2) ||
		fleetRows[0].(map[string]any)["current_order_occurrences"] != float64(3) ||
		fleetRows[4].(map[string]any)["current_order_occurrences"] != float64(0) ||
		fleetRows[5].(map[string]any)["role"] != "duplicate-fleet-field" {
		t.Fatal("duplicate, absent or ambiguous order count lost")
	}
	if fleetRows[2].(map[string]any)["current_order_occurrences"] != nil {
		t.Fatal("ambiguous fleet was treated as a known order count")
	}

	orders := rawCall(t, handleFleetOrders, map[string]any{"fleet_id": 0})
	rows := orders["rows"].([]any)
	if orders["view"] != "orders" || orders["view_status"] != "duplicate" || orders["current_order_occurrences"] != float64(3) ||
		orders["row_count"] != float64(20) || orders["next_offset"] != nil || orders["owned_reference_paths_preview"].([]any)[1] != "/1/0/0/0/1" {
		t.Fatal("saved current_order wrappers or membership paths lost")
	}
	for i, suffix := range []string{"0", "0/0", "0/0/0", "0/0/1", "0/0/2", "0/0/3", "0/0/3/0", "0/0/3/1",
		"0/0/4", "0/0/5", "0/0/6", "0/1", "0/1/0", "0/1/1", "0/1/2", "0/2", "0/2/0", "0/2/0/0", "1", "2"} {
		if rows[i].(map[string]any)["path"] != "/2/0/"+suffix {
			t.Fatalf("saved current_order order changed at row %d", i)
		}
	}
	for i, want := range map[int]struct{ key, ref, status string }{
		3: {"planet", "integer", "numeric-candidate"}, 4: {"special_project", "integer", "observed"},
		9: {"fleet", "integer", "none"}, 10: {"archaeological_site", "integer", "numeric-candidate"},
		12: {"planet", "duplicate", ""}, 13: {"planet", "duplicate", ""},
		14: {"special_project", "integer", "duplicate"}, 16: {"planet", "unsupported", ""},
	} {
		row := rows[i].(map[string]any)
		if row["reference_target"] != want.key || row["reference_status"] != want.ref {
			t.Fatalf("explicit reference shape lost at row %d", i)
		}
		if want.status == "" {
			if row["reference"] != nil {
				t.Fatalf("ambiguous or unsupported field matched a record at row %d", i)
			}
		} else if row["reference"].(map[string]any)["status"] != want.status {
			t.Fatalf("wrong candidate status at row %d", i)
		}
	}
	for _, i := range []int{2, 6, 7, 8, 17} {
		if rows[i].(map[string]any)["reference_target"] != nil {
			t.Fatalf("generic or anonymous field interpreted as a reference at row %d", i)
		}
	}
	project := rows[4].(map[string]any)
	if project["reference"].(map[string]any)["path"] != "/1/0/1/0" ||
		orders["candidate_tables"].([]any)[2].(map[string]any)["saved_table"] != "selected country.events.special_project.id" {
		t.Fatal("special project did not stay in the selected country")
	}
	identity := map[string]any{"save_id": orders["save_id"], "revision": orders["revision"], "entry_index": orders["entry_index"], "path": project["path"]}
	tree := rawCall(t, handleReadSaveTree, identity)
	if tree["node"].(map[string]any)["path"] != project["path"] {
		t.Fatal("order raw path did not resolve")
	}
	delete(identity, "path")
	identity["offset"], identity["limit"] = project["entry_source_offset"], project["entry_source_bytes"]
	bytes := rawCall(t, handleReadSaveBytes, identity)
	if bytes["data"] != base64.StdEncoding.EncodeToString([]byte("special_project=0")) {
		t.Fatal("order field did not preserve source bytes")
	}

	duplicate := rawCall(t, handleFleetOrders, map[string]any{"fleet_id": 1})
	if duplicate["view_status"] != "fleet-duplicate" || duplicate["rows"].([]any)[1].(map[string]any)["path"] != "/2/2" {
		t.Fatal("duplicate fleet records were merged or lost")
	}
	foreign := rawCall(t, handleFleetOrders, map[string]any{"fleet_id": 4})
	if foreign["view_status"] != "fleet-not-observed-in-owned-fleets" || len(foreign["rows"].([]any)) != 0 {
		t.Fatal("ambiguous or foreign fleet was accepted")
	}
}

func TestFleetOrdersUnavailableAndCountryProjectScope(t *testing.T) {
	for _, tc := range []struct{ name, manager, status string }{
		{"absent", "", "not-recorded"},
		{"manager only", "fleets_manager={other=yes}", "not-recorded"},
		{"empty", "fleets_manager={owned_fleets={}}", "empty"},
		{"unsupported", "fleets_manager={owned_fleets=none}", "partial"},
		{"duplicate", "fleets_manager={owned_fleets={}} fleets_manager={owned_fleets={}}", "duplicate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "player={country=0} country={0={" + tc.manager + "}}"
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
			result := rawCall(t, handleFleetOrders, nil)
			if result["view_status"] != tc.status || result["fleet_table"].(map[string]any)["status"] != "not-recorded" {
				t.Fatal("unavailable ownership coverage was not reported")
			}
		})
	}
	const source = `player={country=0} country={0={fleets_manager={owned_fleets={{fleet=0}}} events={other=yes}}
1={events={special_project={id=9}}}} fleet={0={current_order={collect_data_fleet_order={special_project=9}}}}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	result := rawCall(t, handleFleetOrders, map[string]any{"fleet_id": 0})
	if result["rows"].([]any)[2].(map[string]any)["reference"].(map[string]any)["status"] != "missing" {
		t.Fatal("matching project from a different country leaked into candidates")
	}
}

func TestFleetOrdersPaginationAndSelection(t *testing.T) {
	manager := "{fleets_manager={owned_fleets={" + strings.Repeat("{fleet=0} {fleet=1} ", 11) + "}}}"
	fleet := "{current_order={" + strings.Repeat("order_id=0 ", 23) + "}}"
	source := "player={{country=0} {country=0}} country={0=" + manager + " 1=" + manager + "} fleet={0=" + fleet + " 1=" + fleet + "}"
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	s := server.NewMCPServer("test", "test")
	registerFleetOrders(s)
	tool := s.ListTools()["get_fleet_orders"].Tool
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint ||
		tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only annotations")
	}
	first := rawCall(t, handleFleetOrders, map[string]any{"fleet_id": 0})
	if len(first["rows"].([]any)) != 20 || first["row_count"] != float64(24) || first["next_offset"] != float64(20) {
		t.Fatal("order fields were not paged in 20-row chunks")
	}
	args := map[string]any{"fleet_id": 0, "country_id": 0, "view": "orders", "offset": 20,
		"save_id": first["save_id"], "page_context": first["page_context"]}
	second := rawCall(t, handleFleetOrders, args)
	if rows := second["rows"].([]any); len(rows) != 4 || second["next_offset"] != nil ||
		rows[0].(map[string]any)["path"] != "/2/0/0/19" {
		t.Fatal("second page lost raw order position")
	}
	for key, bad := range map[string]any{"fleet_id": 1, "country_id": 1, "offset": 25, "page_context": ""} {
		saved := args[key]
		args[key] = bad
		expectFleetOrdersError(t, args)
		args[key] = saved
	}
	for _, bad := range []map[string]any{
		{"view": "fleets", "offset": 20, "page_context": first["page_context"]},
		{"view": "orders"}, {"view": "fleets", "fleet_id": 0}, {"view": "unknown"},
		{"fleet_id": -1}, {"fleet_id": "0"}, {"country_id": 0.5}, {"save_id": false},
	} {
		expectFleetOrdersError(t, bad)
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(source + " ")}})
	expectFleetOrdersError(t, args)
}

func expectFleetOrdersError(t *testing.T, args map[string]any) {
	t.Helper()
	result, err := handleFleetOrders(context.Background(), rawRequest(args))
	if err != nil || !result.IsError {
		t.Fatal("invalid input accepted")
	}
}
