package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type Planet struct {
	FinalDesignationRaw *string                  `clausewitz:"final_designation"`
	AscensionTierRaw    *int                     `clausewitz:"ascension_tier"`
	StabilityRaw        *float64                 `clausewitz:"stability"`
	CrimeRaw            *float64                 `clausewitz:"crime"`
	AmenitiesRaw        *float64                 `clausewitz:"amenities"`
	AmenitiesUsageRaw   *float64                 `clausewitz:"amenities_usage"`
	FreeAmenitiesRaw    *float64                 `clausewitz:"free_amenities"`
	TotalHousingRaw     *float64                 `clausewitz:"total_housing"`
	HousingUsageRaw     *float64                 `clausewitz:"housing_usage"`
	FreeHousingRaw      *float64                 `clausewitz:"free_housing"`
	NumPopsRaw          *int                     `clausewitz:"num_sapient_pops"`
	ColonyID            *int                     `clausewitz:"colony"`
	OwnerID             *int                     `clausewitz:"owner"`
	SurveyedByID        *int                     `clausewitz:"surveyed_by"`
	PopGroups           []int                    `clausewitz:"pop_groups"`
	Name                LocalizedName            `clausewitz:"name"`
	PlanetClass         string                   `clausewitz:"planet_class"`
	PlanetSize          int                      `clausewitz:"planet_size"`
	Owner               int                      `clausewitz:"owner"`
	Controller          int                      `clausewitz:"controller"`
	ControllerID        *int                     `clausewitz:"controller"`
	Stability           float64                  `clausewitz:"stability"`
	Crime               float64                  `clausewitz:"crime"`
	Amenities           float64                  `clausewitz:"amenities"`
	AmenitiesUsage      float64                  `clausewitz:"amenities_usage"`
	FreeAmenities       float64                  `clausewitz:"free_amenities"`
	TotalHousing        float64                  `clausewitz:"total_housing"`
	HousingUsage        float64                  `clausewitz:"housing_usage"`
	FreeHousing         float64                  `clausewitz:"free_housing"`
	NumPops             int                      `clausewitz:"num_sapient_pops"`
	FinalDesignation    string                   `clausewitz:"final_designation"`
	AscensionTier       int                      `clausewitz:"ascension_tier"`
	Districts           []int                    `clausewitz:"-"`
	DistrictRefs        SurfaceRefList           `clausewitz:"districts"`
	LegacyDistricts     []clausewitz.Value       `clausewitz:"-"`
	BuildingsCache      []int                    `clausewitz:"-"`
	BuildingCacheRefs   SurfaceRefList           `clausewitz:"buildings_cache"`
	LegacyBuildingRefs  SurfaceRefList           `clausewitz:"buildings"`
	ExternalBuildings   ExternalSurfaceBuildings `clausewitz:"externally_owned_buildings"`
	SurveyedBy          int                      `clausewitz:"surveyed_by"`
	Deposits            []int                    `clausewitz:"-"`
	DepositRefs         SurfaceRefList           `clausewitz:"deposits"`
	TerraformProcess    *clausewitz.Value        `clausewitz:"-"`
	TerraformDuplicate  bool                     `clausewitz:"-"`
	TimedModifier       TimedModifierList        `clausewitz:"timed_modifier"`
	Produces            map[string]float64       `clausewitz:"produces"`
	Upkeep              map[string]float64       `clausewitz:"upkeep"`
	Profits             map[string]float64       `clausewitz:"profits"`
}

func (p *Planet) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields Planet
	if err := clausewitz.DecodeValue(value, (*fields)(p)); err != nil {
		return err
	}
	p.DistrictRefs.rejectDuplicate(value, "districts")
	p.BuildingCacheRefs.rejectDuplicate(value, "buildings_cache")
	p.LegacyBuildingRefs.rejectDuplicate(value, "buildings")
	p.DepositRefs.rejectDuplicate(value, "deposits")
	p.ExternalBuildings.rejectDuplicate(value)
	p.Districts, p.BuildingsCache, p.Deposits = p.DistrictRefs.Values, p.BuildingCacheRefs.Values, p.DepositRefs.Values
	for _, pair := range value.Object.Pairs {
		switch pair.Key {
		case "district":
			p.LegacyDistricts = append(p.LegacyDistricts, pair.Value)
		case "terraform_process":
			p.TerraformDuplicate = p.TerraformProcess != nil
			p.TerraformProcess = &pair.Value
		}
	}
	return nil
}

type TimedModifierList struct {
	Items []TimedModifier `clausewitz:"items"`
}

type TimedModifier struct {
	Modifier string `clausewitz:"modifier"`
	Days     int    `clausewitz:"days"`
}
