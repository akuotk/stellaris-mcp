package gamestate

import (
	"os"
	"testing"
)

func TestGrandArchivePreservesRawShapesAndOrder(t *testing.T) {
	gs, err := Load([]byte(`country={0={modules={standard_grand_archive_module={
	 exhibits={historical_item={specimen=0 specimen=2 unknown={nested={id=0}}} aesthetic_wonder={} historical_item={}}
	 auto_culling_settings={}
	 culling_value={{} {amount=0} none}
	 specimen_management_settings={auto_buy=unknown auto_buy=another mixed={key=yes anonymous}}
	 }}} 1={modules={standard_grand_archive_module={}}} 2={modules={}} 3={}}`))
	if err != nil {
		t.Fatal(err)
	}
	modules := gs.Country[0].GrandArchiveModules
	a := modules.Archive
	if !modules.Recorded || modules.Unsupported || a == nil || a.Unsupported {
		t.Fatal("supported archive module rejected")
	}
	if a.Exhibits == nil || !a.Exhibits.IsObject() || len(a.Exhibits.Object.Pairs) != 3 {
		t.Fatal("exhibit fields lost")
	}
	fields := a.Exhibits.Object.Pairs
	entries := fields[0].Value.Object.Pairs
	if fields[0].Key != "historical_item" || fields[2].Key != fields[0].Key || len(entries) != 3 || entries[0].Value.Scalar != "0" || entries[1].Key != "specimen" || entries[1].Value.Scalar != "2" || !entries[2].Value.IsObject() {
		t.Fatal("raw values, duplicate keys or nested exhibit shape lost")
	}
	if a.AutoCulling == nil || !a.AutoCulling.IsObject() || len(a.AutoCulling.Object.Pairs) != 0 || a.CullingValue == nil || !a.CullingValue.IsList() || len(a.CullingValue.List) != 3 || a.CullingValue.List[2].Scalar != "none" {
		t.Fatal("empty object, list positions or scalar values lost")
	}
	settings := a.SpecimenSettings.Object.Pairs
	if len(settings) != 3 || settings[0].Value.Scalar != "unknown" || settings[1].Value.Scalar != "another" || !settings[2].Value.Mixed {
		t.Fatal("raw settings or mixed marker lost")
	}
	if gs.Country[1].GrandArchiveModules.Archive == nil || gs.Country[1].GrandArchiveModules.Archive.Exhibits != nil || gs.Country[2].GrandArchiveModules.Archive != nil || gs.Country[3].GrandArchiveModules.Recorded {
		t.Fatal("missing and recorded-empty module states conflated")
	}
}

func TestGrandArchiveMalformedModulesStayIsolated(t *testing.T) {
	for _, raw := range []string{
		`standard_grand_archive_module=none`,
		`standard_grand_archive_module={} standard_grand_archive_module={}`,
		`standard_grand_archive_module={exhibits={} exhibits={}}`,
	} {
		gs, err := Load([]byte(`date=2201.01.01 country={0={modules={` + raw + `
		 standard_economy_module={resources={energy=0}}
		 standard_diplomacy_module={casus_belli={{owner=0 country=0}}}
		 standard_species_rights_module={species_rights={{species_index=0}}}
		 }}}`))
		if err != nil || gs.Date != "2201.01.01" {
			t.Fatal("optional archive prevented unrelated save loading")
		}
		c := gs.Country[0]
		modules := c.GrandArchiveModules
		if !modules.Unsupported && (modules.Archive == nil || !modules.Archive.Unsupported) {
			t.Fatal("invalid or duplicate archive wrapper accepted")
		}
		if c.EconomyModules.Economy == nil || len(c.EconomyModules.Economy.Resources.Entries) != 1 || len(c.DiplomaticModules.Standard.CasusBelli.Items) != 1 || len(c.SpeciesRightsModules.Rights.Entries.Items) != 1 {
			t.Fatal("invalid archive affected valid economy, diplomacy or species rights")
		}
	}
	gs, err := Load([]byte(`country={0={modules={standard_economy_module=none standard_diplomacy_module=none standard_species_rights_module=none
	 standard_grand_archive_module={exhibits={}}}}}`))
	if err != nil {
		t.Fatal("invalid neighboring modules prevented save loading")
	}
	c := gs.Country[0]
	if !c.EconomyModules.Unsupported || !c.DiplomaticModules.Standard.Unsupported || !c.SpeciesRightsModules.Rights.Unsupported || c.GrandArchiveModules.Archive == nil || c.GrandArchiveModules.Archive.Exhibits == nil {
		t.Fatal("invalid neighboring modules affected valid archive")
	}
}

// This opt-in survey reads every local save and logs aggregate counts only.
func TestGrandArchiveLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local Grand Archive coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	decoded, modules, emptyExhibits, categories, emptyCategories, emptyAutoCulling := 0, 0, 0, 0, 0, 0
	cullingLists, cullingItems, emptyCullingItems, scalarAutoBuy, unsupported := 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("save index %d whole-save decoding failed", index)
		}
		decoded++
		for _, c := range gs.Country {
			wrapper := c.GrandArchiveModules
			a := wrapper.Archive
			if wrapper.Unsupported || a != nil && a.Unsupported {
				unsupported++
				continue
			}
			if a == nil {
				continue
			}
			modules++
			if a.Exhibits != nil && a.Exhibits.IsObject() {
				pairs := a.Exhibits.Object.Pairs
				if len(pairs) == 0 {
					emptyExhibits++
				}
				categories += len(pairs)
				for _, pair := range pairs {
					if pair.Value.IsObject() && len(pair.Value.Object.Pairs) == 0 {
						emptyCategories++
					}
				}
			}
			if a.AutoCulling != nil && a.AutoCulling.IsObject() && len(a.AutoCulling.Object.Pairs) == 0 {
				emptyAutoCulling++
			}
			if a.CullingValue != nil && a.CullingValue.IsList() {
				cullingLists++
				cullingItems += len(a.CullingValue.List)
				for _, item := range a.CullingValue.List {
					if item.IsObject() && len(item.Object.Pairs) == 0 {
						emptyCullingItems++
					}
				}
			}
			if a.SpecimenSettings != nil && a.SpecimenSettings.IsObject() {
				for _, pair := range a.SpecimenSettings.Object.Pairs {
					if pair.Key == "auto_buy" && pair.Value.IsScalar() {
						scalarAutoBuy++
					}
				}
			}
		}
	}
	t.Logf("local Grand Archive coverage: saves=%d decoded=%d modules=%d empty_exhibit_objects=%d exhibit_category_entries=%d empty_categories=%d empty_auto_culling_objects=%d culling_lists=%d culling_list_items=%d empty_culling_items=%d scalar_auto_buy_fields=%d unsupported_modules=%d", len(files), decoded, modules, emptyExhibits, categories, emptyCategories, emptyAutoCulling, cullingLists, cullingItems, emptyCullingItems, scalarAutoBuy, unsupported)
	if unsupported > 0 {
		t.Fatal("local archive module shapes need additional support")
	}
}
