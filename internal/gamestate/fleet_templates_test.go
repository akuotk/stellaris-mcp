package gamestate

import (
	"os"
	"testing"
)

func TestFleetTemplatesPreserveReferencesAndUnsupported(t *testing.T) {
	gs, err := Load([]byte(`country={0={fleet_template_manager={fleet_template={0 1 1}}}} fleet={0={fleet_template=0} 1={fleet_template=0 fleet_template=1}} fleet_template={0={fleet=0 fleet_size=0 reinforcement=0 fleet_template_design={{design=0 count=0} {ship_design_implementation={design=1 upgrade=0 growth_stage=0}}} home_base={orbitable={starbase=0}} all_queued={construction_queue_items={0 0}}} 1={fleet=invalid} 2=none}`))
	if err != nil {
		t.Fatal(err)
	}
	m := gs.Country[0].FleetTemplateRefs
	if !m.Recorded || m.Unsupported || len(m.Templates.Values) != 3 || m.Templates.Values[0] != 0 {
		t.Fatal("manager references lost")
	}
	table := gs.FleetTemplates
	if !table.Recorded || table.TableIssue || len(table.Items) != 1 || table.DecodeErrors[1] == "" {
		t.Fatal("table or invalid record handling failed")
	}
	v := table.Items[0]
	if v.Fleet == nil || *v.Fleet != 0 || len(v.Designs) != 2 || v.Designs[0].Count == nil || *v.Designs[0].Count != "0" || v.Designs[1].Count != nil || v.Designs[1].ShipDesign == nil || v.Designs[1].ShipDesign.Design == nil || *v.Designs[1].ShipDesign.Design != 1 || v.HomeBase == nil || v.HomeBase.Orbitable == nil || v.HomeBase.Orbitable.Starbase == nil || *v.HomeBase.Orbitable.Starbase != 0 || v.AllQueued == nil || len(v.AllQueued.ConstructionItems.Values) != 2 {
		t.Fatal("saved template data lost")
	}
	if gs.Fleet[0].TemplateRef.Value == nil || *gs.Fleet[0].TemplateRef.Value != 0 || !gs.Fleet[1].TemplateRef.Unsupported {
		t.Fatal("fleet back-reference handling failed")
	}
}
func TestFleetTemplatesLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("local catalog not set")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("catalog unavailable")
	}
	var saves, records, incomplete, managerRefs, fleetRefs int
	for _, f := range files {
		gs, err := LoadFromZip(f.Path)
		if err != nil {
			t.Fatal("save decode failed")
		}
		saves++
		if gs.FleetTemplates.TableIssue || len(gs.FleetTemplates.DecodeErrors) > 0 {
			incomplete++
		}
		records += len(gs.FleetTemplates.Items)
		for _, c := range gs.Country {
			managerRefs += len(c.FleetTemplateRefs.Templates.Values)
		}
		for _, fleet := range gs.Fleet {
			if fleet.TemplateRef.Value != nil {
				fleetRefs++
			}
		}
	}
	t.Logf("saves=%d records=%d incomplete_tables=%d manager_refs=%d fleet_refs=%d", saves, records, incomplete, managerRefs, fleetRefs)
	if incomplete > 0 {
		t.Fatal("local template shapes need support")
	}
}
