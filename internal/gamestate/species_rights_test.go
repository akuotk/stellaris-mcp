package gamestate

import (
	"os"
	"testing"
)

func TestSpeciesRightsRawOrderAndOptionalIndices(t *testing.T) {
	gs, err := Load([]byte(`country={0={modules={standard_species_rights_module={
	 enabled=unknown default={species_index=4294967295 citizenship=example former_citizenship=old last_changed_citizenship_type=0}
	 primary={} built_species={species_index=0}
	 species_rights={{species_index=0 citizenship=example citizenship=another last_changed_value=""}
	 {species_index=0 species_index=1 unknown={nested=yes} living_standard=raw}
	 {species_index=unknown migration_control=no} {} malformed}
	 }}} 1={modules={standard_species_rights_module={species_rights={}}}} 2={modules={}} 3={}}`))
	if err != nil {
		t.Fatal(err)
	}
	module := gs.Country[0].SpeciesRightsModules.Rights
	if !module.Recorded || module.Unsupported || !module.Enabled.Recorded || module.Enabled.Value != "unknown" || !module.Default.Recorded || !module.Primary.Recorded || len(module.Primary.Fields) != 0 || !module.Built.Recorded {
		t.Fatal("module state, raw flag or base records lost")
	}
	if len(module.Default.Fields) != 4 || module.Default.Fields[0] != (SpeciesRightsField{Key: "species_index", Value: "4294967295"}) || module.Default.Fields[3].Value != "0" {
		t.Fatal("default fields, numeric text or order lost")
	}
	if !module.Entries.Recorded || module.Entries.Unsupported || len(module.Entries.Items) != 5 {
		t.Fatal("species list entries or positions lost")
	}
	first, duplicateIndex, invalidIndex := module.Entries.Items[0], module.Entries.Items[1], module.Entries.Items[2]
	if first.SpeciesIndex.Value == nil || *first.SpeciesIndex.Value != 0 || !first.DuplicateFields || len(first.Fields) != 4 || first.Fields[1].Value != "example" || first.Fields[2].Value != "another" || first.Fields[3].Value != "" {
		t.Fatal("zero, duplicates or explicit empty scalar lost")
	}
	if !duplicateIndex.SpeciesIndex.Unsupported || duplicateIndex.SpeciesIndex.Value != nil || !duplicateIndex.Unsupported || len(duplicateIndex.Fields) != 3 || duplicateIndex.Fields[2].Key != "living_standard" {
		t.Fatal("duplicate index became a usable reference or supported neighbors were lost")
	}
	if !invalidIndex.SpeciesIndex.Unsupported || invalidIndex.SpeciesIndex.Value != nil || invalidIndex.Fields[0].Value != "unknown" || !module.Entries.Items[4].Unsupported {
		t.Fatal("invalid index or malformed record silently accepted")
	}
	if !gs.Country[1].SpeciesRightsModules.Rights.Entries.Recorded || len(gs.Country[1].SpeciesRightsModules.Rights.Entries.Items) != 0 || gs.Country[2].SpeciesRightsModules.Rights.Recorded || gs.Country[3].SpeciesRightsModules.Recorded {
		t.Fatal("missing and empty module/list states conflated")
	}
}

func TestSpeciesRightsMalformedModulesStayIsolated(t *testing.T) {
	for _, raw := range []string{
		`standard_species_rights_module=none`,
		`standard_species_rights_module={} standard_species_rights_module={}`,
		`standard_species_rights_module={default={} default={}}`,
		`standard_species_rights_module={species_rights={unexpected={}}}`,
		`standard_species_rights_module={species_rights=none}`,
		`standard_species_rights_module={species_rights={{species_index=0} unexpected=yes}}`,
	} {
		gs, err := Load([]byte(`date=2201.01.01 country={0={modules={` + raw + `
		 standard_economy_module={resources={energy=0}}
		 standard_diplomacy_module={casus_belli={{owner=0 country=0}}}
		 }}}`))
		if err != nil || gs.Date != "2201.01.01" {
			t.Fatal("optional rights module prevented whole-save loading")
		}
		c := gs.Country[0]
		modules := c.SpeciesRightsModules
		if !modules.Unsupported && !modules.Rights.Unsupported && !modules.Rights.Entries.Unsupported {
			t.Fatal("unsupported or duplicate wrapper was silently accepted")
		}
		if c.EconomyModules.Economy == nil || len(c.EconomyModules.Economy.Resources.Entries) != 1 || len(c.DiplomaticModules.Standard.CasusBelli.Items) != 1 {
			t.Fatal("invalid rights module affected valid economy or diplomacy")
		}
	}
	gs, err := Load([]byte(`country={0={modules={standard_economy_module=none standard_diplomacy_module=none
	 standard_species_rights_module={species_rights={{species_index=0}}}}}}`))
	if err != nil || !gs.Country[0].EconomyModules.Unsupported || !gs.Country[0].DiplomaticModules.Standard.Unsupported || len(gs.Country[0].SpeciesRightsModules.Rights.Entries.Items) != 1 {
		t.Fatal("invalid economy or diplomacy affected valid rights")
	}
}

// This opt-in survey reads every local save and logs aggregate counts only.
func TestSpeciesRightsLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local species rights coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	decoded, modules, defaults, primaries, built, lists, rows, fields := 0, 0, 0, 0, 0, 0, 0, 0
	candidates, unresolved, missingIndex, duplicateRecords, unsupported := 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("save index %d whole-save decoding failed", index)
		}
		decoded++
		countRecord := func(record SpeciesRightsRecord) {
			if !record.Recorded {
				return
			}
			fields += len(record.Fields)
			if record.Unsupported || record.SpeciesIndex.Unsupported {
				unsupported++
			}
			if record.DuplicateFields {
				duplicateRecords++
			}
			if record.SpeciesIndex.Value == nil {
				missingIndex++
			} else if _, ok := gs.Species.Items[*record.SpeciesIndex.Value]; ok {
				candidates++
			} else {
				unresolved++
			}
		}
		for _, c := range gs.Country {
			module := c.SpeciesRightsModules.Rights
			if c.SpeciesRightsModules.Unsupported || module.Unsupported || module.Enabled.Unsupported || module.Entries.Unsupported {
				unsupported++
			}
			if module.Recorded {
				modules++
			}
			if module.Default.Recorded {
				defaults++
			}
			if module.Primary.Recorded {
				primaries++
			}
			if module.Built.Recorded {
				built++
			}
			if module.Entries.Recorded {
				lists++
			}
			countRecord(module.Default)
			countRecord(module.Primary)
			countRecord(module.Built)
			rows += len(module.Entries.Items)
			for _, record := range module.Entries.Items {
				countRecord(record)
			}
		}
	}
	t.Logf("local species rights coverage: saves=%d decoded=%d modules=%d defaults=%d primaries=%d built_species=%d lists=%d species_records=%d scalar_fields=%d species_id_candidates=%d unresolved_species_indices=%d missing_species_indices=%d duplicate_records=%d unsupported_shapes=%d", len(files), decoded, modules, defaults, primaries, built, lists, rows, fields, candidates, unresolved, missingIndex, duplicateRecords, unsupported)
	if unsupported > 0 {
		t.Fatal("local species rights shapes need additional support")
	}
}
