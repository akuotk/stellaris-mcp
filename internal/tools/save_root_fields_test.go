package tools

import (
	"context"
	"strings"
	"testing"
)

func TestSaveRootFieldsListsMetaAndGamestate(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{
		{"meta", []byte(`version="Cygnus v4.5.2" date="2300.01.01" required_dlcs={ "Utopia" "Federations" }`)},
		{"gamestate", []byte(`version="Cygnus v4.5.2" tick=123 cheated_on_save=no required_dlcs={ "Utopia" } achievement={ 1 2 3 } country={0={n=a} 1={n=b}} storms={} mixed={ a=1 2 } flag=a flag=b nested={ {a=1} 2 }`)},
	})
	result, err := handleSaveRootFields(context.Background(), rawRequest(map[string]any{}))
	if err != nil || result.IsError {
		t.Fatal("root fields failed")
	}
	text := resultText(result)
	ordered := []string{
		"[meta]", `/0 version = "Cygnus v4.5.2"`, `/2 required_dlcs: list of 2: "Utopia" "Federations"`,
		"[gamestate]", `/1 tick = "123"`, `/2 cheated_on_save = "no"`, `/4 achievement: list of 3: "1" "2" "3"`,
		"/5 country: block with 2 saved fields (dedicated tools: get_empire_overview, get_country_saved_fields)",
		"/6 storms: empty block", "/7 mixed: mixed saved block", `/8 flag [repeated key, occurrence 1] = "a"`, `/9 flag [repeated key, occurrence 2] = "b"`,
		"/10 nested: list of 2 (includes non-scalar items; use raw tools): \"2\"\n",
	}
	last := -1
	for _, want := range ordered {
		at := strings.Index(text, want)
		if at < 0 || at < last {
			t.Fatalf("missing or out of order %q in\n%s", want, text)
		}
		last = at
	}
	result, _ = handleSaveRootFields(context.Background(), rawRequest(map[string]any{"entry": "gamestate", "key": "required_dlcs"}))
	text = resultText(result)
	if strings.Contains(text, "[meta]") || !strings.Contains(text, `/3 required_dlcs: list of 1: "Utopia"`) || strings.Contains(text, "tick") {
		t.Fatalf("filtered output wrong:\n%s", text)
	}
	result, _ = handleSaveRootFields(context.Background(), rawRequest(map[string]any{"entry": "meta", "key": "absent"}))
	if !strings.Contains(resultText(result), "key not recorded") {
		t.Fatalf("absent key: %s", resultText(result))
	}
	for _, args := range []map[string]any{{"entry": "other"}, {"key": 3}, {"save_id": 1}} {
		r, err := handleSaveRootFields(context.Background(), rawRequest(args))
		if err != nil || !r.IsError {
			t.Fatalf("invalid args accepted %v", args)
		}
	}
}

func TestSaveRootFieldsReportsMissingEntries(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`tick=1`)}})
	result, _ := handleSaveRootFields(context.Background(), rawRequest(map[string]any{}))
	if !strings.Contains(resultText(result), "[meta]\n  entry not present") {
		t.Fatalf("missing meta not reported: %s", resultText(result))
	}
}
