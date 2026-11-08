package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type Leader struct {
	CountryID         *int           `clausewitz:"country"`
	Name              LeaderName     `clausewitz:"name"`
	Species           int            `clausewitz:"species"`
	Gender            string         `clausewitz:"gender"`
	Country           int            `clausewitz:"country"`
	Class             string         `clausewitz:"class"`
	Tier              string         `clausewitz:"tier"`
	Experience        float64        `clausewitz:"experience"`
	Age               int            `clausewitz:"age"`
	Level             int            `clausewitz:"level"`
	BonusSkillLevel   int            `clausewitz:"bonus_skill_level"`
	Job               string         `clausewitz:"job"`
	Ethic             string         `clausewitz:"ethic"`
	Planet            int            `clausewitz:"planet"`
	Traits            []string       `clausewitz:"traits"`
	Location          LeaderLocation `clausewitz:"location"`
	LocationAmbiguous bool           `clausewitz:"-"`
	CouncilLocation   LeaderLocation `clausewitz:"council_location"`
	RecruitmentDate   string         `clausewitz:"recruitment_date"`
}

func (l *Leader) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields Leader
	if err := clausewitz.DecodeValue(value, (*fields)(l)); err != nil {
		return err
	}
	l.LocationAmbiguous = uniqueSavedFields(value, "location") != nil
	for _, pair := range value.Object.Pairs {
		if pair.Key == "location" && uniqueSavedFields(pair.Value, "type", "assignment", "id") != nil {
			l.LocationAmbiguous = true
		}
	}
	return nil
}

type LeaderName struct {
	FullNames LocalizedName `clausewitz:"full_names"`
}

type LeaderLocation struct {
	Type        string `clausewitz:"type"`
	Assignment  string `clausewitz:"assignment"`
	ID          int    `clausewitz:"id"`
	IDRaw       *int   `clausewitz:"id"`
	Position    int    `clausewitz:"position"`
	PositionRaw *int   `clausewitz:"position"`
}
