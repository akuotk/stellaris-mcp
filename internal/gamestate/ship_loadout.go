package gamestate

import (
	"archive/zip"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type ShipLoadout struct {
	Ship, Fleet, Design      LoadoutRecord
	ShipID, DesignID         *int
	FleetRef                 OptionalIntRef
	SameNumericFleetID       bool
	FleetShips               OptionalIntList
	LegacyDesign, ImplDesign OptionalIntRef
	GrowthStage, Upgrade     OptionalIntRef
	ShipLayout               LoadoutLayout
	DesignLayouts            []LoadoutLayout
	GrowthStagesUnsupported  bool
}

type LoadoutRecord struct {
	Value clausewitz.Value
	Issue string
}

type LoadoutLayout struct {
	Value    clausewitz.Value
	Stage    *int
	Sections []LoadoutSection
}

type LoadoutSection struct {
	Value          clausewitz.Value
	Slot, Template OptionalString
	Components     []LoadoutComponent
}

type LoadoutComponent struct {
	Value          clausewitz.Value
	Slot, Template OptionalString
}

func LoadShipLoadoutFromArchive(archive *zip.Reader, shipID, designID *int) (*ShipLoadout, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return nil, err
	}
	return loadShipLoadout(data, shipID, designID)
}

func loadShipLoadout(data []byte, shipID, designID *int) (*ShipLoadout, error) {
	root, err := clausewitz.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse gamestate: %w", err)
	}
	return decodeShipLoadout(clausewitz.Value{Object: root}, shipID, designID), nil
}

func decodeShipLoadout(root clausewitz.Value, shipID, designID *int) *ShipLoadout {
	result := &ShipLoadout{ShipID: shipID, DesignID: designID}
	if shipID != nil {
		result.Ship = loadoutRecord(root, "ships", *shipID)
		if result.Ship.Issue != "" {
			return result
		}
		value := result.Ship.Value
		result.FleetRef = savedPathInt(value, "fleet")
		result.SameNumericFleetID = loadoutRecord(root, "fleet", *shipID).Issue == ""
		if id := result.FleetRef.Value; id != nil {
			result.Fleet = loadoutRecord(root, "fleet", *id)
			if result.Fleet.Issue == "" {
				field, recorded, unsupported := savedFieldPath(result.Fleet.Value, "ships")
				result.FleetShips = OptionalIntList{Recorded: recorded, Unsupported: unsupported}
				if recorded && !unsupported {
					result.FleetShips.UnmarshalClausewitz(field)
				}
			}
		}
		result.LegacyDesign = savedPathInt(value, "ship_design")
		result.ImplDesign = savedPathInt(value, "ship_design_implementation", "design")
		result.GrowthStage = savedPathInt(value, "ship_design_implementation", "growth_stage")
		result.Upgrade = savedPathInt(value, "ship_design_implementation", "upgrade")
		result.DesignID = unambiguousShipDesign(result.LegacyDesign, result.ImplDesign)
		result.ShipLayout = decodeLoadoutLayout(value, "design", "weapon", "component_slot")
	}
	if result.DesignID == nil {
		return result
	}
	result.Design = loadoutRecord(root, "ship_design", *result.DesignID)
	if result.Design.Issue != "" {
		return result
	}
	result.DesignLayouts, result.GrowthStagesUnsupported = decodeShipDesignLayouts(result.Design.Value)
	return result
}

func decodeShipDesignLayouts(value clausewitz.Value) ([]LoadoutLayout, bool) {
	var layouts []LoadoutLayout
	for _, pair := range value.Object.Pairs {
		if pair.Key == "section" || pair.Key == "ship_size" || pair.Key == "required_component" {
			layouts = append(layouts, decodeLoadoutLayout(value, "template", "component", "slot"))
			break
		}
	}
	stages, recorded, unsupported := savedFieldPath(value, "growth_stages")
	if recorded {
		if unsupported || (!stages.IsList() && (!stages.IsObject() || len(stages.Object.Pairs) != 0)) {
			return layouts, true
		}
		for i, stage := range stages.List {
			layout := decodeLoadoutLayout(stage, "template", "component", "slot")
			layout.Stage = &i
			layouts = append(layouts, layout)
		}
	}
	return layouts, false
}

func unambiguousShipDesign(legacy, implementation OptionalIntRef) *int {
	if legacy.Unsupported || implementation.Unsupported || (legacy.Value != nil && implementation.Value != nil && *legacy.Value != *implementation.Value) {
		return nil
	}
	if implementation.Value != nil {
		return implementation.Value
	}
	return legacy.Value
}

