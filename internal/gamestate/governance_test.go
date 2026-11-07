package gamestate

import (
	"os"
	"testing"
)

func TestGovernancePreservesRawOrderAndMissingValues(t *testing.T) {
	gs, err := Load([]byte(`country={0={
	 active_policies={{policy=example_policy selected=option_a date=0} {policy=example_policy selected=option_b} {policy=""}}
	 edicts={{edict=example_edict date=0 start_date=2200.01.01 perpetual=unknown} {}}
	 policy_flags={example_flag example_flag ""}
	} 1={active_policies={} edicts={} policy_flags={}} 2={}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	if !c.ActivePolicies.Recorded || c.ActivePolicies.Unsupported || len(c.ActivePolicies.Items) != 3 || *c.ActivePolicies.Items[0].Date != "0" || *c.ActivePolicies.Items[1].Selected != "option_b" || c.ActivePolicies.Items[1].Date != nil || c.ActivePolicies.Items[2].Policy == nil || *c.ActivePolicies.Items[2].Policy != "" {
		t.Fatal("policy order, duplicate keys, raw values or missing fields lost")
	}
	if !c.Edicts.Recorded || c.Edicts.Unsupported || len(c.Edicts.Items) != 2 || *c.Edicts.Items[0].Perpetual != "unknown" || *c.Edicts.Items[0].Date != "0" || c.Edicts.Items[1].Edict != nil || c.Edicts.Items[1].Perpetual != nil {
		t.Fatal("edict values or missing fields lost")
	}
	if !c.PolicyFlags.Recorded || c.PolicyFlags.Unsupported || len(c.PolicyFlags.Values) != 3 || c.PolicyFlags.Values[0] != c.PolicyFlags.Values[1] || c.PolicyFlags.Values[2] != "" {
		t.Fatal("raw flag order or values lost")
	}
	empty, missing := gs.Country[1], gs.Country[2]
	if !empty.ActivePolicies.Recorded || !empty.Edicts.Recorded || !empty.PolicyFlags.Recorded || len(empty.ActivePolicies.Items) != 0 || len(empty.Edicts.Items) != 0 || len(empty.PolicyFlags.Values) != 0 || missing.ActivePolicies.Recorded || missing.Edicts.Recorded || missing.PolicyFlags.Recorded {
		t.Fatal("empty and missing collections conflated")
	}
}

func TestGovernanceUnsupportedShapesStayIsolated(t *testing.T) {
	for _, fields := range []string{
		`active_policies=none edicts=none policy_flags={unexpected={}}`,
		`active_policies={unexpected={}} edicts={unexpected={}} policy_flags={unexpected={}}`,
		`active_policies={{policy=valid selected=valid} {selected={nested=yes}}} edicts={{edict=valid} {perpetual={nested=yes}}} policy_flags={valid unexpected={}}`,
	} {
		gs, err := Load([]byte(`date=2201.01.01 country={0={num_sapient_pops=7 ` + fields + `} 1={active_policies={{policy=valid selected=valid}} edicts={{edict=valid}}}}`))
		if err != nil {
			t.Fatal("optional governance parsing prevented whole-save loading")
		}
		c := gs.Country[0]
		if !c.ActivePolicies.Unsupported || !c.Edicts.Unsupported || !c.PolicyFlags.Unsupported || len(c.ActivePolicies.Items) != 0 || len(c.Edicts.Items) != 0 || len(c.PolicyFlags.Values) != 0 {
			t.Fatal("unsupported collections displayed as complete or left partial values")
		}
		if gs.Date != "2201.01.01" || c.NumPops != 7 || len(gs.Country[1].ActivePolicies.Items) != 1 || len(gs.Country[1].Edicts.Items) != 1 {
			t.Fatal("unrelated save data lost")
		}
	}
}

// The opt-in survey reads every local save and reports aggregate counts only.
func TestGovernanceLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local governance coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	decoded, countries, policyLists, policies, edictLists, edicts, flagLists, flags, unsupported, missingPolicyFields, missingEdictFields := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("catalog save index %d whole-save decoding failed", index)
		}
		decoded++
		for _, c := range gs.Country {
			countries++
			if c.ActivePolicies.Recorded {
				policyLists++
			}
			if c.Edicts.Recorded {
				edictLists++
			}
			if c.PolicyFlags.Recorded {
				flagLists++
			}
			if c.ActivePolicies.Unsupported || c.Edicts.Unsupported || c.PolicyFlags.Unsupported {
				unsupported++
			}
			policies += len(c.ActivePolicies.Items)
			edicts += len(c.Edicts.Items)
			flags += len(c.PolicyFlags.Values)
			for _, p := range c.ActivePolicies.Items {
				if p.Policy == nil || p.Selected == nil {
					missingPolicyFields++
				}
			}
			for _, e := range c.Edicts.Items {
				if e.Edict == nil || e.Date == nil || e.StartDate == nil || e.Perpetual == nil {
					missingEdictFields++
				}
			}
		}
	}
	t.Logf("local governance coverage: saves=%d decoded=%d countries=%d recorded_policy_lists=%d policy_entries=%d recorded_edict_lists=%d edict_entries=%d recorded_flag_lists=%d flag_entries=%d unsupported_countries=%d policy_entries_missing_key_or_selection=%d edict_entries_missing_fields=%d", len(files), decoded, countries, policyLists, policies, edictLists, edicts, flagLists, flags, unsupported, missingPolicyFields, missingEdictFields)
	if unsupported > 0 {
		t.Fatal("local governance shapes need additional support")
	}
}
