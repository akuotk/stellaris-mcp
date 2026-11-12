package tools

import (
	"context"
	"fmt"
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

func TestEmpireOverviewShowsSavedResourceAmounts(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	const heading = "\nSaved resource amounts (economy module, saved order; stockpile candidates, units unverified):\n"
	for _, tc := range []struct{ country, want string }{
		// Missing at each level is reported as such, never as zero.
		{``, "  country.modules not recorded\n"},
		{`modules={}`, "  standard_economy_module not recorded\n"},
		{`modules={standard_economy_module={}}`, "  resources not recorded\n"},
		{`modules={standard_economy_module={resources={}}}`, "  Empty\n"},
		// Saved order and saved text are kept; a repeated key and a non-number are marked.
		{`modules={standard_economy_module={resources={minerals=12.50 energy=0 energy=-3 odd="a b"}}}`,
			"  Minerals [minerals]=12.50, Energy [energy]=0, Energy [energy]=-3 [duplicate saved resource key], Odd [odd]=\"a b\"\n"},
		{`modules={standard_economy_module={resources={energy=1 bad={x=1}}}}`,
			"  Unsupported resource entries; supported entries below may be partial, use raw tree/byte tools\n  Energy [energy]=1\n"},
		{`modules={standard_economy_module={resources={energy=1}} standard_economy_module={resources={energy=2}}}`,
			"  Unsupported modules shape or duplicate known fields; use raw tree/byte tools\n"},
	} {
		syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={` + tc.country + `}}`)}})
		result, err := handleEmpireOverview(context.Background(), rawRequest(map[string]any{}))
		if err != nil || result.IsError {
			t.Fatalf("overview failed for %q", tc.country)
		}
		text := resultText(result)
		if want := heading + tc.want + "\n"; !strings.Contains(text, want) {
			t.Errorf("country %q: want section %q in:\n%s", tc.country, want, text)
		}
	}
}

// The overview repeats what get_resource_market reads; the two must not drift.
func TestEmpireOverviewResourcesMatchResourceMarket(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`player={{country=0}} country={0={modules={standard_economy_module={resources={energy=1234.5 alloys=0 unity=876.125}}}}}`)}})
	overview, err := handleEmpireOverview(context.Background(), rawRequest(map[string]any{}))
	if err != nil || overview.IsError {
		t.Fatal("overview failed")
	}
	market, err := handleResourceMarket(context.Background(), rawRequest(map[string]any{}))
	if err != nil || market.IsError {
		t.Fatal("resource market failed")
	}
	for _, pair := range [][2]string{{"energy", "1234.5"}, {"alloys", "0"}, {"unity", "876.125"}} {
		if want := fmt.Sprintf("[%s]=%s", pair[0], pair[1]); !strings.Contains(resultText(overview), want) {
			t.Errorf("overview lacks %s", want)
		}
		if want := fmt.Sprintf("resource=%q amount=%q", pair[0], pair[1]); !strings.Contains(resultText(market), want) {
			t.Errorf("resource market lacks %s", want)
		}
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
