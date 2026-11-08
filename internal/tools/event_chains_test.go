package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestEventChainsSavedOrderAndBoundPages(t *testing.T) {
	var input strings.Builder
	input.WriteString(`country={0={events={event_chain={id=0 scope={type=unknown id=0} counter=0} completed_event_chain=first `)
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&input, `event_chain={id=%d} `, i+1)
	}
	input.WriteString(`}} 1={events={completed_event_chain=other}}}`)
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(input.String())}})
	s := server.NewMCPServer("test", "test")
	registerEventChains(s)
	if _, ok := s.ListTools()["get_event_chains"]; !ok {
		t.Fatal("tool not registered")
	}
	result, err := handleEventChains(context.Background(), rawRequest(map[string]any{"country_id": 0}))
	if err != nil || result.IsError {
		t.Fatal("first page failed")
	}
	got := resultText(result)
	for _, want := range []string{`entry[0] key="event_chain"`, `entry[1] key="completed_event_chain"`, `key="id": "0"`, "saved_entries=22", "next_offset=20"} {
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
	result, err = handleEventChains(context.Background(), rawRequest(map[string]any{"country_id": 0, "offset": 20, "page_context": token}))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "next_offset=none") {
		t.Fatal("second page failed")
	}
	for _, args := range []map[string]any{{"country_id": 0, "offset": 20}, {"country_id": 1, "page_context": token}, {"country_id": 0, "offset": 0.1}, {"country_id": 0, "save_id": 0}} {
		result, err = handleEventChains(context.Background(), rawRequest(args))
		if err != nil || !result.IsError {
			t.Fatal("invalid request accepted")
		}
	}
	writeSyntheticSave(t, filepath.Join(dir, "synthetic.sav"), []syntheticEntry{{"gamestate", []byte(input.String() + " changed=yes")}})
	result, err = handleEventChains(context.Background(), rawRequest(map[string]any{"country_id": 0, "offset": 20, "page_context": token}))
	if err != nil || !result.IsError {
		t.Fatal("changed save accepted old page context")
	}
}

func TestEventChainsMissingAndDuplicateEvents(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`country={0={}}`, "country.events not recorded"},
		{`country={0={events={} events={}}}`, "country.events unsupported or duplicate"},
		{`country={0={events=bad}}`, "country.events unsupported or duplicate"},
	} {
		syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(tc.input)}})
		result, err := handleEventChains(context.Background(), rawRequest(map[string]any{"country_id": 0}))
		if err != nil || result.IsError || !strings.Contains(resultText(result), tc.want) {
			t.Fatalf("missing %s", tc.want)
		}
	}
}

func TestEventChainsNestedTargetsPreserveAmbiguity(t *testing.T) {
	data := `saved_event_target={name=example type=1 id=0 opener_id=3} saved_event_target={name=example type=1 id=0 opener_id=4} saved_event_target={name=bad id=0} country={0={events={event_chain={scope={from={saved_event_target={name=example type=1 id=0 opener_id=9} saved_event_target={name=other type=1 id=99} saved_event_target={name=bad name=duplicate type=1 id=0}}} mixed={anonymous key=value}}}}} galactic_object={0={}} planets={planet={0={}}} ships={0={}} fleet={0={}}`
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(data)}})
	result, err := handleEventChains(context.Background(), rawRequest(map[string]any{"country_id": 0}))
	if err != nil || result.IsError {
		t.Fatal("event chains failed")
	}
	got := resultText(result)
	for _, want := range []string{"Root target comparison incomplete: 1", "2 root candidate(s) match saved name/type/id", "0 root candidate(s) match saved name/type/id", "Root candidate comparison unresolved", "Nested target scan incomplete for mixed data", "opener and identity unverified", "Numeric candidates in decoded tables: country, system, planet, ship, fleet", "No numeric candidates in decoded tables; source coverage unverified"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	if strings.Count(got, "Nested saved_event_target") != 3 {
		t.Fatal("nested targets missing or duplicated")
	}
}
