package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type AgreementStore struct {
	Items map[int]Agreement `clausewitz:"agreements"`
}
type Agreement struct {
	Owner          *int                    `clausewitz:"owner"`
	Target         *int                    `clausewitz:"target"`
	Status         string                  `clausewitz:"active_status"`
	DateAdded      string                  `clausewitz:"date_added"`
	DateChanged    string                  `clausewitz:"date_changed"`
	Terms          AgreementTerms          `clausewitz:"term_data"`
	Specialization AgreementSpecialization `clausewitz:"subject_specialization"`
}
type AgreementTerm struct {
	Key   string `clausewitz:"key"`
	Value string `clausewitz:"value"`
}
type AgreementTerms struct {
	Preset             string          `clausewitz:"agreement_preset"`
	CanIntegrate       *string         `clausewitz:"can_subject_be_integrated"`
	CanDiplomacy       *string         `clausewitz:"can_subject_do_diplomacy"`
	CanVote            *string         `clausewitz:"can_subject_vote"`
	HasAccess          *string         `clausewitz:"has_access"`
	HasSensors         *string         `clausewitz:"has_sensors"`
	JoinsOverlordWars  *string         `clausewitz:"joins_overlord_wars"`
	CallsOverlordToWar *string         `clausewitz:"calls_overlord_to_war"`
	Expansion          *string         `clausewitz:"subject_expansion_type"`
	Discrete           []AgreementTerm `clausewitz:"discrete_terms"`
	Resources          []AgreementTerm `clausewitz:"resource_terms"`
}
type AgreementSpecialization struct {
	Level      *string `clausewitz:"level"`
	Experience *string `clausewitz:"experience"`
	Conversion struct {
		Progress   *string `clausewitz:"progress"`
		InProgress *string `clausewitz:"in_progress"`
		Done       *string `clausewitz:"done"`
	} `clausewitz:"subject_conversion_process"`
}
type SituationStore struct {
	Items map[int]Situation `clausewitz:"situations"`
}
type Situation struct {
	ProgressRaw          *float64 `clausewitz:"progress"`
	LastMonthProgressRaw *float64 `clausewitz:"last_month_progress"`
	Country              *int     `clausewitz:"country"`
	Type                 string   `clausewitz:"type"`
	Progress             float64  `clausewitz:"progress"`
	LastMonthProgress    float64  `clausewitz:"last_month_progress"`
	Approach             string   `clausewitz:"approach"`
}
type Megastructure struct {
	Owner      *int   `clausewitz:"owner"`
	Type       string `clausewitz:"type"`
	Coordinate struct {
		Origin *int `clausewitz:"origin"`
	} `clausewitz:"coordinate"`
}
type GalacticSystem struct {
	SectorRef    OptionalIntRef  `clausewitz:"sector"`
	StarbaseRefs OptionalIntList `clausewitz:"starbases"`
	Name         LocalizedName   `clausewitz:"name"`
	Planets      []int           `clausewitz:"planet"`
}

// Keep duplicate sector references from being mistaken for a unique link.
func (s *GalacticSystem) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields GalacticSystem
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		return err
	}
	*s = GalacticSystem(decoded)
	if uniqueSavedFields(value, "sector") != nil {
		s.SectorRef = OptionalIntRef{Recorded: true, Unsupported: true}
	}
	return nil
}
