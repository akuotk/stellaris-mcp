package tools

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func TestSectorsOwnershipAndSystemCrossReferences(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	gs, err := gamestate.Load([]byte(`country={0={sectors={owned={0 0 1 7 8}}}}
	 sectors={0={owner=0 local_capital=0 type=0 auto_expand=unknown systems={0 0 1 2 3 4 99}} 1={owner=1 local_capital=77 systems={}} 2={owner=0} 3={owner=1} 8={owner=0 owner=1}}
	 galactic_object={0={sector=0} 1={sector=1} 2={} 3={sector=bad} 4={sector=0 sector=1} 5={sector=0} 6={sector=2} 7={sector=4294967295} 8={sector=987654}}
	 colony={0={carrier={type=planet reference=6}}} planets={planet={0={name={key="Candidate planet"} owner=1} 6={colony=0 name={key="Reciprocal carrier"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	summary := resultText(renderSectors(gs, 0, &c, nil))
	for _, want := range []string{"Sector 0:", "2 duplicate references", "Sector 1:", "Sector owner differs from selected country", "Sector 2:", "membership=missing reverse reference", "Sector 7: referenced record unavailable", "Sector 8: referenced record unsupported or duplicate", "Sector decoding incomplete", "type=\"0\" auto_expand=\"unknown\" local_capital=0", byIntWidth("raw 4294967295=1 (meaning unverified); other IDs without a decoded sector record=1", "raw 4294967295=0 (meaning unverified); other IDs without a decoded sector record=1"), "Survey coverage incomplete"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in %s", want, summary)
		}
	}
	if strings.Contains(summary, "Sector 3:") || strings.Contains(summary, "System position") {
		t.Fatal("foreign sector or unrequested detail included")
	}
	selected := 0
	details := resultText(renderSectors(gs, 0, &c, &selected))
	for _, want := range []string{"System position 0 id=0", "System position 1 id=0", "reciprocal sector/system IDs agree", "sector/system reference mismatch", "system.sector=not recorded", "system.sector=unsupported", "Referenced system record unavailable", "System 5: system.sector points here; sector.systems membership=missing reverse reference", byIntWidth("Unsupported system.sector fields=2", "Unsupported system.sector fields=3"), byIntWidth("undecodable references=2 (not included in the counts above)", "undecodable references=3 (not included in the counts above)"), "Possible colony ID match: 0", "Possible planet ID match: 0 Candidate planet", "namespace unverified; ID equality does not establish capital identity"} {
		if !strings.Contains(details, want) {
			t.Fatalf("missing %q in %s", want, details)
		}
	}
	selected = 1
	if !strings.Contains(resultText(renderSectors(gs, 0, &c, &selected)), "No decoded colony/planet ID candidate") {
		t.Fatal("unresolved local capital hidden")
	}
	selected = 2
	if !strings.Contains(resultText(renderSectors(gs, 0, &c, &selected)), "System 6: system.sector points here; sector.systems membership=list not recorded") {
		t.Fatal("missing sector system list hidden")
	}
	selected = 3
	if !renderSectors(gs, 0, &c, &selected).IsError {
		t.Fatal("foreign sector selected without reference path")
	}
}

func TestSectorsOptionalOutputAndNoDefaultZeroPaths(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	for _, item := range []struct{ data, want string }{
		{`country={0={}}`, "Sector table not recorded"},
		{`country={0={sectors={owned={}}}} sectors={}`, "country.sectors.owned empty"},
		{`country={0={sectors={}}} sectors={}`, "country.sectors.owned not recorded"},
		{`country={0={sectors=unsupported}} sectors={0={owner=0 systems={bad}}}`, "country.sectors.owned unsupported or duplicate"},
		{`country={0={sectors={owned={0}}}} sectors={0={owner=0 systems={bad}}} galactic_object={1={sector=0}}`, "sector.systems membership=unsupported; use raw tools"},
	} {
		gs, err := gamestate.Load([]byte(item.data))
		if err != nil {
			t.Fatal(err)
		}
		c := gs.Country[0]
		var selected *int
		if strings.Contains(item.want, "sector.systems membership") {
			id := 0
			selected = &id
		}
		text := resultText(renderSectors(gs, 0, &c, selected))
		if !strings.Contains(text, item.want) {
			t.Fatalf("missing %q in %s", item.want, text)
		}
	}
	gs, err := gamestate.Load([]byte(`country={0={}} sectors={0={owner=invalid}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderSectors(gs, 0, &c, nil))
	if strings.Contains(text, "\nSector 0:") {
		t.Fatal("failed owner parsing created a country-zero path")
	}
}

func TestSectorsRegisteredHandlerAndInputValidation(t *testing.T) {
	oldLoc := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = oldLoc })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={sectors={owned={0}}}} sectors={0={owner=0 systems={0}}} galactic_object={0={sector=0}}`)}})
	s := server.NewMCPServer("test", "test")
	registerSectors(s)
	entry, ok := s.ListTools()["get_sectors"]
	if !ok || entry.Tool.Annotations.ReadOnlyHint == nil || !*entry.Tool.Annotations.ReadOnlyHint || entry.Tool.Annotations.DestructiveHint == nil || *entry.Tool.Annotations.DestructiveHint {
		t.Fatal("sector tool missing or annotations incorrect")
	}
	for _, key := range []string{"save_id", "country_id", "sector_id"} {
		if _, ok := entry.Tool.InputSchema.Properties[key]; !ok {
			t.Fatalf("missing input %q", key)
		}
	}
	for _, value := range []any{"0", nil, true, -1, 0.5, math.NaN(), math.Inf(1), float64(1 << 53)} {
		r, err := handleSectors(context.Background(), rawRequest(map[string]any{"sector_id": value}))
		if err != nil || !r.IsError {
			t.Fatal("invalid sector ID accepted")
		}
	}
	for _, args := range []map[string]any{nil, {"sector_id": 0}, {"country_id": 0, "sector_id": float64(0)}} {
		r, err := handleSectors(context.Background(), rawRequest(args))
		if err != nil || r.IsError || !strings.Contains(resultText(r), "Sector 0:") {
			t.Fatal("normal sector handler failed")
		}
	}
}
