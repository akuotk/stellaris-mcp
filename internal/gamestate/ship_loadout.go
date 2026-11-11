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
	References               []ShipSavedReference
	Inbound                  []ShipInboundReference
	InboundCount             int
	InboundShips             int
	InboundUnscanned         int
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
		result.References = shipReferenceFields(value)
		resolveShipReferences(root, result.References)
		result.Inbound, result.InboundCount, result.InboundShips, result.InboundUnscanned = shipInboundReferences(root, *shipID)
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

// ShipSavedReference is an explicit saved reference field inside a ship record.
// Table matches are numeric candidates; the saved type field is kept raw.
type ShipSavedReference struct {
	Path, Table, Status string
	ID                  *int
	Type                *OptionalString
}

// ShipInboundReference is a saved ship-table reference in a ship record (including the selected one) naming the selected ship ID.
type ShipInboundReference struct {
	ShipID int
	Path   string
	Type   *OptionalString
}

const ShipInboundPreviewLimit = 20

var shipScalarRefTables = map[string]string{
	"leader": "leaders", "army": "army", "kill_target": "ships", "original_owner": "country", "design_upgrade": "ship_design",
}

// shipRefKeySlot numbers the top-level ship fields that carry references, so repeated
// fields can be detected without allocating per record.
func shipRefKeySlot(key string) int {
	switch key {
	case "leader":
		return 0
	case "army":
		return 1
	case "kill_target":
		return 2
	case "original_owner":
		return 3
	case "design_upgrade":
		return 4
	case "colonization_data":
		return 5
	case "homepop":
		return 6
	case "auras":
		return 7
	case "targeting_module":
		return 8
	}
	return -1
}

const unsupportedShipRef = "reference unsupported or duplicate"

// shipReferenceFields lists explicit reference fields in saved order without resolving them.
// A repeated top-level reference field is listed once, at its first position, as unsupported.
// Non-object containers (for example weapon=none) have no nested reference fields.
func shipReferenceFields(ship clausewitz.Value) []ShipSavedReference {
	return collectShipReferences(ship, nil)
}

// collectShipReferences returns all references, or with match set only ships-table
// references naming that ID (paths are formatted only for returned entries).
func collectShipReferences(ship clausewitz.Value, match *int) []ShipSavedReference {
	var refs []ShipSavedReference
	var counts [9]int
	for _, pair := range ship.Object.Pairs {
		if slot := shipRefKeySlot(pair.Key); slot >= 0 {
			counts[slot]++
		}
	}
	keep := func(table string, id *int) bool {
		return match == nil || (table == "ships" && id != nil && *id == *match)
	}
	add := func(path func() string, table string, ref OptionalIntRef, typ *OptionalString) {
		if !keep(table, ref.Value) {
			return
		}
		item := ShipSavedReference{Path: path(), Table: table, ID: ref.Value, Type: typ}
		if ref.Unsupported || ref.Value == nil {
			item.Status = unsupportedShipRef
		}
		refs = append(refs, item)
	}
	unsupported := func(path, table string) {
		if match == nil {
			refs = append(refs, ShipSavedReference{Path: path, Table: table, Status: unsupportedShipRef})
		}
	}
	var seen [9]bool
	section := 0
	for _, pair := range ship.Object.Pairs {
		if pair.Key == "section" {
			weapon := 0
			if pair.Value.IsObject() {
				for _, field := range pair.Value.Object.Pairs {
					if field.Key != "weapon" {
						continue
					}
					s, w := section, weapon
					refs = appendShipTargetRef(refs, field.Value, match, func() string { return fmt.Sprintf("section[%d].weapon[%d].target", s, w) }, "target")
					refs = appendShipTargetRef(refs, field.Value, match, func() string { return fmt.Sprintf("section[%d].weapon[%d].targeting_module.target", s, w) }, "targeting_module", "target")
					weapon++
				}
			}
			section++
			continue
		}
		slot := shipRefKeySlot(pair.Key)
		if slot < 0 || seen[slot] {
			continue
		}
		seen[slot] = true
		repeated := counts[slot] > 1
		key := pair.Key
		switch slot {
		case 0, 1, 2, 3, 4:
			table := shipScalarRefTables[key]
			if repeated {
				unsupported(key, table)
				continue
			}
			add(func() string { return key }, table, savedPathInt(ship, key), nil)
		case 5, 6:
			switch {
			case repeated:
				unsupported(key+".species", "species_db")
			case pair.Value.IsObject():
				if ref := savedPathInt(ship, key, "species"); ref.Recorded {
					add(func() string { return key + ".species" }, "species_db", ref, nil)
				}
			}
		case 7:
			auras := pair.Value
			if repeated || auras.Mixed || (!auras.IsList() && !(auras.IsObject() && len(auras.Object.Pairs) == 0)) {
				unsupported("auras", "ships")
				continue
			}
			for i, aura := range auras.List {
				if source, recorded, bad := savedObjectPath(aura, "source"); recorded && (bad || source.IsObject()) {
					if ref := savedPathInt(aura, "source", "ship"); ref.Recorded {
						index := i
						add(func() string { return fmt.Sprintf("auras[%d].source.ship", index) }, "ships", ref, nil)
					}
				}
			}
		case 8:
			if repeated {
				unsupported("targeting_module.target.id", "ships")
				continue
			}
			refs = appendShipTargetRef(refs, ship, match, func() string { return "targeting_module.target" }, "targeting_module", "target")
		}
	}
	return refs
}

