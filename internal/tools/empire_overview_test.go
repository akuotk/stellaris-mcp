package tools

import (
	"context"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
)

func TestEmpireOverviewDistinguishesMissingFromZero(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={} 1={num_sapient_pops=0 fleet_size=0 used_naval_capacity=0 naval_capacity=0 starbase_capacity=0 empire_size=0}}`)}})
	for _, tc := range []struct {
		id    int
		wants []string
	}{{0, []string{"Population (raw save units): not recorded", "Fleet Size: not recorded", "not recorded used / not recorded maximum", "Starbase capacity: not recorded", "Empire Size: not recorded"}}, {1, []string{"Population (raw save units): 0", "Fleet Size: 0", "0 used / 0 maximum", "Starbase capacity: 0", "Empire Size: 0"}}} {
		result, err := handleEmpireOverview(context.Background(), rawRequest(map[string]any{"country_id": tc.id}))
		if err != nil || result.IsError {
			t.Fatal("overview failed")
		}
		text := resultText(result)
		for _, want := range tc.wants {
			if !strings.Contains(text, want) {
				t.Errorf("missing %q in %s", want, text)
			}
		}
		if !strings.Contains(text, "non-planet carriers excluded") {
			t.Fatal("planet count scope omitted")
		}
	}
}
func TestNavalCapacityPreservesSavedPrecision(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={used_naval_capacity=1.25 naval_capacity=7.5 fleet_size=0.75}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	if got := formatNavalCapacity(&c); got != "1.25 used / 7.5 maximum" {
		t.Fatal(got)
	}
	result, err := fleetSummary(gs, &c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resultText(result), "Fleet Size: 0.75") {
		t.Fatal(resultText(result))
	}
}

func TestEmpireOverviewBalanceSumIsOrderIndependent(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	// (1e16 + -1e16) + 1 is 1, but (1e16 + 1) + -1e16 is 0: the sum depends on category order.
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={budget={current_month={balance={a={energy=10000000000000000} b={energy=-10000000000000000} c={energy=1}}}}}}`)}})
	for i := 0; i < 50; i++ {
		result, err := handleEmpireOverview(context.Background(), rawRequest(map[string]any{}))
		if err != nil || result.IsError {
			t.Fatal("overview failed")
		}
		if text := resultText(result); !strings.Contains(text, "[energy]=1\n") {
			t.Fatalf("run %d: balance not summed in sorted category order: %s", i, text)
		}
	}
}
