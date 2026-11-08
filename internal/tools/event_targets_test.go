package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestSavedEventTargetsCandidatesAndBoundPages(t *testing.T) {
	var input strings.Builder
	input.WriteString(`country={0={}} fleet={0={}} planets={planet={0={}}} ships={0={}} galactic_object={0={}} saved_event_target={id=0 type=unknown name=first opener_id=0} saved_event_target={id=0 type=other name=another} saved_event_target={id=0 id=1 type=unknown name=bad} saved_event_target=none `)
	for i := 0; i < 19; i++ {
		fmt.Fprintf(&input, `saved_event_target={id=%d type=unknown name=synthetic} `, i+1)
	}
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(input.String())}})
	s := server.NewMCPServer("test", "test")
	registerSavedEventTargets(s)
	if _, ok := s.ListTools()["get_saved_event_targets"]; !ok {
		t.Fatal("tool not registered")
	}
	result, err := handleSavedEventTargets(context.Background(), rawRequest(nil))
	if err != nil || result.IsError {
		t.Fatal("first page failed")
	}
	got := resultText(result)
	for _, want := range []string{"saved_records=23", "Unqualified numeric ID occurs in 2 root records; namespace unverified", "Numeric ID candidates: country, system, planet, ship, fleet (namespace unverified)", "ID unsupported or duplicate", "Target shape unsupported", "next_offset=20"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	var token string
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "page_context=") {
			token = strings.TrimPrefix(line, "page_context=")
		}
	}
	if token == "" {
		t.Fatal("missing page context")
	}
	result, err = handleSavedEventTargets(context.Background(), rawRequest(map[string]any{"offset": 20, "page_context": token}))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "next_offset=none") {
		t.Fatal("second page failed")
	}
	for _, args := range []map[string]any{{"offset": 20}, {"offset": 0.5}, {"save_id": 0}} {
		result, err = handleSavedEventTargets(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Fatal("invalid request accepted")
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(input.String() + " changed=yes")}})
	result, err = handleSavedEventTargets(context.Background(), rawRequest(map[string]any{"offset": 20, "page_context": token}))
	if err != nil || !result.IsError {
		t.Fatal("changed save accepted old page context")
	}
}

func TestSavedEventTargetsRootOnlyAndAbsent(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`country={0={events={event_chain={scope={saved_event_target={id=0}}}}}}`)}})
	result, err := handleSavedEventTargets(context.Background(), rawRequest(nil))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "saved_records=0") {
		t.Fatal("nested target treated as root record")
	}
}
