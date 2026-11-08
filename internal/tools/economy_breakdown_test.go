package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
)

func TestBudgetDisplaysSavedAndCalculatedData(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={budget={current_month={income={jobs={energy=3.125}} expenses={upkeep={energy=1.0001}} balance={net={energy=2.1249}}} last_month={income={}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	out := resultText(renderEconomyBreakdown(&c, "current", c.Budget.CurrentMonth))
	for _, want := range []string{"saved current month", "[jobs]", "[energy]=3.125", "[energy]=1.0001", "COMPUTED NET", "SAVED BALANCE CATEGORIES", "[energy]=2.1249"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
	last := resultText(renderEconomyBreakdown(&c, "last", c.Budget.LastMonth))
	for _, want := range []string{"saved last month", "Expense categories: not recorded", "Balance categories: not recorded", "Incomplete:"} {
		if !strings.Contains(last, want) {
			t.Errorf("missing %q in %s", want, last)
		}
	}
	missing := resultText(renderEconomyBreakdown(&c, "current", gamestate.BudgetMonth{}))
	if !strings.Contains(missing, "Budget month: not recorded") {
		t.Fatal(missing)
	}
}

func TestBudgetTradeExtraAndHistoryRemainSeparateFromCalculatedTotals(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={budget={
	current_month={income={jobs={energy=3}} expenses={upkeep={energy=1}}
	trade_income={treaty={energy=99.000 energy=-0.00} treaty={energy=7}} trade_expenses={oops=invalid}
	trade_balance={} trade_balance={duplicate={energy=9}} extra_income=bad extra_balance={}}
	last_month={trade_income={last_only={energy=5}}}
	current_month_commercial_pacts=00.000
	income_high_water_mark={length=02 current={energy=04.0} history={sample={energy=03.0} sample={energy=02.0}}}
	}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	text := resultText(renderEconomyBreakdown(&c, "current", c.Budget.CurrentMonth))
	for _, want := range []string{
		"COMPUTED NET (income minus expenses)",
		"[energy]: +2",
		`current_month_commercial_pacts="00.000"`,
		`Entry 0 key="treaty": "energy"="99.000" "energy"="-0.00" [duplicate saved resource key]`,
		`Entry 1 key="treaty" [duplicate saved key]: "energy"="7"`,
		`key="oops": unsupported saved shape or entries`,
		"trade_balance: duplicate saved fields",
		"extra_income: unsupported saved shape",
		"extra_balance: empty",
		`length="02" (saved value)`,
		`current: "energy"="04.0"`,
		`Entry 1 key="sample" [duplicate saved key]: "energy"="02.0"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in synthetic output: %s", want, text)
		}
	}
	last := resultText(renderEconomyBreakdown(&c, "last", c.Budget.LastMonth))
	if strings.Contains(text, "last_only") || !strings.Contains(last, "last_only") || strings.Contains(last, "treaty") {
		t.Fatal("saved month scopes were combined")
	}
}
