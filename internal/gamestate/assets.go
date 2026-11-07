package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type AgreementStore struct {
	ConnectionRecordTable[Agreement]
}

func (s *AgreementStore) UnmarshalClausewitz(value clausewitz.Value) error {
	*s = AgreementStore{}
	if !value.IsObject() || uniqueSavedFields(value, "agreements") != nil {
		s.TableIssue = true
		return nil
	}
	for _, pair := range value.Object.Pairs {
		if pair.Key == "agreements" {
			return s.ConnectionRecordTable.UnmarshalClausewitz(pair.Value)
		}
	}
	return nil
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

func (a *Agreement) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "owner", "target", "active_status", "date_added", "date_changed", "term_data", "subject_specialization"); err != nil {
		return err
	}
	type fields Agreement
	return clausewitz.DecodeValue(value, (*fields)(a))
}

type AgreementTerm struct {
	Key   string `clausewitz:"key"`
	Value string `clausewitz:"value"`
}

func (t *AgreementTerm) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "key", "value"); err != nil {
		return err
	}
	type fields AgreementTerm
	return clausewitz.DecodeValue(value, (*fields)(t))
}

type AgreementTerms struct {
	Preset                        string          `clausewitz:"agreement_preset"`
	CanIntegrate                  *string         `clausewitz:"can_subject_be_integrated"`
	CanDiplomacy                  *string         `clausewitz:"can_subject_do_diplomacy"`
	CanVote                       *string         `clausewitz:"can_subject_vote"`
	HasAccess                     *string         `clausewitz:"has_access"`
	HasSensors                    *string         `clausewitz:"has_sensors"`
	JoinsOverlordWars             *string         `clausewitz:"joins_overlord_wars"`
	CallsOverlordToWar            *string         `clausewitz:"calls_overlord_to_war"`
	Expansion                     *string         `clausewitz:"subject_expansion_type"`
	ForcedInitialLoyalty          *string         `clausewitz:"forced_initial_loyalty"`
	FirstRenegotiationHasCooldown *string         `clausewitz:"has_cooldown_on_first_renegotiation"`
	Discrete                      []AgreementTerm `clausewitz:"discrete_terms"`
	Resources                     []AgreementTerm `clausewitz:"resource_terms"`
}

func (t *AgreementTerms) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "agreement_preset", "can_subject_be_integrated", "can_subject_do_diplomacy", "can_subject_vote", "has_access", "has_sensors", "joins_overlord_wars", "calls_overlord_to_war", "subject_expansion_type", "forced_initial_loyalty", "has_cooldown_on_first_renegotiation"); err != nil {
		return err
	}
	type fields AgreementTerms
	return clausewitz.DecodeValue(value, (*fields)(t))
}

type AgreementSpecialization struct {
	Level      *string             `clausewitz:"level"`
	Experience *string             `clausewitz:"experience"`
	Conversion AgreementConversion `clausewitz:"subject_conversion_process"`
}

func (s *AgreementSpecialization) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "level", "experience", "subject_conversion_process"); err != nil {
		return err
	}
	type fields AgreementSpecialization
	return clausewitz.DecodeValue(value, (*fields)(s))
}

type AgreementConversion struct {
	Progress   *string `clausewitz:"progress"`
	InProgress *string `clausewitz:"in_progress"`
	Done       *string `clausewitz:"done"`
	Ignore     *string `clausewitz:"ignore"`
}

func (c *AgreementConversion) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "progress", "in_progress", "done", "ignore"); err != nil {
		return err
	}
	type fields AgreementConversion
	return clausewitz.DecodeValue(value, (*fields)(c))
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
	Hyperlanes   HyperlaneList   `clausewitz:"hyperlane"`
	BypassRefs   OptionalIntList `clausewitz:"bypasses"`
	WormholeRefs OptionalIntList `clausewitz:"natural_wormholes"`
	SectorRef    OptionalIntRef  `clausewitz:"sector"`
	StarbaseRefs OptionalIntList `clausewitz:"starbases"`
	Name         LocalizedName   `clausewitz:"name"`
	Planets      []int           `clausewitz:"planet"`
}

// Keep duplicate references from being mistaken for a unique link.
func (s *GalacticSystem) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields GalacticSystem
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		return err
	}
	*s = GalacticSystem(decoded)
	if uniqueSavedFields(value, "starbases") != nil {
		s.StarbaseRefs = OptionalIntList{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "sector") != nil {
		s.SectorRef = OptionalIntRef{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "hyperlane") != nil {
		s.Hyperlanes = HyperlaneList{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "bypasses") != nil {
		s.BypassRefs = OptionalIntList{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "natural_wormholes") != nil {
		s.WormholeRefs = OptionalIntList{Recorded: true, Unsupported: true}
	}
	return nil
}
