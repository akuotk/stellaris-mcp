package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestSavedMessagesOrderAndPagination(t *testing.T) {
	const record = `message={country=0 country=1 scope={system=2} }`
	var source strings.Builder
	source.WriteString(`unrelated={message=ignore} ` + record + ` other=value message=scalar message={country=1 stray} message={a b} message={} ` + record)
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&source, " message={number=%d}", i)
	}
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(source.String())}})

	s := server.NewMCPServer("test", "test")
	Register(s, dir, loc)
	entry, ok := s.ListTools()["get_saved_messages"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("missing read-only registration")
	}

	ctx := context.Background()
	first, err := handleSavedMessages(ctx, rawRequest(nil))
	if err != nil || first.IsError {
		t.Fatal("first page failed")
	}
	got := resultText(first)
	var token, saveID string
	for _, line := range strings.Split(got, "\n") {
		if value, ok := strings.CutPrefix(line, "page_context="); ok {
			token = value
		}
		if value, ok := strings.CutPrefix(line, "save_id="); ok {
			saveID = value
		}
	}
	if token == "" || saveID == "" {
		t.Fatal("missing continuation context")
	}
	wants := []string{
		"saved_records=22 offset=0",
		`record[0] root[1] key="message": object with 3 saved fields`,
		`Field 0 key="country": "0" (saved scalar)`,
		`Field 1 key="country" [duplicate saved key]: "1" (saved scalar)`,
		`Field 0 key="system": "2" (saved scalar)`,
		`record[1] root[3] key="message": "scalar" (saved scalar)`,
		`record[2] root[4] key="message": mixed saved block`,
		`record[3] root[5] key="message": list with 2 saved entries`,
		`record[4] root[6] key="message": empty object`,
		`record[5] root[7] key="message": object with 3 saved fields`,
		`record[19] root[21] key="message"`,
		"next_offset=20",
	}
	for _, want := range wants {
		at := strings.Index(got, want)
		if at < 0 {
			t.Fatalf("missing or out-of-order output %q", want)
		}
		got = got[at+len(want):]
	}
	if strings.Contains(resultText(first), "record[20]") || strings.Contains(resultText(first), "ignore") {
		t.Fatal("first page included an out-of-page or nested message")
	}

	args := map[string]any{"save_id": saveID, "offset": 20, "page_context": token}
	second, err := handleSavedMessages(ctx, rawRequest(args))
	if err != nil || second.IsError {
		t.Fatal("second page failed")
	}
	got = resultText(second)
	for _, want := range []string{
		"saved_records=22 offset=20",
		`record[20] root[22] key="message"`,
		`record[21] root[23] key="message"`,
		"next_offset=none",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}

	for _, invalid := range []map[string]any{
		{"offset": 20},
		{"offset": 20, "page_context": "wrong"},
		{"offset": 23, "page_context": token},
		{"offset": 0.5},
		{"save_id": 0},
		{"page_context": 0},
	} {
		result, err := handleSavedMessages(ctx, rawRequest(invalid))
		if err != nil || !result.IsError {
			t.Errorf("accepted invalid request %v", invalid)
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(source.String() + " changed=yes")}})
	result, err := handleSavedMessages(ctx, rawRequest(args))
	if err != nil || !result.IsError || !strings.Contains(resultText(result), "page_context") {
		t.Fatal("accepted stale continuation")
	}
}

func TestSavedMessagesAbsent(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`country={0={message={}}}`)}})
	result, err := handleSavedMessages(context.Background(), rawRequest(nil))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "saved_records=0 offset=0\n") ||
		!strings.Contains(resultText(result), "next_offset=none") {
		t.Fatal("expected empty root message list")
	}
}
