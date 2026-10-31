package gamestate

import (
	"os"
	"testing"
)

func TestEspionageOperationsValuesAndOrderedLogs(t *testing.T) {
	gs, err := Load([]byte(`espionage_operations={operations={0={spy_network=0 type="synthetic_operation" index=0 last_roll=0 days_left=0 difficulty=0 info=0 target={type=country id=0 opener_id=0} log={{title="first" tooltip="first_tip" roll=0 skill=0 info=0 difficulty=0 date="2201.01.01"} {title="second" date="2201.01.02"}} flags={synthetic_flag=1234}} 1={} 8=none}}`))
	if err != nil {
		t.Fatal(err)
	}
	store := gs.EspionageOperations
	if store.TableIssue || len(store.DecodeErrors) > 0 || len(store.Items) != 2 {
		t.Fatal("supported operation records not retained")
	}
	op := store.Items[0]
	for _, v := range []*int{op.SpyNetwork, op.Index, op.Target.ID, op.Target.OpenerID} {
		if v == nil || *v != 0 {
			t.Fatal("zero reference lost")
		}
	}
	for _, v := range []*float64{op.LastRoll, op.DaysLeft, op.Difficulty, op.Info, op.Log[0].Roll, op.Log[0].Skill, op.Log[0].Info, op.Log[0].Difficulty} {
		if v == nil || *v != 0 {
			t.Fatal("zero raw value lost")
		}
	}
	if len(op.Log) != 2 || op.Log[0].Title != "first" || op.Log[1].Title != "second" || op.Flags["synthetic_flag"].Scalar == nil || *op.Flags["synthetic_flag"].Scalar != "1234" {
		t.Fatal("log order or flags lost")
	}
	missing := store.Items[1]
	if missing.SpyNetwork != nil || missing.DaysLeft != nil || missing.Target.ID != nil || missing.Log != nil {
		t.Fatal("missing values became zero")
	}
}

func TestEspionageOperationsUnsupportedDoesNotBreakSave(t *testing.T) {
	gs, err := Load([]byte(`date="2201.01.01" espionage_operations={operations={0={last_roll={unexpected=yes}} 1={spy_network=0} 1={spy_network=1} invalid_id={}}}`))
	if err != nil {
		t.Fatal("optional operation shape broke whole save")
	}
	if gs.Date != "2201.01.01" || !gs.EspionageOperations.TableIssue || len(gs.EspionageOperations.DecodeErrors) != 2 {
		t.Fatal("unsupported/duplicate records not flagged")
	}
	gs, err = Load([]byte(`espionage_operations=unsupported date="2201.01.02"`))
	if err != nil || !gs.EspionageOperations.TableIssue || gs.Date != "2201.01.02" {
		t.Fatal("unsupported table not isolated")
	}
}

func TestEspionageOperationsLocalCatalogCoverage(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for local read-only operation coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil {
		t.Fatal("save catalog read failed")
	}
	records, unsupported, networkRefs, resolvedNetworks, activeReciprocal, targetsComparable, targetsAgree, logEntries, missingNetwork := 0, 0, 0, 0, 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("save index %d whole-save parsing failed", index)
		}
		if gs.EspionageOperations.TableIssue {
			unsupported++
		}
		unsupported += len(gs.EspionageOperations.DecodeErrors)

		records += len(gs.EspionageOperations.Items)
		for id, op := range gs.EspionageOperations.Items {
			logEntries += len(op.Log)
			if op.SpyNetwork == nil {
				missingNetwork++
				continue
			}
			networkRefs++
			network, ok := gs.SpyNetworks[*op.SpyNetwork]
			if !ok {
				continue
			}
			resolvedNetworks++
			for _, ref := range network.ActiveOperations {
				if ref == id {
					activeReciprocal++
					break
				}
			}
			if op.Target.Type == "country" && op.Target.ID != nil && network.Target != nil {
				targetsComparable++
				if *op.Target.ID == *network.Target {
					targetsAgree++
				}
			}
		}
	}
	t.Logf("local operation coverage: saves=%d records=%d unsupported=%d network_refs=%d resolved_networks=%d active_reciprocal=%d country_targets_comparable=%d country_targets_agree=%d log_entries=%d missing_network=%d", len(files), records, unsupported, networkRefs, resolvedNetworks, activeReciprocal, targetsComparable, targetsAgree, logEntries, missingNetwork)
	if unsupported > 0 {
		t.Fatal("local operation shape needs additional support")
	}
}

func TestEspionageOperationStructuredFlags(t *testing.T) {
	gs, err := Load([]byte(`espionage_operations={operations={0={flags={synthetic_flag={flag_date="2201.01.01" flag_days=0}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(gs.EspionageOperations.DecodeErrors) > 0 {
		t.Fatal("structured flag rejected")
	}
	flag := gs.EspionageOperations.Items[0].Flags["synthetic_flag"]
	if flag.Scalar != nil || flag.FlagDate == nil || *flag.FlagDate != "2201.01.01" || flag.FlagDays == nil || *flag.FlagDays != "0" {
		t.Fatal("structured flag data lost")
	}
}
