package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestGovernanceSavedValuesAndCollectionStates(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`country={0={
	 active_policies={{policy=example_policy selected=option_one date=0} {policy=example_policy selected=option_two} {policy=""}}
	 edicts={{edict=example_edict date=0 start_date=2200.01.01 perpetual=unknown} {}}
	 policy_flags={example_flag example_flag ""}
	} 1={active_policies={} edicts={} policy_flags={}} 2={}
	 3={active_policies=unsupported edicts={{perpetual={nested=yes}}} policy_flags={unsupported={}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderGovernance(0, &c))
	for _, want := range []string{`policy="example_policy" selected="option_one" date="0"`, `policy="example_policy" selected="option_two" date=not recorded`, `policy="" selected=not recorded`, `edict="example_edict" date="0" start_date="2200.01.01" perpetual="unknown"`, `edict=not recorded`, `Entry 0 value="example_flag"`, `Entry 1 value="example_flag"`, `Entry 2 value=""`, "relationship to policy selections unverified", "current edict activity remain unverified"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Index(text, `selected="option_one"`) > strings.Index(text, `selected="option_two"`) {
		t.Fatal("saved policy order changed")
	}
	for _, item := range []struct {
		id   int
		want string
	}{{1, "  Empty"}, {2, "  Not recorded"}, {3, "  Unsupported saved shape"}} {
		c := gs.Country[item.id]
		text := resultText(renderGovernance(item.id, &c))
		if strings.Count(text, item.want) != 3 {
			t.Fatalf("collection states not explicit for country %d", item.id)
		}
	}
}

func TestGovernanceRegisteredHandler(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={active_policies={{policy=example selected=choice}} edicts={}} 1={active_policies={{policy=other}}}}`)}})
	s := server.NewMCPServer("test", "test")
	registerGovernance(s)
	entry, ok := s.ListTools()["get_policies_edicts"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint || entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("governance tool missing or annotations incorrect")
	}
	for _, key := range []string{"save_id", "country_id"} {
		if _, ok := entry.Tool.InputSchema.Properties[key]; !ok {
			t.Fatalf("missing input %q", key)
		}
	}
	for _, args := range []map[string]any{nil, {"country_id": 0}} {
		r, err := handleGovernance(context.Background(), rawRequest(args))
		if err != nil || r.IsError || !strings.Contains(resultText(r), `selected="choice"`) || strings.Contains(resultText(r), `policy="other"`) {
			t.Fatal("normal governance request failed or included another country's records")
		}
	}
	for _, args := range []map[string]any{{"country_id": 99}, {"country_id": "0"}, {"save_id": 1}, {"save_id": "../../outside.sav"}} {
		r, err := handleGovernance(context.Background(), rawRequest(args))
		if err != nil || !r.IsError {
			t.Fatal("invalid governance selection accepted")
		}
	}
}
