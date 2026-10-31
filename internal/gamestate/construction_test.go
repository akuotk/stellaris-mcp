package gamestate

import (
	"os"
	"testing"
)

func TestConstructionReferencesAndMissingValues(t *testing.T) {
	gs, err := Load([]byte(`construction={queue_mgr={queues={0={owner=0 location={type=0 id=0} simultaneous=0 type="synthetic" disabled=no items={0 2 2}} 9=none}} item_mgr={items={
  0={queue=0 paying_country=0 progress=0 progress_needed=10 resources={energy=0.0001234} buildable_planet_building={building="example_building" planet=0 zone=0}}
  1={}
  2={queue=0 paying_country=0 buildable_ship={ship_design_implementation={design=0 upgrade=0 growth_stage=0} orbitable={starbase=0}}}
 }}}`))
	if err != nil {
		t.Fatal(err)
	}
	if gs.Construction.DecodeError != "" {
		t.Fatal("supported shape incorrectly rejected")
	}
	q := gs.Construction.QueueManager.Queues[0]
	if len(gs.Construction.QueueManager.Queues) != 1 || q.Owner == nil || *q.Owner != 0 || q.Location.Type == nil || *q.Location.Type != 0 || q.Simultaneous == nil || *q.Simultaneous != 0 || q.Disabled == nil || *q.Disabled || len(q.Items) != 3 || q.Items[2] != 2 {
		t.Fatal("queue references, order or zero values lost")
	}
	item := gs.Construction.ItemManager.Items[0]
	if item.Queue == nil || *item.Queue != 0 || item.PayingCountry == nil || *item.PayingCountry != 0 || item.Progress == nil || *item.Progress != 0 || item.Resources["energy"] != 0.0001234 || item.Building.Planet == nil || *item.Building.Planet != 0 {
		t.Fatal("item pointers, resource precision or payload lost")
	}
	if gs.Construction.ItemManager.Items[1].Progress != nil || gs.Construction.ItemManager.Items[1].Queue != nil {
		t.Fatal("missing fields became zero")
	}
	ship := gs.Construction.ItemManager.Items[2].Ship
	if ship.ShipDesign.Design == nil || *ship.ShipDesign.Design != 0 || ship.Orbitable.Starbase == nil {
		t.Fatal("nested ship payload references lost")
	}
}

func TestConstructionUnsupportedShapeDoesNotBreakOtherSaveData(t *testing.T) {
	gs, err := Load([]byte(`date="2201.01.01" construction={queue_mgr={queues={0={owner=0}}} item_mgr={items={0={resources={unexpected={nested=yes}}}}}} country={0={num_sapient_pops=7}}`))
	if err != nil {
		t.Fatal("unsupported optional construction broke whole save")
	}
	if gs.Date != "2201.01.01" || gs.Country[0].NumPops != 7 || gs.Construction.DecodeError == "" {
		t.Fatal("unsupported shape not flagged or unrelated data lost")
	}
}

func TestConstructionLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local construction coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil {
		t.Fatal("save catalog read failed")
	}
	decoded, incomplete, queueCount, itemCount, resolvedBackRefs, missingBackRefs := 0, 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("catalog save index %d whole-save decoding failed", index)
		}
		decoded++
		if gs.Construction.DecodeError != "" {
			incomplete++
			continue
		}
		queues, items := gs.Construction.QueueManager.Queues, gs.Construction.ItemManager.Items
		queueCount += len(queues)
		itemCount += len(items)
		for id, item := range items {
			if item.Queue == nil {
				missingBackRefs++
				continue
			}
			q, ok := queues[*item.Queue]
			if !ok {
				missingBackRefs++
				continue
			}
			membership := false
			for _, ref := range q.Items {
				if ref == id {
					membership = true
				}
			}
			if membership {
				resolvedBackRefs++
			} else {
				missingBackRefs++
			}
		}
	}
	t.Logf("local construction coverage: saves=%d decoded=%d incomplete_construction=%d queues=%d items=%d reciprocal_item_refs=%d unresolved_or_missing_membership=%d", len(files), decoded, incomplete, queueCount, itemCount, resolvedBackRefs, missingBackRefs)
	if incomplete > 0 {
		t.Fatal("local construction shape needs additional support")
	}
}
