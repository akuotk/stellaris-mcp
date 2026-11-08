package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

// Federations are stored as an ID-indexed table, including ID zero.
type Federation struct {
	Name                   LocalizedName         `clausewitz:"name"`
	Leader                 *int                  `clausewitz:"-"`
	LeaderRef              OptionalIntRef        `clausewitz:"-"`
	Members                []int                 `clausewitz:"-"`
	MemberRefs             OptionalIntList       `clausewitz:"-"`
	Associates             []int                 `clausewitz:"-"`
	AssociateRefs          OptionalIntList       `clausewitz:"-"`
	StartDate              string                `clausewitz:"start_date"`
	FederationProgression  FederationProgression `clausewitz:"federation_progression"`
	ProgressionUnsupported bool                  `clausewitz:"-"`
	ShipDesignCollection   struct {
		ShipDesign []int `clausewitz:"ship_design"`
	} `clausewitz:"ship_design_collection"`
	ProposedLawVotesCooldowns []FederationLawCooldown `clausewitz:"proposed_law_votes_cooldowns"`
}

func (f *Federation) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields Federation
	var decoded fields
	if !value.IsObject() {
		return clausewitz.DecodeValue(value, &decoded)
	}
	var rest clausewitz.Object
	for _, pair := range value.Object.Pairs {
		if pair.Key != "federation_progression" {
			rest.Pairs = append(rest.Pairs, pair)
		}
	}
	if err := clausewitz.DecodeValue(clausewitz.Value{Object: &rest}, &decoded); err != nil {
		return err
	}
	*f = Federation(decoded)
	f.LeaderRef = savedPathInt(value, "leader")
	f.Leader = f.LeaderRef.Value
	f.MemberRefs, f.AssociateRefs = communityIntList(value, "members"), communityIntList(value, "associates")
	f.Members, f.Associates = f.MemberRefs.Values, f.AssociateRefs.Values
	progression, recorded, unsupported := savedFieldPath(value, "federation_progression")
	if unsupported || (recorded && clausewitz.DecodeValue(progression, &f.FederationProgression) != nil) {
		f.FederationProgression = FederationProgression{}
		f.ProgressionUnsupported = true
	} else {
		p := &f.FederationProgression
		p.EnvoyRefs = communityIntList(value, "federation_progression", "envoy")
		p.Envoy = p.EnvoyRefs.Values
	}
	return nil
}

type FederationProgression struct {
	FederationType string                             `clausewitz:"federation_type"`
	Experience     *float64                           `clausewitz:"experience"`
	BaseCohesion   *float64                           `clausewitz:"base_cohesion"`
	Cohesion       *float64                           `clausewitz:"cohesion"`
	Levels         *int                               `clausewitz:"levels"`
	Laws           map[string]string                  `clausewitz:"laws"`
	ActionSettings map[string]FederationActionSetting `clausewitz:"action_settings"`
	Envoy          []int                              `clausewitz:"-"`
	EnvoyRefs      OptionalIntList                    `clausewitz:"-"`
	Perks          []FederationPerk                   `clausewitz:"perks"`
	TimedModifier  struct {
		Items []FederationModifier `clausewitz:"items"`
	} `clausewitz:"timed_modifier"`
	SuccessionType         string            `clausewitz:"succession_type"`
	SuccessionTerm         string            `clausewitz:"succession_term"`
	LastSuccessionDate     string            `clausewitz:"last_succession_date"`
	Expired                string            `clausewitz:"expired"`
	Flags                  map[string]string `clausewitz:"flags"`
	OnlyLeaderBuildsFleets *bool             `clausewitz:"only_leader_builds_fleets"`
	AllowSubjectsToJoin    *bool             `clausewitz:"allow_subjects_to_join"`
	EqualVotingPower       *bool             `clausewitz:"equal_voting_power"`
	FreeMigration          *bool             `clausewitz:"free_migration"`
	ResearchAgreement      *bool             `clausewitz:"research_agreement"`
	FailedVoteHalfPrice    *bool             `clausewitz:"failed_vote_half_price"`
	ResearchSharing        *bool             `clausewitz:"research_sharing"`
}

type FederationActionSetting struct {
	VoteType       string `clausewitz:"vote_type"`
	AcceptanceType string `clausewitz:"acceptance_type"`
}

type FederationPerk struct {
	Level *int   `clausewitz:"level"`
	Type  string `clausewitz:"type"`
}

type FederationModifier struct {
	Modifier   string   `clausewitz:"modifier"`
	Days       *int     `clausewitz:"days"`
	Multiplier *float64 `clausewitz:"multiplier"`
}

type FederationLawCooldown struct {
	Token     string `clausewitz:"token"`
	Target    string `clausewitz:"target"`
	StartDate string `clausewitz:"start_date"`
}