func loadoutRecord(root clausewitz.Value, tableName string, id int) LoadoutRecord {
	table, recorded, unsupported := savedFieldPath(root, tableName)
	switch {
	case !recorded:
		return LoadoutRecord{Issue: "table not recorded"}
	case unsupported || !table.IsObject():
		return LoadoutRecord{Issue: "table unsupported or duplicate"}
	}
	var record LoadoutRecord
	count := 0
	for _, pair := range table.Object.Pairs {
		key, err := strconv.Atoi(pair.Key)
		if err == nil && key == id {
			record.Value = pair.Value
			count++
		}
	}
	switch {
	case count == 0:
		record.Issue = "record unavailable"
	case count > 1:
		record.Issue = "duplicate saved record ID"
	case record.Value.IsScalar() && record.Value.Scalar == "none":
		record.Issue = `saved record is "none"`
	case !record.Value.IsObject():
		record.Issue = "record shape unsupported"
	}
	return record
}

func decodeLoadoutLayout(value clausewitz.Value, sectionTemplate, componentKey, componentSlot string) LoadoutLayout {
	layout := LoadoutLayout{Value: value}
	if !value.IsObject() {
		return layout
	}
	for _, pair := range value.Object.Pairs {
		if pair.Key != "section" {
			continue
		}
		section := LoadoutSection{Value: pair.Value, Slot: savedFieldString(pair.Value, "slot"), Template: savedFieldString(pair.Value, sectionTemplate)}
		if pair.Value.IsObject() {
			for _, field := range pair.Value.Object.Pairs {
				if field.Key == componentKey {
					section.Components = append(section.Components, LoadoutComponent{
						Value: field.Value, Slot: savedFieldString(field.Value, componentSlot), Template: savedFieldString(field.Value, "template"),
					})
				}
			}
		}
		layout.Sections = append(layout.Sections, section)
	}
	return layout
}

type LoadoutComparison struct {
	SectionStatus                              string
	Matched, Different, Unresolved, DesignOnly int
}

func CompareShipLoadout(ship, design LoadoutLayout) LoadoutComparison {
	actualSections := loadoutSectionsBySlot(ship.Sections)
	designSections := loadoutSectionsBySlot(design.Sections)
	result := LoadoutComparison{SectionStatus: "exact saved section pairs agree"}
	switch {
	case len(ship.Sections) == 0:
		result.SectionStatus = "ship sections not recorded"
	case !design.Value.IsObject() || actualSections == nil || designSections == nil:
		result.SectionStatus = "section comparison unresolved (unsupported or duplicate slot/template)"
	case len(actualSections) != len(designSections):
		result.SectionStatus = "saved section pairs differ"
	}
	for _, section := range ship.Sections {
		planned, found := designSections[section.Slot.Value]
		matchingSection := actualSections != nil && found && section.Template.Value == planned.Template.Value
		if !matchingSection {
			if result.SectionStatus == "exact saved section pairs agree" {
				result.SectionStatus = "saved section pairs differ"
			}
			result.Unresolved += len(section.Components)
			continue
		}
		actualSlots := loadoutComponentsBySlot(section.Components)
		plannedSlots := loadoutComponentsBySlot(planned.Components)
		for _, weapon := range section.Components {
			actual := actualSlots[weapon.Slot.Value]
			design := plannedSlots[weapon.Slot.Value]
			switch {
			case !usableLoadoutString(weapon.Slot) || !usableLoadoutString(weapon.Template) || len(actual) != 1 || len(design) != 1 || !usableLoadoutString(design[0].Template):
				result.Unresolved++
			case design[0].Template.Value != weapon.Template.Value:
				result.Different++
			default:
				result.Matched++
			}
		}
		for _, component := range planned.Components {
			if !usableLoadoutString(component.Slot) {
				result.Unresolved++
			} else if len(actualSlots[component.Slot.Value]) == 0 {
				result.DesignOnly++
			}
		}
	}
	return result
}

func usableLoadoutString(value OptionalString) bool {
	return value.Recorded && !value.Unsupported && value.Value != ""
}

func loadoutSectionsBySlot(sections []LoadoutSection) map[string]LoadoutSection {
	result := make(map[string]LoadoutSection, len(sections))
	for _, section := range sections {
		if !usableLoadoutString(section.Slot) || !usableLoadoutString(section.Template) {
			return nil
		}
		if _, duplicate := result[section.Slot.Value]; duplicate {
			return nil
		}
		result[section.Slot.Value] = section
	}
	return result
}

func loadoutComponentsBySlot(components []LoadoutComponent) map[string][]LoadoutComponent {
	result := make(map[string][]LoadoutComponent, len(components))
	for _, component := range components {
		if usableLoadoutString(component.Slot) {
			result[component.Slot.Value] = append(result[component.Slot.Value], component)
		}
	}
	return result
}
