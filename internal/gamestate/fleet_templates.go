package gamestate

import (
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type FleetTemplateManagerRefs struct {
	Recorded    bool
	Templates   OptionalIntList `clausewitz:"fleet_template"`
	Unsupported bool
}

func (v *FleetTemplateManagerRefs) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = FleetTemplateManagerRefs{Recorded: true}
	if uniqueSavedFields(value, "fleet_template") != nil {
		v.Unsupported = true
		return nil
	}
	type fields FleetTemplateManagerRefs
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		v.Unsupported = true
		return nil
	}
	*v = FleetTemplateManagerRefs(decoded)
	v.Recorded = true
	return nil
}

type FleetTemplates struct {
	Recorded     bool
	Items        map[int]FleetTemplate
	DecodeErrors map[int]string
	TableIssue   bool
}

type FleetTemplate struct {
	Fleet         *int                   `clausewitz:"fleet"`
	FleetSize     *string                `clausewitz:"fleet_size"`
	Reinforcement *string                `clausewitz:"reinforcement"`
	EditedByHuman *string                `clausewitz:"is_edited_by_human"`
	Designs       []FleetTemplateDesign  `clausewitz:"fleet_template_design"`
	HomeBase      *FleetTemplateHomeBase `clausewitz:"home_base"`
	AllQueued     *FleetTemplateQueued   `clausewitz:"all_queued"`
}

func (v *FleetTemplate) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "fleet", "fleet_size", "reinforcement", "is_edited_by_human", "fleet_template_design", "home_base", "all_queued"); err != nil {
		return err
	}
	if value.IsObject() {
		for _, pair := range value.Object.Pairs {
			if pair.Key == "fleet_template_design" && pair.Value.IsObject() && len(pair.Value.Object.Pairs) > 0 {
				return fmt.Errorf("unsupported fleet template design list shape")
			}
		}
	}
	type fields FleetTemplate
	return clausewitz.DecodeValue(value, (*fields)(v))
}

type FleetTemplateDesign struct {
	Design     *int                     `clausewitz:"design"`
	ShipDesign *FleetTemplateShipDesign `clausewitz:"ship_design_implementation"`
	Count      *string                  `clausewitz:"count"`
	Items      OptionalIntList          `clausewitz:"items"`
}

func (v *FleetTemplateDesign) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "design", "ship_design_implementation", "count", "items"); err != nil {
		return err
	}
	type fields FleetTemplateDesign
	return clausewitz.DecodeValue(value, (*fields)(v))
}

type FleetTemplateShipDesign ShipDesignImpl

func (v *FleetTemplateShipDesign) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "design", "upgrade", "growth_stage"); err != nil {
		return err
	}
	return clausewitz.DecodeValue(value, (*ShipDesignImpl)(v))
}

type FleetTemplateHomeBase struct {
	Orbitable *FleetTemplateOrbitable `clausewitz:"orbitable"`
}

func (v *FleetTemplateHomeBase) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "orbitable"); err != nil {
		return err
	}
	type fields FleetTemplateHomeBase
	return clausewitz.DecodeValue(value, (*fields)(v))
}

type FleetTemplateOrbitable struct {
	Starbase *int `clausewitz:"starbase"`
}

func (v *FleetTemplateOrbitable) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "starbase"); err != nil {
		return err
	}
	type fields FleetTemplateOrbitable
	return clausewitz.DecodeValue(value, (*fields)(v))
}

type FleetTemplateQueued struct {
	ConstructionItems OptionalIntList           `clausewitz:"construction_queue_items"`
	UpgradeDesigns    OptionalIntList           `clausewitz:"upgrade_designs"`
	Implementations   []FleetTemplateShipDesign `clausewitz:"upgrade_design_implementations"`
}

func (v *FleetTemplateQueued) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "construction_queue_items", "upgrade_designs", "upgrade_design_implementations"); err != nil {
		return err
	}
	if value.IsObject() {
		for _, pair := range value.Object.Pairs {
			if pair.Key == "upgrade_design_implementations" && pair.Value.IsObject() && len(pair.Value.Object.Pairs) > 0 {
				return fmt.Errorf("unsupported template upgrade implementation list shape")
			}
		}
	}
	type fields FleetTemplateQueued
	return clausewitz.DecodeValue(value, (*fields)(v))
}

func (v *FleetTemplates) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = FleetTemplates{Recorded: true, Items: map[int]FleetTemplate{}, DecodeErrors: map[int]string{}}
	if !value.IsObject() {
		v.TableIssue = true
		return nil
	}
	seen := map[int]bool{}
	for _, pair := range value.Object.Pairs {
		id, err := strconv.Atoi(pair.Key)
		if err != nil {
			v.TableIssue = true
			continue
		}
		if seen[id] {
			delete(v.Items, id)
			v.DecodeErrors[id] = "duplicate fleet template ID"
			continue
		}
		seen[id] = true
		if pair.Value.IsScalar() && pair.Value.Scalar == "none" {
			continue
		}
		var template FleetTemplate
		if err := clausewitz.DecodeValue(pair.Value, &template); err != nil {
			v.DecodeErrors[id] = err.Error()
			continue
		}
		v.Items[id] = template
	}
	return nil
}
