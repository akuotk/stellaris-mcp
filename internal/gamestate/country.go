package gamestate

import (
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type Country struct {
	SpecialProjects             CountrySpecialProjects      `clausewitz:"events"`
	LastCompletedSpecialProject OptionalString              `clausewitz:"last_completed_special_project"`
	FleetSizeRaw                *float64                    `clausewitz:"fleet_size"`
	UsedNavalCapRaw             *float64                    `clausewitz:"used_naval_capacity"`
	StarbaseCapRaw              *int                        `clausewitz:"starbase_capacity"`
	NumPopsRaw                  *int                        `clausewitz:"num_sapient_pops"`
	EmpireSizeRaw               *int                        `clausewitz:"empire_size"`
	CouncilGovernment           CouncilGovernment           `clausewitz:"government"`
	Relics                      OptionalStringList          `clausewitz:"relics"`
	LastActivatedRelic          OptionalString              `clausewitz:"last_activated_relic"`
	LastReceivedRelic           OptionalString              `clausewitz:"last_received_relic"`
	GrandArchiveModules         CountryGrandArchiveModules  `clausewitz:"modules"`
	SpeciesRightsModules        CountrySpeciesRightsModules `clausewitz:"modules"`
	FleetTemplateRefs           FleetTemplateManagerRefs    `clausewitz:"fleet_template_manager"`
	DiplomaticModules           CountryDiplomaticModules    `clausewitz:"modules"`
	OverlordRef                 OptionalIntRef              `clausewitz:"overlord"`
	SubjectRefs                 OptionalIntList             `clausewitz:"subjects"`
	SubjectDate                 OptionalString              `clausewitz:"subject_date"`
	OwnedMegastructureRefs      OptionalIntList             `clausewitz:"owned_megastructures"`
	TrackAllSituations          OptionalString              `clausewitz:"track_all_situations"`
	SectorRefs                  CountrySectorRefs           `clausewitz:"sectors"`
	VictoryRankRaw              *int                        `clausewitz:"victory_rank"`
	EconomyModules              CountryEconomyModules       `clausewitz:"modules"`
	ActivePolicies              ActivePolicyList            `clausewitz:"active_policies"`
	Edicts                      EdictList                   `clausewitz:"edicts"`
	PolicyFlags                 OptionalStringList          `clausewitz:"policy_flags"`
	MilitaryPowerRaw            *float64                    `clausewitz:"military_power"`
	EconomyPowerRaw             *float64                    `clausewitz:"economy_power"`
	TechPowerRaw                *float64                    `clausewitz:"tech_power"`
	Name                        LocalizedName               `clausewitz:"name"`
	Adjective                   LocalizedName               `clausewitz:"adjective"`
	Ethos                       Ethos                       `clausewitz:"ethos"`
	Government                  Government                  `clausewitz:"government"`
	TechStatus                  TechStatus                  `clausewitz:"tech_status"`
	Budget                      Budget                      `clausewitz:"budget"`
	RelationsManager            RelationsManager            `clausewitz:"relations_manager"`
	FleetsManager               FleetsManager               `clausewitz:"fleets_manager"`
	Capital                     int                         `clausewitz:"capital"`
	MilitaryPower               float64                     `clausewitz:"military_power"`
	EconomyPower                float64                     `clausewitz:"economy_power"`
	TechPower                   float64                     `clausewitz:"tech_power"`
	FleetSize                   float64                     `clausewitz:"fleet_size"`
	UsedNavalCap                float64                     `clausewitz:"used_naval_capacity"`
	NavalCap                    *float64                    `clausewitz:"naval_capacity"`
	StarbaseCap                 int                         `clausewitz:"starbase_capacity"`
	NumPops                     int                         `clausewitz:"num_sapient_pops"`
	EmpireSize                  int                         `clausewitz:"empire_size"`
	VictoryRank                 int                         `clausewitz:"victory_rank"`
	OwnedPlanets                []int                       `clausewitz:"owned_planets"`
	OwnedLeaders                []int                       `clausewitz:"owned_leaders"`
	OwnedArmies                 []int                       `clausewitz:"owned_armies"`
	Traditions                  OptionalStringList          `clausewitz:"traditions"`
	AscensionPerks              OptionalStringList          `clausewitz:"ascension_perks"`
	TraditionCategories         OptionalStringList          `clausewitz:"tradition_categories"`
	LastPickedTradition         OptionalString              `clausewitz:"last_picked_tradition"`
}

// Duplicate saved fields cannot be treated as one authoritative value.
func (c *Country) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields Country
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		return err
	}
	*c = Country(decoded)
	if uniqueSavedFields(value, "events") != nil {
		c.SpecialProjects = CountrySpecialProjects{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "fleets_manager") != nil {
		c.FleetsManager.OwnershipAmbiguous = true
	}
	if uniqueSavedFields(value, "last_completed_special_project") != nil {
		c.LastCompletedSpecialProject = OptionalString{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "government") != nil {
		c.CouncilGovernment = CouncilGovernment{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "relics") != nil {
		c.Relics = OptionalStringList{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "last_activated_relic") != nil {
		c.LastActivatedRelic = OptionalString{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "last_received_relic") != nil {
		c.LastReceivedRelic = OptionalString{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "overlord") != nil {
		c.OverlordRef = OptionalIntRef{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "subjects") != nil {
		c.SubjectRefs = OptionalIntList{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "subject_date") != nil {
		c.SubjectDate = OptionalString{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "owned_megastructures") != nil {
		c.OwnedMegastructureRefs = OptionalIntList{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "track_all_situations") != nil {
		c.TrackAllSituations = OptionalString{Recorded: true, Unsupported: true}
	}
	return nil
}

// Optional fields isolate unsupported shapes instead of preventing other save data from loading.
type OptionalStringList struct {
	Recorded    bool
	Values      []string
	Unsupported bool
}

func (v *OptionalStringList) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = OptionalStringList{Recorded: true}
	if err := clausewitz.DecodeValue(value, &v.Values); err != nil {
		v.Values = nil
		v.Unsupported = true
	}
	return nil
}

type OptionalString struct {
	Recorded    bool
	Value       string
	Unsupported bool
}

func (v *OptionalString) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = OptionalString{Recorded: true}
	if !value.IsScalar() {
		v.Unsupported = true
	} else {
		v.Value = value.Scalar
	}
	return nil
}

type Ethos struct {
	Ethic  []string `clausewitz:"ethic"`
	Ethics []string `clausewitz:"ethics"`
}

type Government struct {
	Type               string   `clausewitz:"type"`
	Authority          string   `clausewitz:"authority"`
	Civics             []string `clausewitz:"civics"`
	Origin             string   `clausewitz:"origin"`
	CouncilPositions   []int    `clausewitz:"council_positions"`
	PickedCouncilTypes []string `clausewitz:"picked_council_types"`
}

type TechStatus struct {
	Technology           []string           `clausewitz:"technology"`
	CompletedLevels      []*int             `clausewitz:"-"`
	PhysicsQueue         []ResearchItem     `clausewitz:"physics_queue"`
	SocietyQueue         []ResearchItem     `clausewitz:"society_queue"`
	EngineeringQueue     []ResearchItem     `clausewitz:"engineering_queue"`
	Alternatives         TechAlternatives   `clausewitz:"alternatives"`
	StoredForTech        map[string]float64 `clausewitz:"stored_techpoints_for_tech"`
	StoredPoints         []float64          `clausewitz:"stored_techpoints"`
	AlwaysAvailable      []string           `clausewitz:"always_available_tech"`
	LastIncreased        string             `clausewitz:"last_increased_tech"`
	AutoPhysics          *string            `clausewitz:"auto_researching_physics"`
	AutoSociety          *string            `clausewitz:"auto_researching_society"`
	AutoEngineering      *string            `clausewitz:"auto_researching_engineering"`
	Potential            []PotentialTech    `clausewitz:"-"`
	PotentialBlocks      int                `clausewitz:"-"`
	PotentialUnsupported bool               `clausewitz:"-"`
}

type PotentialTech struct {
	Key   string
	Value *string
	Kind  string
}

// UnmarshalClausewitz pairs a technology only with its immediately following level.
func (t *TechStatus) UnmarshalClausewitz(v clausewitz.Value) error {
	type plain TechStatus
	var p plain
	if err := clausewitz.DecodeValue(v, &p); err != nil {
		return err
	}
	if !v.IsObject() {
		return fmt.Errorf("tech_status is not an object")
	}
	for i, pair := range v.Object.Pairs {
		if pair.Key == "potential" {
			p.PotentialBlocks++
			if !pair.Value.IsObject() {
				p.PotentialUnsupported = true
				continue
			}
			for _, item := range pair.Value.Object.Pairs {
				entry := PotentialTech{Key: item.Key}
				switch {
				case item.Value.IsScalar():
					value := item.Value.Scalar
					entry.Value = &value
					entry.Kind = "scalar"
				case item.Value.IsObject():
					entry.Kind = "object"
				default:
					entry.Kind = "list"
				}
				p.Potential = append(p.Potential, entry)
			}
		}
		if pair.Key != "technology" {
			continue
		}
		var level *int
		if i+1 < len(v.Object.Pairs) && v.Object.Pairs[i+1].Key == "level" && v.Object.Pairs[i+1].Value.IsScalar() {
			if n, err := strconv.Atoi(v.Object.Pairs[i+1].Value.Scalar); err == nil {
				level = &n
			}
		}
		p.CompletedLevels = append(p.CompletedLevels, level)
	}
	*t = TechStatus(p)
	return nil
}

type ResearchItem struct {
	SpecialProjectRef OptionalIntRef `clausewitz:"special_project"`
	TechnologyRaw     *string        `clausewitz:"technology"`
	ProgressRaw       *float64       `clausewitz:"progress"`
	DateRaw           *string        `clausewitz:"date"`
	Technology        string         `clausewitz:"technology"`
	Progress          float64        `clausewitz:"progress"`
	Date              string         `clausewitz:"date"`
}

// A duplicate project reference is not a unique saved link.
func (r *ResearchItem) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields ResearchItem
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		return err
	}
	*r = ResearchItem(decoded)
	if uniqueSavedFields(value, "special_project") != nil {
		r.SpecialProjectRef = OptionalIntRef{Recorded: true, Unsupported: true}
	}
	return nil
}

type TechAlternatives struct {
	Physics     []string `clausewitz:"physics"`
	Society     []string `clausewitz:"society"`
	Engineering []string `clausewitz:"engineering"`
}

type Budget struct {
	CurrentMonth BudgetMonth `clausewitz:"current_month"`
	LastMonth    BudgetMonth `clausewitz:"last_month"`
}

type BudgetMonth struct {
	Recorded bool                 `clausewitz:"-"`
	Income   map[string]Resources `clausewitz:"income"`
	Expenses map[string]Resources `clausewitz:"expenses"`
	Balance  map[string]Resources `clausewitz:"balance"`
}

func (m *BudgetMonth) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields BudgetMonth
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		return err
	}
	*m = BudgetMonth(decoded)
	m.Recorded = true
	return nil
}

type Resources = map[string]float64

type RelationsManager struct {
	Relation []Relation `clausewitz:"relation"`
}

type Relation struct {
	CountryID          *int     `clausewitz:"country"`
	ContactRaw         *string  `clausewitz:"contact"`
	TrustRaw           *float64 `clausewitz:"trust"`
	RelationCurrentRaw *float64 `clausewitz:"relation_current"`
	Country            int      `clausewitz:"country"`
	Contact            bool     `clausewitz:"contact"`
	Trust              float64  `clausewitz:"trust"`
	RelationCurrent    float64  `clausewitz:"relation_current"`
}

type FleetsManager struct {
	OwnedFleets        []OwnedFleet `clausewitz:"owned_fleets"`
	OwnershipAmbiguous bool         `clausewitz:"-"`
}

func (m *FleetsManager) UnmarshalClausewitz(value clausewitz.Value) error {
	*m = FleetsManager{}
	if uniqueSavedFields(value, "owned_fleets") != nil {
		m.OwnershipAmbiguous = true
		for _, pair := range value.Object.Pairs {
			if pair.Key != "owned_fleets" {
				continue
			}
			var entries []OwnedFleet
			if clausewitz.DecodeValue(pair.Value, &entries) == nil {
				m.OwnedFleets = append(m.OwnedFleets, entries...)
			}
		}
		return nil
	}
	type fields FleetsManager
	if err := clausewitz.DecodeValue(value, (*fields)(m)); err != nil {
		return err
	}
	for _, entry := range m.OwnedFleets {
		if entry.ReferenceAmbiguous {
			m.OwnershipAmbiguous = true
		}
	}
	return nil
}

type OwnedFleet struct {
	FleetID            *int `clausewitz:"fleet"`
	Fleet              int  `clausewitz:"fleet"`
	ReferenceAmbiguous bool `clausewitz:"-"`
}

func (f *OwnedFleet) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields OwnedFleet
	if err := clausewitz.DecodeValue(value, (*fields)(f)); err != nil {
		return err
	}
	f.ReferenceAmbiguous = uniqueSavedFields(value, "fleet") != nil
	return nil
}
