package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestFactionsHandlerKeepsScopeAndReciprocalMemberEvidence(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	data := []byte(`player={{country=0}}
	country={0={adjective={key=Home} modules={standard_pop_factions_module={pop_factions={0 0 1 2 3}
	actual_count={00.0 1} potential_count={2} potential={type=possible parameters={}} potential={type=also_possible}}}}
	1={adjective={key=Foreign}}}
	pop_factions={0={name={key="Faction Zero"} country=0 support_percent=00.25 support_power=-0.00
	members={0 1 2 3 4 5 6 7 8 0}
	last_month_colony_pops={{colony=0 num_pops=00.00 change=-0} {colony=0 num_pops=0} {colony=9} {colony=0 colony=1}}
	parameters={{key=example data={type=unknown id=0}}}}
	1={country=1 name={key="Conflicting faction"}} 2={} 2=none 3=none 9={country=1 name={key="Unscoped faction"}}}
	pop={1={pop_faction=0} 2={pop_faction=0} 3={pop_faction=1} 7={pop_faction=0 pop_faction=1}}
	pop_groups={0={factions={objects={0} pops={0}}} 2={factions={objects={0} pops={2}}}
	5={factions={objects={0 0} pops={4 5}}} 6={factions={objects={0} pops={}}}
	8={factions={objects={0} pops={0}}} 8=none}
	colony={0={carrier={type=planet reference=0}}}
	planets={planet={0={name={key="Synthetic world"} colony=0}}}`)
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", data}})
	s := server.NewMCPServer("test", "test")
	registerFactions(s)
	entry, ok := s.ListTools()["get_factions"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint ||
		entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("get_factions registration or read-only annotations missing")
	}
	summary, err := handleFactions(context.Background(), rawRequest(nil))
	if err != nil || summary.IsError {
		t.Fatalf("get_factions summary failed: %v", err)
	}
	out := resultText(summary)
	for _, want := range []string{
		"standard_pop_factions_module.pop_factions (saved order): [0 0 1 2 3]",
		`actual_count (saved order): ["00.0" "1"]`,
		"potential [repeated saved key]",
		"Selected module membership=2 duplicate references",
		"Saved faction country differs from selected country.",
		"Faction 2: duplicate saved record ID or fields",
		"Faction 3: referenced record unavailable",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in faction summary: %s", want, out)
		}
	}
	if strings.Contains(out, "Unscoped faction") || strings.Contains(out, "members (saved order)") {
		t.Fatal("foreign faction or detailed members leaked into the summary")
	}
	detail, err := handleFactions(context.Background(), rawRequest(map[string]any{"faction_id": float64(0)}))
	if err != nil || detail.IsError {
		t.Fatalf("get_factions detail failed: %v", err)
	}
	out = resultText(detail)
	for _, want := range []string{
		`support_percent="00.25" faction_approval=not recorded support_power="-0.00"`,
		"members (saved order): [0 1 2 3 4 5 6 7 8 0]",
		"Pop group 0: reciprocal faction ID agrees, raw population=0",
		"Legacy pop reciprocal links: 1",
		"Member 2: ID appears in both population namespaces. Membership unresolved.",
		"Legacy pop 3: saved pop_faction=1 differs from faction ID",
		"Member 4: population record unavailable",
		"Pop group 5: duplicate faction back-references, raw amounts=[4 5]",
		"Pop group 6: unsupported or mismatched faction arrays",
		"Legacy pop 7: duplicate pop_faction fields",
		"Pop group 8: duplicate faction fields or group records",
		"Member 0: duplicate saved member reference",
		`Row 0 colony=0 num_pops="00.00" change="-0"`,
		"Synthetic world, reciprocal planet link validated",
		"Duplicate saved colony reference.",
		"Colony record unavailable.",
		"Row 3: duplicate saved fields",
		"parameters[0]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in faction detail: %s", want, out)
		}
	}
	foreign, err := handleFactions(context.Background(), rawRequest(map[string]any{"faction_id": float64(9)}))
	if err != nil || !foreign.IsError {
		t.Fatal("unscoped faction accepted")
	}
}
