package gamestate

// Colony stores the population and economy records separated from planets in 4.5 saves.
type Colony struct {
	Stability        float64            `clausewitz:"stability"`
	Crime            float64            `clausewitz:"crime"`
	Amenities        float64            `clausewitz:"amenities"`
	AmenitiesUsage   float64            `clausewitz:"amenities_usage"`
	FreeAmenities    float64            `clausewitz:"free_amenities"`
	TotalHousing     float64            `clausewitz:"total_housing"`
	HousingUsage     float64            `clausewitz:"housing_usage"`
	FreeHousing      float64            `clausewitz:"free_housing"`
	NumPops          int                `clausewitz:"num_sapient_pops"`
	FinalDesignation string             `clausewitz:"final_designation"`
	AscensionTier    int                `clausewitz:"ascension_tier"`
	Districts        []int              `clausewitz:"districts"`
	BuildingsCache   []int              `clausewitz:"buildings_cache"`
	Produces         map[string]float64 `clausewitz:"produces"`
	Upkeep           map[string]float64 `clausewitz:"upkeep"`
	Profits          map[string]float64 `clausewitz:"profits"`
	PopGroups        []int              `clausewitz:"pop_groups"`
	Carrier          struct {
		Type      string `clausewitz:"type"`
		Reference int    `clausewitz:"reference"`
	} `clausewitz:"carrier"`
}

type PopGroup struct {
	Ethos    PopEthos               `clausewitz:"ethos"`
	Factions PopFactionDistribution `clausewitz:"factions"`
	Key      struct {
		Species   int    `clausewitz:"species"`
		SpeciesID *int   `clausewitz:"species"`
		Category  string `clausewitz:"category"`
	} `clausewitz:"key"`
	Planet          int     `clausewitz:"planet"`
	Size            int     `clausewitz:"size"`
	SizeRaw         *int    `clausewitz:"size"`
	LastMonthGrowth int     `clausewitz:"last_month_growth"`
	Habitability    float64 `clausewitz:"habitability"`
	Happiness       float64 `clausewitz:"happiness"`
}

func (gs *GameState) resolveColonies() {
	for id, p := range gs.Planets.Planet {
		if p.ColonyID == nil {
			continue
		}
		c, ok := gs.Colonies[*p.ColonyID]
		if !ok || c.Carrier.Type != "planet" || c.Carrier.Reference != id {
			continue
		}
		p.Stability = c.Stability
		p.Crime = c.Crime
		p.Amenities = c.Amenities
		p.AmenitiesUsage = c.AmenitiesUsage
		p.FreeAmenities = c.FreeAmenities
		p.TotalHousing = c.TotalHousing
		p.HousingUsage = c.HousingUsage
		p.FreeHousing = c.FreeHousing
		p.NumPops = c.NumPops
		p.FinalDesignation = c.FinalDesignation
		p.AscensionTier = c.AscensionTier
		p.Districts = c.Districts
		p.BuildingsCache = c.BuildingsCache
		p.Produces = c.Produces
		p.Upkeep = c.Upkeep
		p.Profits = c.Profits
		p.PopGroups = c.PopGroups
		gs.Planets.Planet[id] = p
	}
}
