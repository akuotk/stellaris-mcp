package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

// Construction preserves queue order and item back-references without interpreting progress units.
type Construction struct {
	DecodeError  string `clausewitz:"-"`
	QueueManager struct {
		Queues map[int]ConstructionQueue `clausewitz:"queues"`
	} `clausewitz:"queue_mgr"`
	ItemManager struct {
		Items map[int]ConstructionItem `clausewitz:"items"`
	} `clausewitz:"item_mgr"`
}

type ConstructionQueue struct {
	Owner    *int `clausewitz:"owner"`
	Location struct {
		Type *int `clausewitz:"type"`
		ID   *int `clausewitz:"id"`
	} `clausewitz:"location"`
	Simultaneous *float64 `clausewitz:"simultaneous"`
	Type         string   `clausewitz:"type"`
	Items        []int    `clausewitz:"items"`
	Disabled     *bool    `clausewitz:"disabled"`
}

type ConstructionItem struct {
	Queue            *int                   `clausewitz:"queue"`
	PayingCountry    *int                   `clausewitz:"paying_country"`
	Progress         *float64               `clausewitz:"progress"`
	ProgressNeeded   *float64               `clausewitz:"progress_needed"`
	Resources        map[string]float64     `clausewitz:"resources"`
	District         *ConstructionBuildable `clausewitz:"buildable_district"`
	Building         *ConstructionBuildable `clausewitz:"buildable_planet_building"`
	UpgradeBuilding  *ConstructionBuildable `clausewitz:"buildable_planet_upgrade_building"`
	ReplaceBuilding  *ConstructionBuildable `clausewitz:"buildable_planet_replace_building"`
	ClearBlocker     *ConstructionBuildable `clausewitz:"buildable_clear_deposit_blocker"`
	Army             *ConstructionBuildable `clausewitz:"buildable_army"`
	Ship             *ConstructionBuildable `clausewitz:"buildable_ship"`
	ColonyShip       *ConstructionBuildable `clausewitz:"buildable_colony_ship"`
	StarbaseModule   *ConstructionBuildable `clausewitz:"buildable_starbase_module"`
	StarbaseBuilding *ConstructionBuildable `clausewitz:"buildable_starbase_building"`
	StarbaseUpgrade  *ConstructionBuildable `clausewitz:"buildable_starbase_upgrade"`
}

type ConstructionBuildable struct {
	District         string `clausewitz:"district"`
	Building         string `clausewitz:"building"`
	UpgradeBuilding  string `clausewitz:"upgrade_building"`
	ReplaceBuilding  string `clausewitz:"replace_building"`
	Planet           *int   `clausewitz:"planet"`
	Zone             *int   `clausewitz:"zone"`
	Deposit          *int   `clausewitz:"deposit"`
	ArmyType         string `clausewitz:"army_type"`
	Species          *int   `clausewitz:"species"`
	Starbase         *int   `clausewitz:"starbase"`
	Slot             *int   `clausewitz:"slot"`
	StarbaseModule   string `clausewitz:"starbase_module"`
	StarbaseBuilding string `clausewitz:"starbase_building"`
	StarbaseUpgrade  string `clausewitz:"starbase_upgrade"`
	ShipDesign       struct {
		Design      *int `clausewitz:"design"`
		Upgrade     *int `clausewitz:"upgrade"`
		GrowthStage *int `clausewitz:"growth_stage"`
	} `clausewitz:"ship_design_implementation"`
	Orbitable struct {
		Starbase *int `clausewitz:"starbase"`
	} `clausewitz:"orbitable"`
	ColonizationData struct {
		Species *int `clausewitz:"species"`
	} `clausewitz:"colonization_data"`
}

// An unsupported optional construction shape must not block unrelated save tools.
func (c *Construction) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields Construction
	var decoded fields
	err := clausewitz.DecodeValue(value, &decoded)
	*c = Construction(decoded)
	if err != nil {
		c.DecodeError = err.Error()
	}
	return nil
}
