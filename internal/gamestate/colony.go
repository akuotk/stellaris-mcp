package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

// Colony stores the population and economy records separated from planets in 4.5 saves.
type Colony struct {
	FinalDesignationRaw *string                                         `clausewitz:"final_designation"`
	Armies              []int                                           `clausewitz:"army"`
	SpawnedArmies       []int                                           `clausewitz:"spawned_armies"`
	StabilityRaw        *float64                                        `clausewitz:"stability"`
	CrimeRaw            *float64                                        `clausewitz:"crime"`
	AmenitiesRaw        *float64                                        `clausewitz:"amenities"`
	AmenitiesUsageRaw   *float64                                        `clausewitz:"amenities_usage"`
	FreeAmenitiesRaw    *float64                                        `clausewitz:"free_amenities"`
	FreeHousingRaw      *float64                                        `clausewitz:"free_housing"`
	TotalHousingRaw     *float64                                        `clausewitz:"total_housing"`
	HousingUsageRaw     *float64                                        `clausewitz:"housing_usage"`
	NumPopsRaw          *int                                            `clausewitz:"num_sapient_pops"`
	AscensionTierRaw    *int                                            `clausewitz:"ascension_tier"`
	PopJobs             []int                                           `clausewitz:"pop_jobs"`
	Stability           float64                                         `clausewitz:"stability"`
	Crime               float64                                         `clausewitz:"crime"`
	Amenities           float64                                         `clausewitz:"amenities"`
	AmenitiesUsage      float64                                         `clausewitz:"amenities_usage"`
	FreeAmenities       float64                                         `clausewitz:"free_amenities"`
	TotalHousing        float64                                         `clausewitz:"total_housing"`
	HousingUsage        float64                                         `clausewitz:"housing_usage"`
	FreeHousing         float64                                         `clausewitz:"free_housing"`
	NumPops             int                                             `clausewitz:"num_sapient_pops"`
	FinalDesignation    string                                          `clausewitz:"final_designation"`
	AscensionTier       int                                             `clausewitz:"ascension_tier"`
	Districts           []int                                           `clausewitz:"districts"`
	BuildingsCache      []int                                           `clausewitz:"buildings_cache"`
	Produces            map[string]float64                              `clausewitz:"produces"`
	Upkeep              map[string]float64                              `clausewitz:"upkeep"`
	Profits             map[string]float64                              `clausewitz:"profits"`
	PopGroups           []int                                           `clausewitz:"pop_groups"`
	SpeciesRefs         SpeciesRefList                                  `clausewitz:"species_refs"`
	EnslavedSpeciesRefs SpeciesRefList                                  `clausewitz:"enslaved_species_refs"`
	ColonizingSpecies   SpeciesRef                                      `clausewitz:"colonizing_species"`
	SpeciesInformation  ConnectionRecordTable[ColonySpeciesInformation] `clausewitz:"species_information"`
	Carrier             struct {
		Type        string `clausewitz:"type"`
		Reference   int    `clausewitz:"reference"`
		ReferenceID *int   `clausewitz:"reference"`
	} `clausewitz:"carrier"`
}

func (c *Colony) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields Colony
	if err := clausewitz.DecodeValue(value, (*fields)(c)); err != nil {
		return err
	}
	if uniqueSavedFields(value, "species_refs") != nil {
		c.SpeciesRefs = SpeciesRefList{Duplicate: true}
	}
	if uniqueSavedFields(value, "enslaved_species_refs") != nil {
		c.EnslavedSpeciesRefs = SpeciesRefList{Duplicate: true}
	}
	if uniqueSavedFields(value, "colonizing_species") != nil {
		c.ColonizingSpecies = SpeciesRef{Duplicate: true}
	}
	if uniqueSavedFields(value, "species_information") != nil {
		c.SpeciesInformation = ConnectionRecordTable[ColonySpeciesInformation]{Recorded: true, TableIssue: true}
	}
	return nil
}

type ColonySpeciesInformation struct {
	NumPops     *string `clausewitz:"num_pops"`
	NumEnslaved *string `clausewitz:"num_enslaved"`
}

func (s *ColonySpeciesInformation) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "num_pops", "num_enslaved"); err != nil {
		return err
	}
	type fields ColonySpeciesInformation
	return clausewitz.DecodeValue(value, (*fields)(s))
}

