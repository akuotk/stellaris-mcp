package gamestate

// Federations are stored as an ID-indexed table, including ID zero.
type Federation struct {
	Name                  LocalizedName         `clausewitz:"name"`
	Leader                *int                  `clausewitz:"leader"`
	Members               []int                 `clausewitz:"members"`
	Associates            []int                 `clausewitz:"associates"`
	StartDate             string                `clausewitz:"start_date"`
	FederationProgression FederationProgression `clausewitz:"federation_progression"`
	ShipDesignCollection  struct {
		ShipDesign []int `clausewitz:"ship_design"`
	} `clausewitz:"ship_design_collection"`
	ProposedLawVotesCooldowns []FederationLawCooldown `clausewitz:"proposed_law_votes_cooldowns"`
}

type FederationProgression struct {
	FederationType string                             `clausewitz:"federation_type"`
	Experience     *float64                           `clausewitz:"experience"`
	BaseCohesion   *float64                           `clausewitz:"base_cohesion"`
	Cohesion       *float64                           `clausewitz:"cohesion"`
	Levels         *int                               `clausewitz:"levels"`
	Laws           map[string]string                  `clausewitz:"laws"`
	ActionSettings map[string]FederationActionSetting `clausewitz:"action_settings"`
	Envoy          []int                              `clausewitz:"envoy"`
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
