package gamestate

import (
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

func TestRelicsPreserveRawOrderAndMissingValues(t *testing.T) {
	gs, err := Load([]byte(`country={0={relics={r_first r_second r_first ""} last_activated_relic=0 last_received_relic=unknown}
	1={relics={}} 2={} 3={relics=r_single last_received_relic=""}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	if !c.Relics.Recorded || c.Relics.Unsupported || !reflect.DeepEqual(c.Relics.Values, []string{"r_first", "r_second", "r_first", ""}) {
		t.Fatal("saved relic order, duplicates or empty key lost")
	}
	if !c.LastActivatedRelic.Recorded || c.LastActivatedRelic.Value != "0" || !c.LastReceivedRelic.Recorded || c.LastReceivedRelic.Value != "unknown" {
		t.Fatal("raw last-field values lost")
	}
	if !gs.Country[1].Relics.Recorded || len(gs.Country[1].Relics.Values) != 0 || gs.Country[2].Relics.Recorded || gs.Country[2].LastActivatedRelic.Recorded || gs.Country[2].LastReceivedRelic.Recorded {
		t.Fatal("missing and empty states conflated")
	}
	if !reflect.DeepEqual(gs.Country[3].Relics.Values, []string{"r_single"}) || !gs.Country[3].LastReceivedRelic.Recorded || gs.Country[3].LastReceivedRelic.Value != "" {
		t.Fatal("singleton or explicit empty scalar lost")
	}
}

func TestRelicsMalformedAndDuplicateFieldsStayIsolated(t *testing.T) {
	for _, raw := range []string{
		`relics={r_first unknown={nested=yes}} last_activated_relic={bad=yes} last_received_relic={bad=yes}`,
		`relics={r_first} relics={r_second} last_activated_relic=0 last_activated_relic=1 last_received_relic=0 last_received_relic=1`,
	} {
		gs, err := Load([]byte(`date=2201.01.01 country={0={` + raw + `
		 adjective={key="Synthetic country"} num_sapient_pops=7 military_power=0
		 owned_leaders={0 2} ruler=0 ruler_title=synthetic_title
		 fleets_manager={owned_fleets={{fleet=0}}} sectors={owned={0}}
		 modules={standard_economy_module={resources={energy=0}} standard_grand_archive_module={exhibits={}}}
		 }}`))
		if err != nil || gs.Date != "2201.01.01" {
			t.Fatal("optional relic fields prevented whole-save loading")
		}
		c := gs.Country[0]
		if !c.Relics.Unsupported || len(c.Relics.Values) != 0 || !c.LastActivatedRelic.Unsupported || !c.LastReceivedRelic.Unsupported {
			t.Fatal("invalid or duplicate relic field remained authoritative")
		}
		if c.Adjective.Key != "Synthetic country" || c.NumPops != 7 || c.MilitaryPowerRaw == nil || *c.MilitaryPowerRaw != 0 || len(c.FleetsManager.OwnedFleets) != 1 || c.FleetsManager.OwnedFleets[0].FleetID == nil || *c.FleetsManager.OwnedFleets[0].FleetID != 0 || !reflect.DeepEqual(c.SectorRefs.Owned.Values, []int{0}) || c.EconomyModules.Economy == nil || len(c.EconomyModules.Economy.Resources.Entries) != 1 || c.GrandArchiveModules.Archive == nil || c.GrandArchiveModules.Archive.Exhibits == nil {
			t.Fatal("existing country fields changed under relic errors")
		}
		if !reflect.DeepEqual(c.OwnedLeaders, []int{0, 2}) || c.RulerRef.Value == nil || *c.RulerRef.Value != 0 || c.RulerTitle.Value != "synthetic_title" {
			t.Fatal("saved leadership references changed under relic errors")
		}
	}
	gs, err := Load([]byte(`country={0={relics={r_example} last_activated_relic=0 last_activated_relic=1 last_received_relic=2}}`))
	if err != nil || gs.Country[0].Relics.Unsupported || !gs.Country[0].LastActivatedRelic.Unsupported || gs.Country[0].LastReceivedRelic.Unsupported || gs.Country[0].LastReceivedRelic.Value != "2" {
		t.Fatal("one duplicate relic field affected its neighbors")
	}
	if _, err := Load([]byte(`country={0={relics={r_example} military_power=bad}}`)); err == nil {
		t.Fatal("existing country field error was swallowed")
	}
}

// This opt-in survey compares all other Country fields and logs aggregate counts only.
func TestRelicsLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local relic coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	type countryWithoutHook Country
	decoded, compared, lists, relics, activated, received, activatedIntegers, receivedIntegers, unsupported := 0, 0, 0, 0, 0, 0, 0, 0, 0
	for index, file := range files {
		data, err := extractGamestate(file.Path)
		if err != nil {
			t.Fatalf("save index %d read failed", index)
		}
		gs, err := Load(data)
		if err != nil {
			t.Fatalf("save index %d whole-save decoding failed", index)
		}
		decoded++
		var baseline struct {
			Country map[int]countryWithoutHook `clausewitz:"country"`
		}
		if err := clausewitz.Unmarshal(data, &baseline); err != nil || len(gs.Country) != len(baseline.Country) {
			t.Fatalf("save index %d baseline country decoding failed or changed count", index)
		}
		for id, c := range gs.Country {
			before, ok := baseline.Country[id]
			before.Relics = c.Relics
			before.LastActivatedRelic = c.LastActivatedRelic
			before.LastReceivedRelic = c.LastReceivedRelic
			// The compatibility slice is derived from the independently decoded reference list.
			before.OwnedLeaders = before.OwnedLeaderRefs.Values
			if !ok || !reflect.DeepEqual(c, Country(before)) {
				t.Fatalf("save index %d unrelated country fields changed", index)
			}
			compared++
			if c.Relics.Unsupported || c.LastActivatedRelic.Unsupported || c.LastReceivedRelic.Unsupported {
				unsupported++
			}
			if c.Relics.Recorded {
				lists++
			}
			relics += len(c.Relics.Values)
			if c.LastActivatedRelic.Recorded {
				activated++
				if _, err := strconv.ParseInt(c.LastActivatedRelic.Value, 10, 64); err == nil {
					activatedIntegers++
				}
			}
			if c.LastReceivedRelic.Recorded {
				received++
				if _, err := strconv.ParseInt(c.LastReceivedRelic.Value, 10, 64); err == nil {
					receivedIntegers++
				}
			}
		}
	}
	t.Logf("local relic coverage: saves=%d decoded=%d countries_with_other_fields_unchanged=%d recorded_relic_lists=%d relic_entries=%d last_activated_fields=%d last_received_fields=%d last_activated_integer_syntax=%d last_received_integer_syntax=%d unsupported_countries=%d", len(files), decoded, compared, lists, relics, activated, received, activatedIntegers, receivedIntegers, unsupported)
	if unsupported > 0 {
		t.Fatal("local relic field shapes need additional support")
	}
}
