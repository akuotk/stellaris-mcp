package gamestate

import (
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type Country struct {
	Name             LocalizedName    `clausewitz:"name"`
	Adjective        LocalizedName    `clausewitz:"adjective"`
	Ethos            Ethos            `clausewitz:"ethos"`
	Government       Government       `clausewitz:"government"`
	TechStatus       TechStatus       `clausewitz:"tech_status"`
	Budget           Budget           `clausewitz:"budget"`
	RelationsManager RelationsManager `clausewitz:"relations_manager"`
	FleetsManager    FleetsManager    `clausewitz:"fleets_manager"`
	Capital          int              `clausewitz:"capital"`
	MilitaryPower    float64          `clausewitz:"military_power"`
	EconomyPower     float64          `clausewitz:"economy_power"`
	TechPower        float64          `clausewitz:"tech_power"`
	FleetSize        float64          `clausewitz:"fleet_size"`
	UsedNavalCap     float64          `clausewitz:"used_naval_capacity"`
	NavalCap         *float64         `clausewitz:"naval_capacity"`
	StarbaseCap      int              `clausewitz:"starbase_capacity"`
	NumPops          int              `clausewitz:"num_sapient_pops"`
	EmpireSize       int              `clausewitz:"empire_size"`
	VictoryRank      int              `clausewitz:"victory_rank"`
	OwnedPlanets     []int            `clausewitz:"owned_planets"`
	OwnedLeaders     []int            `clausewitz:"owned_leaders"`
	Traditions       []string         `clausewitz:"traditions"`
	AscensionPerks   []string         `clausewitz:"ascension_perks"`
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
	Technology       []string           `clausewitz:"technology"`
	CompletedLevels  []*int             `clausewitz:"-"`
	PhysicsQueue     []ResearchItem     `clausewitz:"physics_queue"`
	SocietyQueue     []ResearchItem     `clausewitz:"society_queue"`
	EngineeringQueue []ResearchItem     `clausewitz:"engineering_queue"`
	Alternatives     TechAlternatives   `clausewitz:"alternatives"`
	StoredForTech    map[string]float64 `clausewitz:"stored_techpoints_for_tech"`
	StoredPoints     []float64          `clausewitz:"stored_techpoints"`
	AlwaysAvailable  []string           `clausewitz:"always_available_tech"`
	LastIncreased    string             `clausewitz:"last_increased_tech"`
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
	Technology string  `clausewitz:"technology"`
	Progress   float64 `clausewitz:"progress"`
	Date       string  `clausewitz:"date"`
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
	Income   map[string]Resources `clausewitz:"income"`
	Expenses map[string]Resources `clausewitz:"expenses"`
	Balance  map[string]Resources `clausewitz:"balance"`
}

type Resources = map[string]float64

type RelationsManager struct {
	Relation []Relation `clausewitz:"relation"`
}

type Relation struct {
	Country         int     `clausewitz:"country"`
	Contact         bool    `clausewitz:"contact"`
	Trust           float64 `clausewitz:"trust"`
	RelationCurrent float64 `clausewitz:"relation_current"`
}

type FleetsManager struct {
	OwnedFleets []OwnedFleet `clausewitz:"owned_fleets"`
}

type OwnedFleet struct {
	Fleet int `clausewitz:"fleet"`
}
