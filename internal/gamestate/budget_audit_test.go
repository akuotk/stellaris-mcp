package gamestate

import (
	"math"
	"os"
	"testing"
)

func TestLocalBudgetArithmeticSurvey(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("local catalog not set")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("catalog unavailable")
	}
	var countries, compared, minusMatches, plusMatches, neither, positiveExpenses, negativeExpenses, zeroExpenses int
	for _, f := range files {
		gs, e := LoadFromZip(f.Path)
		if e != nil {
			t.Fatal("save decoding failed")
		}
		for _, c := range gs.Country {
			countries++
			m := c.Budget.CurrentMonth
			in, ex, bal := map[string]float64{}, map[string]float64{}, map[string]float64{}
			for _, r := range m.Income {
				for k, v := range r {
					in[k] += v
				}
			}
			for _, r := range m.Expenses {
				for k, v := range r {
					ex[k] += v
					if v > 0 {
						positiveExpenses++
					} else if v < 0 {
						negativeExpenses++
					} else {
						zeroExpenses++
					}
				}
			}
			for _, r := range m.Balance {
				for k, v := range r {
					bal[k] += v
				}
			}
			for k, v := range bal {
				compared++
				minus := math.Abs((in[k]-ex[k])-v) < 0.001
				plus := math.Abs((in[k]+ex[k])-v) < 0.001
				if minus {
					minusMatches++
				}
				if plus {
					plusMatches++
				}
				if !minus && !plus {
					neither++
				}
			}
		}
	}
	t.Logf("countries=%d compared=%d income_minus_expense_match=%d income_plus_expense_match=%d neither=%d positive_expense_entries=%d negative_expense_entries=%d zero_expense_entries=%d", countries, compared, minusMatches, plusMatches, neither, positiveExpenses, negativeExpenses, zeroExpenses)
}