// savedObjectPath follows path like savedFieldPath, but a non-object intermediate value
// has no fields, so the path counts as not recorded. Duplicate keys remain unsupported.
func savedObjectPath(value clausewitz.Value, path ...string) (clausewitz.Value, bool, bool) {
	for i, key := range path {
		if !value.IsObject() {
			return clausewitz.Value{}, false, false
		}
		field, recorded, unsupported := savedFieldPath(value, key)
		if !recorded || unsupported {
			return field, recorded, unsupported
		}
		if i == len(path)-1 {
			return field, true, false
		}
		value = field
	}
	return value, true, false
}

// appendShipTargetRef reads an {id type} block; only the id is compared with the ships table.
func appendShipTargetRef(refs []ShipSavedReference, value clausewitz.Value, match *int, label func() string, path ...string) []ShipSavedReference {
	target, recorded, unsupported := savedObjectPath(value, path...)
	if !recorded {
		return refs
	}
	if unsupported || !target.IsObject() {
		if match != nil {
			return refs
		}
		return append(refs, ShipSavedReference{Path: label() + ".id", Table: "ships", Status: unsupportedShipRef})
	}
	ref := savedPathInt(target, "id")
	if match != nil && (ref.Value == nil || *ref.Value != *match) {
		return refs
	}
	item := ShipSavedReference{Path: label() + ".id", Table: "ships", ID: ref.Value}
	if typ := savedFieldString(target, "type"); typ.Recorded {
		item.Type = &typ
	}
	if !ref.Recorded {
		item.Status = "id not recorded"
	} else if ref.Unsupported || ref.Value == nil {
		item.Status = unsupportedShipRef
	}
	return append(refs, item)
}

func resolveShipReferences(root clausewitz.Value, refs []ShipSavedReference) {
	for i := range refs {
		if refs[i].Status != "" || refs[i].ID == nil {
			continue
		}
		record := loadoutRecord(root, refs[i].Table, *refs[i].ID)
		refs[i].Status = record.Issue
		if refs[i].Status == "" {
			refs[i].Status = "unique saved record"
		}
	}
}

// shipInboundReferences scans all ship records for ship-table references naming shipID.
func shipInboundReferences(root clausewitz.Value, shipID int) (refs []ShipInboundReference, count, ships int, unscanned int) {
	table, recorded, unsupported := savedFieldPath(root, "ships")
	if !recorded || unsupported || !table.IsObject() {
		return nil, 0, 0, -1
	}
	for _, pair := range table.Object.Pairs {
		source, err := strconv.Atoi(pair.Key)
		if err != nil || !pair.Value.IsObject() {
			if !(pair.Value.IsScalar() && pair.Value.Scalar == "none") {
				unscanned++
			}
			continue
		}
		matched := false
		for _, ref := range collectShipReferences(pair.Value, &shipID) {
			count++
			matched = true
			if len(refs) < ShipInboundPreviewLimit {
				refs = append(refs, ShipInboundReference{ShipID: source, Path: ref.Path, Type: ref.Type})
			}
		}
		if matched {
			ships++
		}
	}
	return refs, count, ships, unscanned
}
