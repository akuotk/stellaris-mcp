package gamestate

import (
	"os"
	"testing"
)

func TestCasusBelliKeepsRawOrderMissingAndZeroValues(t *testing.T) {
	gs, err := Load([]byte(`country={0={modules={standard_diplomacy_module={casus_belli={{owner=0 country=0 type=example days_left=0} {owner=0 country=1 type=example days_left=-1} {type=""}}} exclusive_diplomacy_module={casus_belli={}}}}
	1={modules={standard_diplomacy_module={}}} 2={modules={}} 3={}}`))
	if err != nil {
		t.Fatal(err)
	}
	modules := gs.Country[0].DiplomaticModules
	if !modules.Recorded || modules.Unsupported || !modules.Standard.Recorded || modules.Standard.Unsupported || !modules.Standard.CasusBelli.Recorded || modules.Standard.CasusBelli.Unsupported || len(modules.Standard.CasusBelli.Items) != 3 {
		t.Fatal("supported diplomacy shape rejected")
	}
	first, second, missing := modules.Standard.CasusBelli.Items[0], modules.Standard.CasusBelli.Items[1], modules.Standard.CasusBelli.Items[2]
	if first.Owner == nil || *first.Owner != 0 || first.Country == nil || *first.Country != 0 || *first.DaysLeft != "0" || *second.Country != 1 || *second.DaysLeft != "-1" || missing.Owner != nil || missing.Country != nil || missing.DaysLeft != nil || missing.Type == nil || *missing.Type != "" {
		t.Fatal("raw values, order or missing fields lost")
	}
	if !modules.Exclusive.Recorded || !modules.Exclusive.CasusBelli.Recorded || len(modules.Exclusive.CasusBelli.Items) != 0 || gs.Country[1].DiplomaticModules.Standard.CasusBelli.Recorded || gs.Country[2].DiplomaticModules.Standard.Recorded || gs.Country[3].DiplomaticModules.Recorded {
		t.Fatal("empty and missing list/module shapes conflated")
	}
}

func TestCasusBelliUnsupportedAndDuplicateShapesStayIsolated(t *testing.T) {
	for _, module := range []string{
		`standard_diplomacy_module=none`,
		`standard_diplomacy_module={casus_belli=none}`,
		`standard_diplomacy_module={casus_belli={unexpected={}}}`,
		`standard_diplomacy_module={casus_belli={} casus_belli={}}`,
		`standard_diplomacy_module={casus_belli={{owner=0} {country=invalid}}}`,
		`standard_diplomacy_module={casus_belli={{owner=0 owner=1}}}`,
		`standard_diplomacy_module={casus_belli={{country=0 country=1}}}`,
		`standard_diplomacy_module={casus_belli={{type=one type=two}}}`,
		`standard_diplomacy_module={casus_belli={{days_left=0 days_left=1}}}`,
		`standard_diplomacy_module={casus_belli={{days_left={nested=yes}}}}`,
	} {
		gs, err := Load([]byte(`date=2201.01.01 country={0={modules={` + module + ` exclusive_diplomacy_module={casus_belli={{owner=0 country=0}}} standard_economy_module={resources={energy=0}}}}}`))
		if err != nil || gs.Date != "2201.01.01" {
			t.Fatal("optional diplomacy parsing prevented unrelated save load")
		}
		c := gs.Country[0]
		standard := c.DiplomaticModules.Standard
		if !standard.Unsupported && !standard.CasusBelli.Unsupported || len(standard.CasusBelli.Items) != 0 {
			t.Fatal("invalid or duplicate record remained usable")
		}
		if c.EconomyModules.Unsupported || c.EconomyModules.Economy == nil || c.EconomyModules.Economy.Resources.Entries[0].Amount != "0" || len(c.DiplomaticModules.Exclusive.CasusBelli.Items) != 1 {
			t.Fatal("invalid standard diplomacy affected economy or exclusive module")
		}
	}
	for _, modules := range []string{`none`, `{standard_diplomacy_module={} standard_diplomacy_module={}}`, `{exclusive_diplomacy_module={} exclusive_diplomacy_module={}}`} {
		gs, err := Load([]byte(`date=2201.01.01 country={0={modules=` + modules + `}}`))
		if err != nil || gs.Date != "2201.01.01" || !gs.Country[0].DiplomaticModules.Unsupported {
			t.Fatal("duplicate or invalid diplomacy modules unreported")
		}
	}
	gs, err := Load([]byte(`country={0={modules={standard_economy_module=none standard_diplomacy_module={casus_belli={{owner=0 country=0}}}}}}`))
	if err != nil || !gs.Country[0].EconomyModules.Unsupported || len(gs.Country[0].DiplomaticModules.Standard.CasusBelli.Items) != 1 {
		t.Fatal("invalid economy affected valid diplomacy records")
	}
}

// This opt-in survey reads every local save and logs aggregate counts only.
func TestCasusBelliLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local casus_belli coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	decoded, standardModules, exclusiveModules, standardEntries, exclusiveEntries, matchingOwner, differingOwner, missingOwner, missingCountry, unresolvedOwner, unresolvedCountry, missingDays, unsupported := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("catalog save index %d whole-save decoding failed", index)
		}
		decoded++
		for countryID, c := range gs.Country {
			modules := c.DiplomaticModules
			if modules.Unsupported {
				unsupported++
			}
			if modules.Standard.Recorded {
				standardModules++
			}
			if modules.Exclusive.Recorded {
				exclusiveModules++
			}
			standardEntries += len(modules.Standard.CasusBelli.Items)
			exclusiveEntries += len(modules.Exclusive.CasusBelli.Items)
			for _, module := range []CasusBelliModule{modules.Standard, modules.Exclusive} {
				if module.Unsupported || module.CasusBelli.Unsupported {
					unsupported++
				}
				for _, entry := range module.CasusBelli.Items {
					if entry.Owner == nil {
						missingOwner++
					} else {
						if *entry.Owner == countryID {
							matchingOwner++
						} else {
							differingOwner++
						}
						if _, ok := gs.Country[*entry.Owner]; !ok {
							unresolvedOwner++
						}
					}
					if entry.Country == nil {
						missingCountry++
					} else if _, ok := gs.Country[*entry.Country]; !ok {
						unresolvedCountry++
					}
					if entry.DaysLeft == nil {
						missingDays++
					}
				}
			}
		}
	}
	t.Logf("local casus_belli coverage: saves=%d decoded=%d standard_modules=%d exclusive_modules=%d standard_entries=%d exclusive_entries=%d owner_matches_containing_country=%d owner_differs=%d missing_owner=%d missing_country=%d unresolved_owner=%d unresolved_country=%d missing_days_left=%d unsupported_shapes=%d", len(files), decoded, standardModules, exclusiveModules, standardEntries, exclusiveEntries, matchingOwner, differingOwner, missingOwner, missingCountry, unresolvedOwner, unresolvedCountry, missingDays, unsupported)
	if unsupported > 0 {
		t.Fatal("local casus_belli shapes need additional support")
	}
}
