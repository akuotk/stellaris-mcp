package tools

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/server"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlayerEventsSavedOrderAndPages(t *testing.T) {
	var in strings.Builder
	in.WriteString(`country={0={}} player_event={country=0 id=1} player_event={country=0 country=1} open_player_event_selection_history={selected={ `)
	for i := 0; i < 22; i++ {
		fmt.Fprintf(&in, `{human=0 option=%d player_event=1} `, i)
	}
	in.WriteString(`} extra=unknown} player_event=bad`)
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(in.String())}})
	s := server.NewMCPServer("test", "test")
	registerPlayerEvents(s)
	if _, ok := s.ListTools()["get_player_events"]; !ok {
		t.Fatal("missing registration")
	}
	result, err := handlePlayerEvents(context.Background(), rawRequest(nil))
	if err != nil || result.IsError {
		t.Fatal("first page failed")
	}
	got := resultText(result)
	for _, want := range []string{"saved_records=26", "Country ID matches a decoded country record", "Country field unsupported or duplicate", "next_offset=20"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
	var token string
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "page_context=") {
			token = strings.TrimPrefix(line, "page_context=")
		}
	}
	if token == "" {
		t.Fatal("missing token")
	}
	result, err = handlePlayerEvents(context.Background(), rawRequest(map[string]any{"offset": 20, "page_context": token}))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "next_offset=none") || !strings.Contains(resultText(result), `key="extra"`) {
		t.Fatal("last page failed")
	}
	for _, args := range []map[string]any{{"offset": 20}, {"offset": 0.5}, {"save_id": 0}} {
		result, err = handlePlayerEvents(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Fatal("invalid request accepted")
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(in.String() + " changed=yes")}})
	result, err = handlePlayerEvents(context.Background(), rawRequest(map[string]any{"offset": 20, "page_context": token}))
	if err != nil || !result.IsError {
		t.Fatal("changed save accepted")
	}
}
func TestPlayerEventsAbsentAndMalformed(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`country={0={player_event={}}} open_player_event_selection_history=unknown`)}})
	result, err := handlePlayerEvents(context.Background(), rawRequest(nil))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "saved_records=1") || !strings.Contains(resultText(result), "unknown") {
		t.Fatal("unexpected malformed handling")
	}
}
