package gamestate

import (
	"os"
	"testing"
)

func TestLoadColonyPopulationAndZeroID(t *testing.T) {
	gs, err := Load([]byte(`
 planets={ planet={
  3={ colony=0 owner=0 planet_class="pc_arid" planet_size=19 name={key="Home"} }
  4={ planet_class="pc_arid" }
  5={ colony=1 owner=1 num_sapient_pops=9 }
  6={ owner=2 num_sapient_pops=12 stability=60 }
 } }
 colony={
  0={ carrier={type=planet reference=3} num_sapient_pops=4200 stability=63.25
      crime=12.75 free_amenities=-125.5 total_housing=8000 housing_usage=5000.25
      free_housing=2999.75 final_designation="col_capital" districts={0 1}
      pop_groups={99} produces={energy=100.5} profits={energy=80.25} }
  1={ carrier={type=planet reference=99} num_sapient_pops=999 }
 }
 pop_groups={99={key={species=1234567890 category="worker"} planet=3 size=4200
   last_month_growth=4 habitability=0.85 happiness=0.65} 100=none}
 `))
	if err != nil {
		t.Fatal(err)
	}
	p := gs.Planets.Planet[3]
	if p.ColonyID == nil || *p.ColonyID != 0 || p.OwnerID == nil || *p.OwnerID != 0 {
		t.Fatalf("zero-valued IDs lost: %+v", p)
	}
	if p.NumPops != 4200 || p.Stability != 63.25 || p.FreeAmenities != -125.5 || p.TotalHousing != 8000 || len(p.Districts) != 2 || p.Profits["energy"] != 80.25 {
		t.Fatalf("colony data not resolved: %+v", p)
	}
	if p.Name.Display() != "Home" || p.PlanetSize != 19 {
		t.Fatal("planet identity lost")
	}
	empty := gs.Planets.Planet[4]
	if empty.ColonyID != nil || empty.OwnerID != nil || empty.SurveyedByID != nil || empty.NumPops != 0 {
		t.Fatalf("missing values became zero IDs: %+v", empty)
	}
	if gs.Planets.Planet[5].NumPops != 9 {
		t.Fatal("mismatched carrier was joined")
	}
	if gs.Planets.Planet[6].NumPops != 12 || gs.Planets.Planet[6].Stability != 60 {
		t.Fatal("legacy inline colony stats lost")
	}
	group := gs.PopGroups[99]
	if group.Key.Species != 1234567890 || group.Size != 4200 || group.Happiness != 0.65 || group.LastMonthGrowth != 4 {
		t.Fatalf("POP group data lost: %+v", group)
	}
}

func TestLoadColonySave(t *testing.T) {
	path := os.Getenv("STELLARIS_TEST_SAVE")
	if path == "" {
		t.Skip("set STELLARIS_TEST_SAVE for read-only integration check")
	}
	gs, err := LoadFromZip(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(gs.Colonies) == 0 {
		t.Fatal("no colony records in integration save")
	}
	resolved := 0
	mismatches := 0
	for id, p := range gs.Planets.Planet {
		if p.ColonyID == nil {
			continue
		}
		c, ok := gs.Colonies[*p.ColonyID]
		if !ok || c.Carrier.Type != "planet" || c.Carrier.Reference != id {
			t.Fatalf("unresolved colony for planet %d", id)
		}
		if p.NumPops != c.NumPops {
			t.Fatalf("colony population not resolved for planet %d", id)
		}
		total := 0
		for _, gid := range p.PopGroups {
			group, ok := gs.PopGroups[gid]
			if !ok {
				t.Fatalf("missing pop group %d", gid)
			}
			total += group.Size
		}
		if total != p.NumPops {
			mismatches++
		}
		resolved++
	}
	if resolved == 0 {
		t.Fatal("no planets resolved")
	}
	for _, player := range gs.Player {
		c, ok := gs.Country[player.Country]
		if !ok {
			t.Fatal("missing player country")
		}
		count, total := 0, 0
		for _, p := range gs.Planets.Planet {
			if p.OwnerID != nil && *p.OwnerID == player.Country && p.ColonyID != nil {
				count++
				total += p.NumPops
			}
		}
		t.Logf("date=%s player_country=%d owned_colonies=%d population_raw=%d country_population_raw=%d", gs.Date, player.Country, count, total, c.NumPops)
	}
	speciesResolved, ethicsRecorded, factionsRecorded, ethicsMismatches, factionMismatches := 0, 0, 0, 0, 0
	for _, group := range gs.PopGroups {
		if s, ok := gs.Species[group.Key.Species]; ok && s.Name.Key != "" {
			speciesResolved++
		}
		if len(group.Ethos.Ethics) > 0 {
			ethicsRecorded++
		}
		if len(group.Factions.Objects) > 0 {
			factionsRecorded++
		}
		if len(group.Ethos.Ethics) != len(group.Ethos.Pops) {
			ethicsMismatches++
		}
		if len(group.Factions.Objects) != len(group.Factions.Pops) {
			factionMismatches++
		}
	}
	t.Logf("species=%d species_resolved_groups=%d ethics_recorded_groups=%d faction_recorded_groups=%d ethics_array_mismatches=%d faction_array_mismatches=%d political_factions=%d", len(gs.Species), speciesResolved, ethicsRecorded, factionsRecorded, ethicsMismatches, factionMismatches, len(gs.PopFactions))
	t.Logf("resolved_planet_colonies=%d colony_records=%d pop_groups=%d group_total_mismatches=%d", resolved, len(gs.Colonies), len(gs.PopGroups), mismatches)
}
