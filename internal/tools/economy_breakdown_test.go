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
