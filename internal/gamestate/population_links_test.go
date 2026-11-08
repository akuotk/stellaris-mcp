package gamestate

import (
	"os"
	"testing"
)

func TestSpeciesAndColonyReferenceShapesStayIsolated(t *testing.T) {
	gs, err := Load([]byte(`country={0={founder_species_ref=0 built_species_ref=0 built_species_ref=1 owned_species_refs={0 0} enslaved_species_refs=0}}
	species_db={0={base_ref=0 home_planet={type=planet reference=0}} 1={home_planet=0} 2={base_ref=0 base_ref=1 home_planet={type=planet type=unknown reference=0}} 3={} 3=none}
	colony={0={species_refs={0} species_information={0={num_pops=00 num_enslaved=0} 1={num_pops=0} 1=none} colonizing_species={unknown=yes}}}
	pop_groups={0={planet=0 month_start_size=00 last_month_growth=-0} 1={planet=0 planet=1} 2={planet={unknown=yes}}}
	pop_jobs={0={planet=0} 1={planet=0 planet=1} 2={planet={unknown=yes}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	if c.FounderSpeciesRef.Value == nil || *c.FounderSpeciesRef.Value != 0 || !c.BuiltSpeciesRef.Duplicate ||
		len(c.OwnedSpeciesRefs.Values) != 2 || !c.EnslavedSpeciesRefs.Unsupported {
		t.Fatal("country species references lost zero, repeated entries or unsupported status")
	}
	s0, s1, s2 := gs.Species.Items[0], gs.Species.Items[1], gs.Species.Items[2]
	if !gs.Species.Recorded || len(gs.Species.DecodeErrors) != 1 || s0.BaseRef.Value == nil ||
		s0.Home.CarrierType == nil || *s0.Home.CarrierType != "planet" || s0.Home.Reference.Value == nil ||
		!s1.Home.Saved.IsScalar() || s1.Home.Reference.Value == nil || !s2.BaseRef.Duplicate || !s2.Home.Duplicate {
		t.Fatal("species base/home fields or duplicate isolation lost")
	}
	colony := gs.Colonies[0]
	row := colony.SpeciesInformation.Items[0]
	if row.NumPops == nil || *row.NumPops != "00" || len(colony.SpeciesInformation.DecodeErrors) != 1 || !colony.ColonizingSpecies.Unsupported {
		t.Fatal("colony species information lost raw values or error isolation")
	}
	if g := gs.PopGroups[0]; g.PlanetRef.Value == nil || *g.PlanetRef.Value != 0 || g.MonthStartSize.Value != "00" ||
		g.LastMonthGrowthRaw == nil || *g.LastMonthGrowthRaw != "-0" || !gs.PopGroups[1].PlanetRefDuplicate || !gs.PopGroups[2].PlanetRef.Unsupported {
		t.Fatal("group back-reference or raw growth values lost")
	}
	if j := gs.PopJobs[0]; j.PlanetID == nil || *j.PlanetID != 0 || !gs.PopJobs[1].PlanetRefDuplicate ||
		gs.PopJobs[1].PlanetID != nil || !gs.PopJobs[2].PlanetRef.Unsupported {
		t.Fatal("job back-reference lost or an ambiguous ID became authoritative")
	}
	gs, err = Load([]byte(`date=2201.01.01 species_db={0={}} species_db={}`))
	if err != nil || gs.Date != "2201.01.01" || !gs.Species.TableIssue || len(gs.Species.Items) != 0 {
		t.Fatal("duplicate species table remained authoritative or affected other data")
	}
}

func TestPopulationLinksLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local population coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	var species, baseLinks, baseMatches, scalarHome, scalarMatches, typedHome, typedMatches, colonyTables, colonyInfo, unsupported int
	var groupRefs, groupMatches, jobRefs, jobMatches, countrySpeciesRefs, countrySpeciesMatches, colonySpeciesRefs, colonySpeciesMatches int
	for _, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatal("whole-save decoding failed")
		}
		if gs.Species.TableIssue {
			unsupported++
		}
		unsupported += len(gs.Species.DecodeErrors)
		for _, s := range gs.Species.Items {
			species++
			if s.BaseRef.Unsupported || s.BaseRef.Duplicate || s.Home.Unsupported || s.Home.Duplicate || s.Home.Reference.Unsupported {
				unsupported++
			}
			if s.BaseRef.Value != nil {
				baseLinks++
				if _, ok := gs.Species.Items[*s.BaseRef.Value]; ok {
					baseMatches++
				}
			}
			if s.Home.Reference.Value != nil {
				_, found := gs.Planets.Planet[*s.Home.Reference.Value]
				if s.Home.Saved.IsScalar() {
					scalarHome++
					if found {
						scalarMatches++
					}
				} else if s.Home.CarrierType != nil && *s.Home.CarrierType == "planet" {
					typedHome++
					if found {
						typedMatches++
					}
				}
			}
		}
		countRefs := func(single []SpeciesRef, lists []SpeciesRefList, total, matched *int) {
			count := func(id int) {
				(*total)++
				if _, ok := gs.Species.Items[id]; ok {
					(*matched)++
				}
			}
			for _, ref := range single {
				if ref.Duplicate || ref.Unsupported {
					unsupported++
				}
				if ref.Value != nil {
					count(*ref.Value)
				}
			}
			for _, list := range lists {
				if list.Duplicate || list.Unsupported {
					unsupported++
				}
				for _, id := range list.Values {
					count(id)
				}
			}
		}
		for _, c := range gs.Country {
			countRefs([]SpeciesRef{c.FounderSpeciesRef, c.BuiltSpeciesRef}, []SpeciesRefList{c.OwnedSpeciesRefs, c.EnslavedSpeciesRefs}, &countrySpeciesRefs, &countrySpeciesMatches)
		}
		if gs.Colonies != nil {
			colonyTables++
		}
		for id, c := range gs.Colonies {
			countRefs([]SpeciesRef{c.ColonizingSpecies}, []SpeciesRefList{c.SpeciesRefs, c.EnslavedSpeciesRefs}, &colonySpeciesRefs, &colonySpeciesMatches)
			colonyInfo += len(c.SpeciesInformation.Items)
			unsupported += len(c.SpeciesInformation.DecodeErrors)
			if c.SpeciesInformation.TableIssue {
				unsupported++
			}
			for _, gid := range c.PopGroups {
				groupRefs++
				if g, ok := gs.PopGroups[gid]; ok && g.PlanetRef.Value != nil && *g.PlanetRef.Value == id {
					groupMatches++
				}
			}
			for _, jid := range c.PopJobs {
				jobRefs++
				if j, ok := gs.PopJobs[jid]; ok && j.PlanetRef.Value != nil && *j.PlanetRef.Value == id {
					jobMatches++
				}
			}
		}
	}
	t.Logf("local population coverage: saves=%d species=%d base_refs=%d base_matches=%d scalar_home_refs=%d scalar_home_candidates=%d typed_home_refs=%d typed_home_matches=%d country_species_refs=%d country_species_matches=%d colony_tables=%d colony_species_refs=%d colony_species_matches=%d colony_species_info=%d pop_group_refs=%d pop_group_colony_matches=%d job_refs=%d job_colony_matches=%d unsupported=%d", len(files), species, baseLinks, baseMatches, scalarHome, scalarMatches, typedHome, typedMatches, countrySpeciesRefs, countrySpeciesMatches, colonyTables, colonySpeciesRefs, colonySpeciesMatches, colonyInfo, groupRefs, groupMatches, jobRefs, jobMatches, unsupported)
	if unsupported != 0 {
		t.Fatal("local population shapes need additional support")
	}
}
