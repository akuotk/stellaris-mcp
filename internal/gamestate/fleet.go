package gamestate

import (
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type Fleet struct {
	CurrentOrderRefs FleetCurrentOrderRefs `clausewitz:"current_order"`
	TemplateRef      OptionalIntRef        `clausewitz:"fleet_template"`
	MilitaryPowerRaw *float64              `clausewitz:"military_power"`
	Name             LocalizedName         `clausewitz:"name"`
	Ships            []int                 `clausewitz:"ships"`
	MilitaryPower    float64               `clausewitz:"military_power"`
	ShipClass        string                `clausewitz:"ship_class"`
	Station          bool                  `clausewitz:"station"`
	Civilian         bool                  `clausewitz:"civilian"`
}

type Ship struct {
	Coordinate               OptionalCoordinate `clausewitz:"coordinate"`
	ArmyID                   *int               `clausewitz:"army"`
	Fleet                    int                `clausewitz:"fleet"`
	FleetID                  *int               `clausewitz:"fleet"`
	Name                     LocalizedName      `clausewitz:"name"`
	ShipDesignImplementation ShipDesignImpl     `clausewitz:"ship_design_implementation"`
	Section                  ShipSection        `clausewitz:"section"`
	Hitpoints                *float64           `clausewitz:"hitpoints"`
	ShieldHitpoints          *float64           `clausewitz:"shield_hitpoints"`
	ArmorHitpoints           *float64           `clausewitz:"armor_hitpoints"`
	MaxHitpoints             *float64           `clausewitz:"max_hitpoints"`
	MaxShieldHitpoints       *float64           `clausewitz:"max_shield_hitpoints"`
	MaxArmorHitpoints        *float64           `clausewitz:"max_armor_hitpoints"`
}

type ShipDesignImpl struct {
	Design      *int `clausewitz:"design"`
	Upgrade     *int `clausewitz:"upgrade"`
	GrowthStage *int `clausewitz:"growth_stage"`
}

type ShipSection struct {
	Design string       `clausewitz:"design"`
	Slot   string       `clausewitz:"slot"`
	Weapon []ShipWeapon `clausewitz:"weapon"`
}

type ShipWeapon struct {
	Template      string `clausewitz:"template"`
	ComponentSlot string `clausewitz:"component_slot"`
}

type ShipDesign struct {
	Name         LocalizedName     `clausewitz:"name"`
	GrowthStages []ShipGrowthStage `clausewitz:"growth_stages"`
}

type ShipGrowthStage struct {
	ShipSize          string          `clausewitz:"ship_size"`
	Sections          []DesignSection `clausewitz:"section"`
	RequiredComponent []string        `clausewitz:"required_component"`
}

type DesignSection struct {
	Template  string            `clausewitz:"template"`
	Slot      string            `clausewitz:"slot"`
	Component []DesignComponent `clausewitz:"component"`
}

type DesignComponent struct {
	Slot     string `clausewitz:"slot"`
	Template string `clausewitz:"template"`
}

func (f *Fleet) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields Fleet
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		return err
	}
	*f = Fleet(decoded)
	if uniqueSavedFields(value, "fleet_template") != nil {
		f.TemplateRef = OptionalIntRef{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "current_order") != nil {
		f.CurrentOrderRefs = FleetCurrentOrderRefs{Unsupported: true}
	}
	return nil
}

type FleetCurrentOrderRefs struct {
	SpecialProject          OptionalIntRef
	AutomatedSpecialProject OptionalIntRef
	ArchaeologicalSite      OptionalIntRef
	Unsupported             bool
}

func (v *FleetCurrentOrderRefs) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = FleetCurrentOrderRefs{}
	if !value.IsObject() || len(value.Object.Pairs) > 1 {
		v.Unsupported = true
		return nil
	}
	v.SpecialProject = fleetOrderRef(value, "collect_data_fleet_order", "special_project")
	v.AutomatedSpecialProject = fleetOrderRef(value, "automate_fleet_order", "sub_order", "collect_data_fleet_order", "special_project")
	v.ArchaeologicalSite = fleetOrderRef(value, "excavate_archaeological_site_fleet_order", "archaeological_site")
	return nil
}

func fleetOrderRef(value clausewitz.Value, path ...string) OptionalIntRef {
	for _, key := range path {
		if !value.IsObject() || uniqueSavedFields(value, key) != nil {
			return OptionalIntRef{Recorded: true, Unsupported: true}
		}
		found := false
		for _, pair := range value.Object.Pairs {
			if pair.Key == key {
				value, found = pair.Value, true
				break
			}
		}
		if !found {
			return OptionalIntRef{}
		}
	}
	var ref OptionalIntRef
	ref.UnmarshalClausewitz(value)
	return ref
}

func (gs *GameState) markAmbiguousFleetOrders(root clausewitz.Value) {
	if uniqueSavedFields(root, "fleet") != nil {
		for id, fleet := range gs.Fleet {
			fleet.CurrentOrderRefs = FleetCurrentOrderRefs{Unsupported: true}
			gs.Fleet[id] = fleet
		}
		return
	}
	for _, table := range root.Object.Pairs {
		if table.Key != "fleet" || !table.Value.IsObject() {
			continue
		}
		seen := make(map[int]bool, len(gs.Fleet))
		for _, pair := range table.Value.Object.Pairs {
			id, err := strconv.Atoi(pair.Key)
			if err != nil {
				continue
			}
			if fleet, ok := gs.Fleet[id]; ok && seen[id] {
				fleet.CurrentOrderRefs = FleetCurrentOrderRefs{Unsupported: true}
				gs.Fleet[id] = fleet
			}
			seen[id] = true
		}
		return
	}
}
