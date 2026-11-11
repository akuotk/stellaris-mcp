package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestSavedRecordLinksComparesNamedIntegerFields(t *testing.T) {
	// Root order: 0 leaders, 1 country, 2 planets, 3 ships, 4 galactic_object, 5 storms, 6 saved_leaders, 7 construction, 8 ground_combat
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`leaders={1={n=a} 2=none 3={n=a} 3={n=a}}
	country={0={n=a} 5={n=a}}
	planets={planet={7={n=a}}}
	ships={9={n=a}}
	galactic_object={4={n=a}}
	storms={storms={0={owner=0 leader=1 coordinate={origin=4 x=4} systems={4 8} ship="9" fleet=-1 planet=7.0 count=12 count=13 nested={{leader=3}} target=9}}}
	saved_leaders={lost=2 found=1}
	construction={queue_mgr={queues={11={owner=5}}}}
	ground_combat={0={planet=7 attackers={0 5 6} externally_owned_build_queues={{key=5 value=11}}}}
	`)}})
	revision := rawRevision(t)
	call := func(args map[string]any) map[string]any {
		t.Helper()
		args["revision"] = revision
		return rawCall(t, handleSavedRecordLinks, args)
	}
	page := call(map[string]any{"path": "/5"})
	type row struct {
		Path, Field, Value, Rule string
		Parent                   string `json:"named_parent"`
		Candidates               []struct{ Table, Status string }
	}
	var rows []row
	raw, _ := json.Marshal(page["rows"])
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		var statuses []string
		for _, c := range r.Candidates {
			statuses = append(statuses, c.Table+":"+c.Status)
		}
		got = append(got, fmt.Sprintf("%s %s=%s %s %s", r.Path, r.Field, r.Value, r.Rule, strings.Join(statuses, ",")))
	}
	want := []string{
		"/5/0/0/0 owner=0 field:owner country:numeric-candidate",
		"/5/0/0/1 leader=1 field:leader leader:numeric-candidate",
		"/5/0/0/2/0 origin=4 qualified:coordinate.origin system:numeric-candidate",
		"/5/0/0/3/0 systems=4 field:systems system:numeric-candidate",
		"/5/0/0/3/1 systems=8 field:systems system:missing",
		"/5/0/0/9/0/0 leader=3 field:leader leader:duplicate",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("rows:\n%s", strings.Join(got, "\n"))
	}
	if page["row_count"].(float64) != 6 || page["integer_scalars"].(float64) != 10 {
		t.Fatalf("counts: %v %v", page["row_count"], page["integer_scalars"])
	}
	if page["named_field"] != "storms" || page["unmapped_integer_field_names"].(float64) != 3 {
		t.Fatalf("context/unmapped: %v %v %v", page["named_field"], page["unmapped_integer_field_names"], page["unmapped_integer_fields_preview"])
	}
	unmapped, _ := json.Marshal(page["unmapped_integer_fields_preview"])
	if string(unmapped) != `[{"count":2,"field":"storms.count"},{"count":1,"field":"coordinate.x"},{"count":1,"field":"storms.target"}]` {
		t.Fatalf("unmapped preview %s", unmapped)
	}

	saved := call(map[string]any{"path": "/6"})
	raw, _ = json.Marshal(saved["rows"])
	if !strings.Contains(string(raw), `"rule":"parent:saved_leaders"`) || !strings.Contains(string(raw), `"status":"none"`) {
		t.Fatalf("saved_leaders rows %s", raw)
	}
	ground := call(map[string]any{"path": "/8", "limit": 2})
	if ground["row_count"].(float64) != 6 || ground["next_offset"].(float64) != 2 {
		t.Fatalf("ground paging %v %v", ground["row_count"], ground["next_offset"])
	}
	second := call(map[string]any{"path": "/8", "offset": 2, "limit": 10, "page_context": ground["page_context"]})
	raw, _ = json.Marshal(second["rows"])
	for _, want := range []string{`"field":"attackers","named_parent":"ground_combat","path":"/8/0/1/2","rule":"field:attackers","value":"6"`, `"status":"missing"`, `"rule":"qualified:externally_owned_build_queues.value"`, `"status":"numeric-candidate","table":"construction_queue"`, `"rule":"qualified:externally_owned_build_queues.key"`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("missing %s in %s", want, raw)
		}
	}
	whole := call(map[string]any{"path": "", "limit": 1})
	if whole["row_count"].(float64) < 12 {
		t.Fatalf("whole gamestate rows %v", whole["row_count"])
	}

	for _, args := range []map[string]any{
		{"path": "/5"},
		{"revision": revision},
		{"revision": revision, "path": "/99"},
		{"revision": revision, "path": "/5", "offset": 1},
		{"revision": revision, "path": "/5", "offset": 1, "page_context": "bad"},
		{"revision": revision, "path": "/5", "limit": 0},
		{"revision": revision, "path": "/5", "entry_index": 0},
		{"revision": "stale", "path": "/5"},
	} {
		r, err := handleSavedRecordLinks(context.Background(), rawRequest(args))
		if err != nil || !r.IsError {
			t.Fatalf("invalid request accepted: %v", args)
		}
	}
}

func TestSavedRecordLinksSentinelsFlagsAndColonies(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`planets={planet={3={n=a}}}
	galactic_object={1={colonies={3 4} flags={megastructure=5 debris=6} owner=4294967295 fleet=18446744073709551616 ship=18446744073709551615}}`)}})
	revision := rawRevision(t)
	page := rawCall(t, handleSavedRecordLinks, map[string]any{"revision": revision, "path": "/1"})
	raw, _ := json.Marshal(page["rows"])
	text := string(raw)
	for _, want := range []string{
		`"field":"colonies","named_parent":"galactic_object","path":"/1/0/0/0","rule":"qualified:galactic_object.colonies","value":"3"`,
		`"candidates":[{"occurrences":1,"path":"/0/0/0","record_status":"object","status":"numeric-candidate","table":"planet"}]`,
		`"status":"uint32-max-value","table":"country"`,
		`"status":"out-of-int-range","table":"ships"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %s", want, text)
		}
	}
	if strings.Contains(text, "megastructure") || strings.Contains(text, "debris") || strings.Contains(text, `"field":"fleet"`) {
		t.Fatalf("flag names or overflowing integers mapped: %s", text)
	}
	if page["row_count"].(float64) != 4 || page["integer_scalars"].(float64) != 6 {
		t.Fatalf("counts %v %v", page["row_count"], page["integer_scalars"])
	}
	summary, _ := json.Marshal(page["summary"])
	if !strings.Contains(string(summary), `"rule":"qualified:galactic_object.colonies"`) {
		t.Fatalf("summary lacks rule: %s", summary)
	}
	unmapped, _ := json.Marshal(page["unmapped_integer_fields_preview"])
	if !strings.Contains(string(unmapped), `"field":"flags.megastructure"`) {
		t.Fatalf("flag values should be counted as unmapped: %s", unmapped)
	}
}

func TestSavedRecordLinksColoniesUseColonyTableWhenPresent(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`planets={planet={3={n=a}}} colony={3={n=a}}
	galactic_object={1={colonies={3}}}`)}})
	page := rawCall(t, handleSavedRecordLinks, map[string]any{"revision": rawRevision(t), "path": "/2"})
	raw, _ := json.Marshal(page["rows"])
	if !strings.Contains(string(raw), `"candidates":[{"occurrences":1,"path":"/1/0","record_status":"object","status":"numeric-candidate","table":"colony"}]`) {
		t.Fatalf("colonies should be compared with the colony table only: %s", raw)
	}
}
