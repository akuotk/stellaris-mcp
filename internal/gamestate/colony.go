package gamestate

import (
	"math"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

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
	Districts           []int                                           `clausewitz:"-"`
	DistrictRefs        SurfaceRefList                                  `clausewitz:"districts"`
	BuildingsCache      []int                                           `clausewitz:"-"`
	BuildingCacheRefs   SurfaceRefList                                  `clausewitz:"buildings_cache"`
	ExternalBuildings   ExternalSurfaceBuildings                        `clausewitz:"externally_owned_buildings"`
	Produces            map[string]float64                              `clausewitz:"produces"`
	Upkeep              map[string]float64                              `clausewitz:"upkeep"`
	Profits             map[string]float64                              `clausewitz:"profits"`
	PopGroups           []int                                           `clausewitz:"pop_groups"`
	SpeciesRefs         SpeciesRefList                                  `clausewitz:"species_refs"`
	EnslavedSpeciesRefs SpeciesRefList                                  `clausewitz:"enslaved_species_refs"`
	ColonizingSpecies   SpeciesRef                                      `clausewitz:"colonizing_species"`
	SpeciesInformation  ConnectionRecordTable[ColonySpeciesInformation] `clausewitz:"species_information"`
	Carrier             ColonyCarrier                                   `clausewitz:"carrier"`
}

type ColonyCarrier struct {
	Type        string `clausewitz:"type"`
	Reference   int    `clausewitz:"reference"`
	ReferenceID *int   `clausewitz:"reference"`
	Unsupported bool   `clausewitz:"-"`
}

func (c *ColonyCarrier) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields ColonyCarrier
	var decoded fields
	if uniqueSavedFields(value, "type", "reference") != nil || clausewitz.DecodeValue(value, &decoded) != nil {
		*c = ColonyCarrier{Unsupported: true}
		return nil
	}
	*c = ColonyCarrier(decoded)
	return nil
}

func (c *Colony) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields Colony
	if err := clausewitz.DecodeValue(value, (*fields)(c)); err != nil {
		return err
	}
	if uniqueSavedFields(value, "carrier") != nil {
		c.Carrier = ColonyCarrier{Unsupported: true}
	}
	c.DistrictRefs.rejectDuplicate(value, "districts")
	c.BuildingCacheRefs.rejectDuplicate(value, "buildings_cache")
	c.ExternalBuildings.rejectDuplicate(value)
	c.Districts, c.BuildingsCache = c.DistrictRefs.Values, c.BuildingCacheRefs.Values
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
	Planet                   int                 `clausewitz:"-"`
	PlanetRef                OptionalIntRef      `clausewitz:"planet"`
	PlanetRefDuplicate       bool                `clausewitz:"-"`
	Size                     int                 `clausewitz:"-"`
	SizeRaw                  *int                `clausewitz:"-"`
	LastMonthGrowth          int                 `clausewitz:"-"`
	LastMonthGrowthRaw       *string             `clausewitz:"-"`
	LastMonthGrowthDuplicate bool                `clausewitz:"-"`
	MonthStartSize           OptionalString      `clausewitz:"month_start_size"`
	Habitability             float64             `clausewitz:"-"`
	Happiness                float64             `clausewitz:"-"`
	SavedSize                SavedPopGroupNumber `clausewitz:"-"`
	SavedGrowth              SavedPopGroupNumber `clausewitz:"-"`
	SavedHabitability        SavedPopGroupNumber `clausewitz:"-"`
	SavedHappiness           SavedPopGroupNumber `clausewitz:"-"`
	RecordDuplicate          bool                `clausewitz:"-"`
}

type SavedPopGroupNumber struct {
	Values  []clausewitz.Value
	Valid   bool
	Integer int64
}

func (n *SavedPopGroupNumber) parseInteger() {
	if len(n.Values) != 1 || !n.Values[0].IsScalar() {
		return
	}
	value, err := strconv.ParseInt(n.Values[0].Scalar, 10, 64)
	if err == nil {
		n.Integer, n.Valid = value, true
	}
}

func (n *SavedPopGroupNumber) parseDecimal() float64 {
	if len(n.Values) == 1 && n.Values[0].IsScalar() {
		value, err := strconv.ParseFloat(n.Values[0].Scalar, 64)
		if err == nil && !math.IsInf(value, 0) && !math.IsNaN(value) {
			n.Valid = true
			return value
		}
	}
	return 0
}

func (g *PopGroup) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields PopGroup
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		return err
	}
	*g = PopGroup(decoded)
	for _, field := range value.Object.Pairs {
		switch field.Key {
		case "size":
			g.SavedSize.Values = append(g.SavedSize.Values, field.Value)
		case "last_month_growth":
			g.SavedGrowth.Values = append(g.SavedGrowth.Values, field.Value)
		case "habitability":
			g.SavedHabitability.Values = append(g.SavedHabitability.Values, field.Value)
		case "happiness":
			g.SavedHappiness.Values = append(g.SavedHappiness.Values, field.Value)
		}
	}
	g.SavedSize.parseInteger()
	g.SavedGrowth.parseInteger()
	g.Habitability = g.SavedHabitability.parseDecimal()
	g.Happiness = g.SavedHappiness.parseDecimal()
	if g.SavedSize.Valid && int64(int(g.SavedSize.Integer)) == g.SavedSize.Integer {
		g.Size = int(g.SavedSize.Integer)
		size := g.Size
		g.SizeRaw = &size
	}
	if g.SavedGrowth.Valid {
		g.LastMonthGrowthRaw = &g.SavedGrowth.Values[0].Scalar
		if int64(int(g.SavedGrowth.Integer)) == g.SavedGrowth.Integer {
			g.LastMonthGrowth = int(g.SavedGrowth.Integer)
		}
	}
	g.LastMonthGrowthDuplicate = len(g.SavedGrowth.Values) > 1
	g.PlanetRefDuplicate = uniqueSavedFields(value, "planet") != nil
	if g.PlanetRefDuplicate {
		g.PlanetRef = OptionalIntRef{Recorded: true}
	} else if g.PlanetRef.Value != nil {
		g.Planet = *g.PlanetRef.Value
	}
	if uniqueSavedFields(value, "month_start_size") != nil {
		g.MonthStartSize = OptionalString{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "factions") != nil {
		g.Factions = PopFactionDistribution{Duplicate: true}
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
		if c.DistrictRefs.Recorded {
			p.Districts = c.Districts
			p.DistrictRefs = c.DistrictRefs
		}
		if c.BuildingCacheRefs.Recorded {
			p.BuildingsCache = c.BuildingsCache
			p.BuildingCacheRefs = c.BuildingCacheRefs
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
