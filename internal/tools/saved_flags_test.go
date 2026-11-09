package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func savedFlagText(t *testing.T, args map[string]any) string {
	t.Helper()
	result, err := handleSavedFlags(context.Background(), rawRequest(args), nil)
	if err != nil || result.IsError {
		t.Fatalf("saved flag read failed: %v %v", result, err)
	}
	return resultText(result)
}

func requireSavedFlagOrder(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		at := strings.Index(got, want)
		if at < 0 {
			t.Fatalf("missing or out-of-order output %q", want)
		}
		got = got[at+len(want):]
	}
}

func TestSavedFlagsRootOrderAndShapes(t *testing.T) {
	const fixture = `
country={flags={ignored=0}}
flags={a=0 a=1 object={value=2 value=3} empty={} list={4 5} mixed={field=6 anonymous}}
unrelated=2
flags={}
flags=scalar
flags={first second}
flags={field=7 anonymous}
flags={a=8}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(fixture)}})
	got := savedFlagText(t, nil)
	requireSavedFlagOrder(t, got,
		"root_occurrences=6 saved_entries=11 offset=0",
		`entry[0] root[1] key="flags"/field[0] key="a" [duplicate saved key]: "0" (saved scalar)`,
		`entry[1] root[1] key="flags"/field[1] key="a" [duplicate saved key]: "1" (saved scalar)`,
		`entry[2] root[1] key="flags"/field[2] key="object": object with 2 saved fields`,
		`Field 0 key="value": "2" (saved scalar)`,
		`Field 1 key="value" [duplicate saved key]: "3" (saved scalar)`,
		`entry[3] root[1] key="flags"/field[3] key="empty": empty object`,
		`entry[4] root[1] key="flags"/field[4] key="list": list with 2 saved entries`,
		`Item 0: "4" (saved scalar)`,
		`Item 1: "5" (saved scalar)`,
		`entry[5] root[1] key="flags"/field[5] key="mixed": mixed saved block; use raw tree/byte tools`,
		`entry[6] root[3] key="flags": empty object`,
		`entry[7] root[4] key="flags" (unsupported root shape; use raw tools): "scalar" (saved scalar)`,
		`entry[8] root[5] key="flags" (unsupported root shape; use raw tools): list with 2 saved entries`,
		`Item 0: "first" (saved scalar)`,
		`entry[9] root[6] key="flags" (unsupported root shape; use raw tools): mixed saved block`,
		`entry[10] root[7] key="flags"/field[0] key="a": "8" (saved scalar)`,
		"next_offset=none")
	if strings.Contains(got, "ignored") {
		t.Fatal("nested flags were included")
	}
}

func TestSavedFlagsAbsentAndLargeList(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte("country={flags={}}")}})
		requireSavedFlagOrder(t, savedFlagText(t, nil), "root_occurrences=0 saved_entries=0", "next_offset=none")
	})
	t.Run("large list", func(t *testing.T) {
		source := "flags={" + strings.Repeat(" synthetic", 25) + "}"
		syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
		got := savedFlagText(t, nil)
		requireSavedFlagOrder(t, got, "root_occurrences=1 saved_entries=1", "large saved block; use raw tree/byte tools")
		if strings.Contains(got, `"synthetic" (saved scalar)`) {
			t.Fatal("large list was expanded")
		}
	})
}

func TestSavedFlagsPagination(t *testing.T) {
	var b strings.Builder
	b.WriteString("flags={repeat=0")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, " field%d=%d", i, i)
	}
	b.WriteString(" repeat=1}")
	source := b.String()
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source)}})
	s := server.NewMCPServer("test", "test")
	Register(s, dir, loc)
	entry, ok := s.ListTools()["get_saved_flags"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only registration")
	}

	first := savedFlagText(t, nil)
	requireSavedFlagOrder(t, first, "root_occurrences=1 saved_entries=22 offset=0",
		`key="repeat" [duplicate saved key]: "0"`, "entry[19]", "next_offset=20")
	if strings.Contains(first, "entry[20]") {
		t.Fatal("page exceeds 20 entries")
	}
	var saveID, token string
	for _, line := range strings.Split(first, "\n") {
		if value, ok := strings.CutPrefix(line, "save_id="); ok {
			saveID = value
		}
		if value, ok := strings.CutPrefix(line, "page_context="); ok {
			token = value
		}
	}
	if saveID == "" || token == "" {
		t.Fatal("missing continuation context")
	}
	args := map[string]any{"save_id": saveID, "offset": 20, "page_context": token}
	requireSavedFlagOrder(t, savedFlagText(t, args), "saved_entries=22 offset=20",
		`entry[20] root[0] key="flags"/field[20] key="field19"`,
		`entry[21] root[0] key="flags"/field[21] key="repeat" [duplicate saved key]: "1"`,
		"next_offset=none")
	for _, invalid := range []map[string]any{
		{"offset": 20},
		{"offset": 20, "page_context": "wrong"},
		{"offset": 23, "page_context": token},
		{"offset": 0.5},
		{"save_id": 0},
		{"page_context": 0},
	} {
		result, err := handleSavedFlags(context.Background(), rawRequest(invalid), nil)
		if err != nil || !result.IsError {
			t.Errorf("accepted invalid request %v", invalid)
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(source + " changed=yes")}})
	result, err := handleSavedFlags(context.Background(), rawRequest(args), nil)
	if err != nil || !result.IsError || !strings.Contains(resultText(result), "page_context") {
		t.Fatal("accepted stale continuation")
	}
}

func TestSavedFlagsLargeNestedBlocks(t *testing.T) {
	var input strings.Builder
	input.WriteString("flags={large_object={")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&input, "key%d=1 ", i)
	}
	input.WriteString("} nested={items={")
	for i := 0; i < 30; i++ {
		input.WriteString("1 ")
	}
	input.WriteString("}}}")
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(input.String())}})
	result, err := handleSavedFlags(context.Background(), rawRequest(nil), nil)
	if err != nil || result.IsError {
		t.Fatal("read failed")
	}
	got := resultText(result)
	if strings.Count(got, "large saved block; use raw tree/byte tools") != 2 || strings.Contains(got, "key29") {
		t.Fatalf("large blocks expanded: %s", got)
	}
}

func TestSavedFlagsScriptReferencesOptIn(t *testing.T) {
	input := "flags={sample={nested=0} " + strings.Repeat("unmatched=1 ", 19) + "sample=2} flags={}"
	savePath := filepath.Join(syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(input)}}), "synthetic.sav")
	root := t.TempDir()
	locale := filepath.Join(root, "localisation", "english")
	for _, dir := range []string{locale, filepath.Join(root, "common"), filepath.Join(root, "events")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("stellaris.exe", "")
	write("launcher-settings.json", "")
	write("events/private-script.txt", "set_global_flag=sample set_global_flag=sample has_global_flag=nested")
	source := gamestate.NewFlagScriptSource(locale)
	loads := 0
	s := server.NewMCPServer("test", "test")
	Register(s, saveDir, loc, func() gamestate.FlagScriptIndex {
		loads++
		return source()
	})
	entry := s.ListTools()["get_saved_flags"]
	call := func(args map[string]any) *mcp.CallToolResult {
		t.Helper()
		result, err := entry.Handler(context.Background(), rawRequest(args))
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	field := func(result, name string) string {
		t.Helper()
		for _, line := range strings.Split(result, "\n") {
			if value, ok := strings.CutPrefix(line, name+"="); ok {
				return value
			}
		}
		t.Fatalf("missing %s", name)
		return ""
	}
	defaultPage := resultText(call(nil))
	if defaultPage != resultText(call(map[string]any{"include_script_references": false})) ||
		strings.Contains(defaultPage, "script_reference_status") || loads != 0 {
		t.Fatal("default mode changed or loaded the script source")
	}
	if !call(map[string]any{"include_script_references": "true"}).IsError || loads != 0 {
		t.Fatal("invalid boolean loaded the script source")
	}
	first := resultText(call(map[string]any{"include_script_references": true}))
	requireSavedFlagOrder(t, first, "script_reference_status=complete",
		`key="sample" [duplicate saved key]`, `Field 0 key="nested": "0"`,
		"set_global_flag sites=2 files=1", "set_timed_global_flag sites=0 files=0",
		"has_global_flag sites=0 files=0", "remove_global_flag sites=0 files=0")
	if loads != 1 || strings.Contains(first, "private-script") || strings.Contains(first, root) ||
		strings.Contains(first, "has_global_flag sites=1") {
		t.Fatal("response included script source details or indexed a nested saved key")
	}
	args := map[string]any{"include_script_references": true, "save_id": field(first, "save_id"), "offset": 20, "page_context": field(first, "page_context")}
	second := resultText(call(args))
	requireSavedFlagOrder(t, second, "offset=20", "set_global_flag sites=2 files=1", "entry[21]", "next_offset=none")
	if strings.Count(second, "set_global_flag sites=") != 1 {
		t.Fatal("root fallback received a lookup")
	}
	args["include_script_references"] = false
	if !call(args).IsError {
		t.Fatal("script reference token accepted in default mode")
	}
	args["include_script_references"] = true
	write("events/private-script.txt", "set_global_flag=sample")
	changed, err := handleSavedFlags(context.Background(), rawRequest(args), gamestate.NewFlagScriptSource(locale))
	if err != nil || !changed.IsError || !strings.Contains(resultText(changed), "page_context") {
		t.Fatal("changed script snapshot accepted")
	}
	writeSyntheticSave(t, savePath, []syntheticEntry{{"gamestate", []byte(input + " changed=yes")}})
	if !call(args).IsError {
		t.Fatal("changed save revision accepted")
	}
	unavailable := savedFlagText(t, map[string]any{"include_script_references": true})
	if !strings.Contains(unavailable, "script_reference_status=unavailable") || strings.Contains(unavailable, "set_global_flag sites=") {
		t.Fatal("unavailable source was presented as zero references")
	}
}
