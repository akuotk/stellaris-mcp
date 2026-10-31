package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestEspionageOperationsCrossReferencesAndRawDetails(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={adjective={key="Synthetic nation"}}}
 spy_networks={0={owner=0 target=1 active_operations={0 1 1 99}} 1={owner=0 target=2 active_operations={2}} 3={owner=4 target=5 active_operations={3}}}
 espionage_operations={operations={
  0={spy_network=0 type="synthetic" index=0 days_left=0 last_roll=0 difficulty=0 info=0 target={type=country id=1 opener_id=0} flags={synthetic_flag={flag_date="2201.01.01" flag_days=0} scalar_flag=0} log={{title="first" tooltip="first_tip" date="2201.01.01" roll=0 skill=0 info=0 difficulty=0} {title="second" date="2201.01.02"}}}
  1={spy_network=1 target={type=country id=9}}
  2={target={type=country id=0}}
  3={spy_network=3 target={type=country id=5}}
  4={spy_network=0 target={type=country id=1}}
  5={spy_network=88 target={type=country id=0}}
 }}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	summary := resultText(renderEspionageOperations(gs, 0, &c, nil))
	for _, want := range []string{"Operation 0:", "operation target.id and network.target agree", "Multiple active_operations references", "active_operations disagrees with operation.spy_network=1", "Target mismatch", "operation.spy_network is not recorded", "Operation 99: referenced record unavailable", "missing reverse reference", "Referenced network record unavailable"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in %s", want, summary)
		}
	}
	if strings.Contains(summary, "Operation 3:") {
		t.Fatal("unrelated operation included")
	}
	id := 0
	text := resultText(renderEspionageOperations(gs, 0, &c, &id))
	for _, want := range []string{"index=0 days_left=0", "last_roll=0 difficulty=0 info=0", "opener_id=0 (opener meaning unverified)", "Log entry 0 (saved order)", "roll=0 skill=0 info=0 difficulty=0", "title=first", "title=second", "Flag synthetic_flag: flag_date=2201.01.01 flag_days=0", "Flag scalar_flag=0"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Index(text, "title=first") > strings.Index(text, "title=second") || strings.Contains(text, "%") || strings.Contains(text, "opener country") {
		t.Fatal("raw order or unverified meaning changed")
	}
	id = 3
	if !renderEspionageOperations(gs, 0, &c, &id).IsError {
		t.Fatal("unrelated operation accepted")
	}
}

func TestEspionageOperationsMissingValuesAndUnsupportedWarning(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`spy_networks={0={owner=0 active_operations={0}}} espionage_operations={operations={0={spy_network=0 log={}} 1={difficulty={unsupported=yes}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	id := 0
	text := resultText(renderEspionageOperations(gs, 0, &gamestate.Country{}, &id))
	for _, want := range []string{"Operation decoding incomplete", "index=not recorded days_left=not recorded", "last_roll=not recorded difficulty=not recorded info=not recorded", "id=not recorded opener_id=not recorded", "Log: empty"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}
