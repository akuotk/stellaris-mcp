package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func tradeRouteText(t *testing.T, args map[string]any) string {
	t.Helper()
	result, err := handleTradeRoutes(context.Background(), rawRequest(args))
	if err != nil || result.IsError {
		t.Fatalf("trade route read failed: %v %v", result, err)
	}
	return resultText(result)
}

func requireTradeRouteOrder(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		at := strings.Index(got, want)
		if at < 0 {
			t.Fatalf("missing or out-of-order %q", want)
		}
		got = got[at+len(want):]
	}
}

func TestTradeRoutesSavedOrderAndCandidates(t *testing.T) {
	const source = `
country={0={} 7={} 7={}}
galactic_object={0={} 2={} 7={}}
planets={planet={0={} 2={}}}
starbase_mgr={starbases={0={}}}
fleet={0={} 2=invalid}
trade_routes={0={owner=0 from=2 to=7 path={{id=2 collected=1} {id=0} {id=0 id=7} {note=x}} path={{id=7}}} 5=none 06={} extra=opaque 8={p q} 9={}}
trade_routes_manager={marker=first trade_routes={0 5 6 404 404} marker=last}
trade_routes={6={a x=y}}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})

	requireTradeRouteOrder(t, tradeRouteText(t, nil),
		"kind=routes saved_entries=7 offset=0",
		"route_roots=2 manager_roots=1 manager_list_fields=1 route_lookup_incomplete=false manager_lookup_incomplete=false",
		`entry[0] root[5] key="trade_routes"/field[0] key="0"`,
		"route_status=unique route_key_occurrences=1 manager_status=unique manager_id_occurrences=1",
		"path field[3] in saved order: list with 4 saved entries",
		`Field 1 key="collected": "1" (saved scalar)`,
		"Item 1: object with 1 saved fields",
		`Field 0 key="id": "0" (saved scalar)`,
		`Field 1 key="id" [duplicate saved key]: "7" (saved scalar)`,
		"path field[4] in saved order: list with 1 saved entries",
		"Multiple saved path fields",
		"owner=0 numeric key lookup country=candidate system=candidate planet=candidate starbase=candidate fleet=candidate",
		"from=2 numeric key lookup country=no-candidate system=candidate planet=candidate starbase=no-candidate fleet=unsupported",
		"to=7 numeric key lookup country=duplicate system=candidate planet=no-candidate starbase=no-candidate fleet=no-candidate",
		"path field[3]/item[0].id=2 numeric key lookup",
		"path field[3]/item[1].id=0 numeric key lookup",
		"path field[3]/item[2].id unsupported or duplicate",
		"path field[3]/item[3].id not recorded",
		"path field[4]/item[0].id=7 numeric key lookup",
		`entry[1] root[5] key="trade_routes"/field[1] key="5": "none" (saved scalar)`,
		"route_status=unsupported route_key_occurrences=1 manager_status=unique",
		`entry[2] root[5] key="trade_routes"/field[2] key="06": empty object`,
		"route_status=duplicate route_key_occurrences=2 manager_status=unique",
		`entry[3] root[5] key="trade_routes"/field[3] key="extra" (unsupported for lookup; use raw tools): "opaque" (saved scalar)`,
		`entry[4] root[5] key="trade_routes"/field[4] key="8": list with 2 saved entries`,
		"route_status=unsupported route_key_occurrences=1 manager_status=outside-list",
		`entry[5] root[5] key="trade_routes"/field[5] key="9": empty object`,
		"route_status=unique route_key_occurrences=1 manager_status=outside-list",
		`entry[6] root[7] key="trade_routes"/field[0] key="6": mixed saved block`,
		"route_status=duplicate route_key_occurrences=2 manager_status=unique")

	requireTradeRouteOrder(t, tradeRouteText(t, map[string]any{"kind": "manager"}),
		"kind=manager saved_entries=7 offset=0",
		`entry[0] root[6] key="trade_routes_manager"/field[0] key="marker": "first"`,
		`entry[1] root[6] key="trade_routes_manager"/field[1] key="trade_routes"/item[0]: "0"`,
		"route_status=unique route_key_occurrences=1 manager_status=unique",
		`/item[1]: "5"`,
		"route_status=unsupported route_key_occurrences=1 manager_status=unique",
		`/item[2]: "6"`,
		"route_status=duplicate route_key_occurrences=2 manager_status=unique",
		`/item[3]: "404"`,
		"route_status=missing route_key_occurrences=0 manager_status=duplicate manager_id_occurrences=2",
		`/item[4]: "404"`,
		"route_status=missing route_key_occurrences=0 manager_status=duplicate manager_id_occurrences=2",
		`entry[6] root[6] key="trade_routes_manager"/field[2] key="marker": "last"`)
}

func TestTradeRoutesIncompleteAndAbsentTables(t *testing.T) {
	t.Run("partial", func(t *testing.T) {
		const source = `trade_routes=opaque trade_routes={0={} 1={path=weird}}
trade_routes_manager={trade_routes={0 bad} trade_routes={0}}
trade_routes_manager={trade_routes={item=0 stray}}`
		syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
		requireTradeRouteOrder(t, tradeRouteText(t, nil),
			"route_roots=2 manager_roots=2 manager_list_fields=3 route_lookup_incomplete=true manager_lookup_incomplete=true",
			`entry[0] root[0] key="trade_routes" (unsupported for lookup; use raw tools): "opaque"`,
			"route_status=unsupported route_key_occurrences=1 manager_status=duplicate manager_id_occurrences=2",
			"route_status=unsupported route_key_occurrences=1 manager_status=unsupported manager_id_occurrences=0",
			"path field[0] has unsupported shape")
		requireTradeRouteOrder(t, tradeRouteText(t, map[string]any{"kind": "manager"}),
			`/item[1] (unsupported for lookup; use raw tools): "bad"`,
			`/field[1] key="trade_routes"/item[0]: "0"`,
			`/field[0] key="trade_routes" (unsupported for lookup; use raw tools): mixed saved block`)
	})
	for _, fixture := range []struct {
		name, source, want string
	}{
		{"both absent", "other=1", "route_roots=0 manager_roots=0 manager_list_fields=0"},
		{"manager absent", "trade_routes={0={}}", "manager_status=not-recorded"},
		{"manager field absent", "trade_routes={0={}} trade_routes_manager={other=1}", "manager_status=not-recorded"},
		{"manager empty", "trade_routes={0={}} trade_routes_manager={trade_routes={}}", "manager_status=outside-list"},
		{"route absent", "trade_routes_manager={trade_routes={0}}", "route_status=missing"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture.source)}})
			args := map[string]any{}
			if fixture.name == "route absent" {
				args["kind"] = "manager"
			}
			requireTradeRouteOrder(t, tradeRouteText(t, args), fixture.want, "next_offset=none")
		})
	}
}

func TestTradeRoutesPagination(t *testing.T) {
	var routes, manager strings.Builder
	for i := 0; i < 22; i++ {
		fmt.Fprintf(&routes, " %d={}", i)
		fmt.Fprintf(&manager, " %d", i)
	}
	source := "trade_routes={" + routes.String() + "} trade_routes_manager={trade_routes={" + manager.String() + "}}"
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	s := server.NewMCPServer("test", "test")
	Register(s, dir, loc)
	entry, ok := s.ListTools()["get_trade_routes"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only registration")
	}

	var continuation map[string]any
	for _, kind := range []string{"routes", "manager"} {
		first := tradeRouteText(t, map[string]any{"kind": kind})
		requireTradeRouteOrder(t, first, "saved_entries=22 offset=0", "entry[19]", "next_offset=20")
		if strings.Contains(first, "entry[20]") {
			t.Fatal("page exceeds 20 entries")
		}
		continuation = map[string]any{"kind": kind, "offset": 20}
		for _, line := range strings.Split(first, "\n") {
			key, value, ok := strings.Cut(line, "=")
			if ok && (key == "save_id" || key == "page_context") {
				continuation[key] = value
			}
		}
		if continuation["save_id"] == "" || continuation["save_id"] == nil || continuation["page_context"] == "" || continuation["page_context"] == nil {
			t.Fatal("missing continuation context")
		}
		requireTradeRouteOrder(t, tradeRouteText(t, continuation), "saved_entries=22 offset=20", "entry[20]", "entry[21]", "next_offset=none")
	}
	for _, args := range []map[string]any{
		{"kind": "routes", "offset": 20, "page_context": continuation["page_context"]},
		{"kind": "manager", "offset": 23, "page_context": continuation["page_context"]},
		{"offset": 20},
		{"offset": 0.5},
		{"kind": "invalid"},
		{"save_id": 0},
		{"page_context": 0},
	} {
		result, err := handleTradeRoutes(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Errorf("accepted invalid request %v", args)
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(source + " changed=yes")}})
	result, err := handleTradeRoutes(context.Background(), rawRequest(continuation))
	if err != nil || !result.IsError || !strings.Contains(resultText(result), "page_context") {
		t.Fatal("accepted stale continuation")
	}
}