type PopGroup struct {
	Ethos    PopEthos               `clausewitz:"ethos"`
	Factions PopFactionDistribution `clausewitz:"factions"`
	Key      struct {
		Species   int    `clausewitz:"species"`
		SpeciesID *int   `clausewitz:"species"`
		Category  string `clausewitz:"category"`
	} `clausewitz:"key"`
	Planet                   int            `clausewitz:"-"`
	PlanetRef                OptionalIntRef `clausewitz:"planet"`
	PlanetRefDuplicate       bool           `clausewitz:"-"`
	Size                     int            `clausewitz:"size"`
	SizeRaw                  *int           `clausewitz:"size"`
	LastMonthGrowth          int            `clausewitz:"last_month_growth"`
	LastMonthGrowthRaw       *string        `clausewitz:"last_month_growth"`
	LastMonthGrowthDuplicate bool           `clausewitz:"-"`
	MonthStartSize           OptionalString `clausewitz:"month_start_size"`
	Habitability             float64        `clausewitz:"habitability"`
	Happiness                float64        `clausewitz:"happiness"`
}

func (g *PopGroup) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields PopGroup
	if err := clausewitz.DecodeValue(value, (*fields)(g)); err != nil {
		return err
	}
	g.PlanetRefDuplicate = uniqueSavedFields(value, "planet") != nil
	if g.PlanetRefDuplicate {
		g.PlanetRef = OptionalIntRef{Recorded: true}
	} else if g.PlanetRef.Value != nil {
		g.Planet = *g.PlanetRef.Value
	}
	if uniqueSavedFields(value, "month_start_size") != nil {
		g.MonthStartSize = OptionalString{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "last_month_growth") != nil {
		g.LastMonthGrowthRaw = nil
		g.LastMonthGrowthDuplicate = true
	}
	return nil
}

func (gs *GameState) resolveColonies() {
	for id, p := range gs.Planets.Planet {
		if p.ColonyID == nil {
			continue
		}
		c, ok := gs.Colonies[*p.ColonyID]
		if !ok || c.Carrier.Type != "planet" || c.Carrier.ReferenceID == nil || *c.Carrier.ReferenceID != id {
			continue
		}
		if c.StabilityRaw != nil {
			p.StabilityRaw = c.StabilityRaw
			p.Stability = c.Stability
		}
		if c.CrimeRaw != nil {
			p.CrimeRaw = c.CrimeRaw
			p.Crime = c.Crime
		}
		if c.AmenitiesRaw != nil {
			p.AmenitiesRaw = c.AmenitiesRaw
			p.Amenities = c.Amenities
		}
		if c.AmenitiesUsageRaw != nil {
			p.AmenitiesUsageRaw = c.AmenitiesUsageRaw
			p.AmenitiesUsage = c.AmenitiesUsage
		}
		if c.FreeAmenitiesRaw != nil {
			p.FreeAmenitiesRaw = c.FreeAmenitiesRaw
			p.FreeAmenities = c.FreeAmenities
		}
		if c.TotalHousingRaw != nil {
			p.TotalHousingRaw = c.TotalHousingRaw
			p.TotalHousing = c.TotalHousing
		}
		if c.HousingUsageRaw != nil {
			p.HousingUsageRaw = c.HousingUsageRaw
			p.HousingUsage = c.HousingUsage
		}
		if c.FreeHousingRaw != nil {
			p.FreeHousingRaw = c.FreeHousingRaw
			p.FreeHousing = c.FreeHousing
		}
		if c.NumPopsRaw != nil {
			p.NumPopsRaw = c.NumPopsRaw
			p.NumPops = c.NumPops
		}
		if c.FinalDesignationRaw != nil {
			p.FinalDesignationRaw = c.FinalDesignationRaw
			p.FinalDesignation = c.FinalDesignation
		}
		if c.AscensionTierRaw != nil {
			p.AscensionTierRaw = c.AscensionTierRaw
			p.AscensionTier = c.AscensionTier
		}
		if c.Districts != nil {
			p.Districts = c.Districts
		}
		if c.BuildingsCache != nil {
			p.BuildingsCache = c.BuildingsCache
		}
		if c.Produces != nil {
			p.Produces = c.Produces
		}
		if c.Upkeep != nil {
			p.Upkeep = c.Upkeep
		}
		if c.Profits != nil {
			p.Profits = c.Profits
		}
		if c.PopGroups != nil {
			p.PopGroups = c.PopGroups
		}
		gs.Planets.Planet[id] = p
	}
}
