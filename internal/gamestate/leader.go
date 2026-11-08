package gamestate

import (
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type Leader struct {
	CountryID         *int              `clausewitz:"-"`
	CountryRef        OptionalIntRef    `clausewitz:"-"`
	SpeciesRef        OptionalIntRef    `clausewitz:"-"`
	PlanetRef         OptionalIntRef    `clausewitz:"-"`
	SavedDetails      []clausewitz.Pair `clausewitz:"-"`
	SavedExperience   OptionalString    `clausewitz:"-"`
	Name              LeaderName        `clausewitz:"name"`
	Species           int               `clausewitz:"-"`
	Gender            string            `clausewitz:"gender"`
	Country           int               `clausewitz:"-"`
	Class             string            `clausewitz:"class"`
	Tier              string            `clausewitz:"tier"`
	Experience        float64           `clausewitz:"-"`
	Age               int               `clausewitz:"age"`
	Level             int               `clausewitz:"level"`
	BonusSkillLevel   int               `clausewitz:"bonus_skill_level"`
	Job               string            `clausewitz:"job"`
	Ethic             string            `clausewitz:"ethic"`
	Planet            int               `clausewitz:"-"`
	Traits            []string          `clausewitz:"traits"`
	Location          LeaderLocation    `clausewitz:"location"`
	LocationAmbiguous bool              `clausewitz:"-"`
	CouncilLocation   LeaderLocation    `clausewitz:"council_location"`
	RecruitmentDate   string            `clausewitz:"recruitment_date"`
}

func (l *Leader) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields Leader
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		return err
	}
	*l = Leader(decoded)
	l.CountryRef, l.SpeciesRef, l.PlanetRef = savedPathInt(value, "country"), savedPathInt(value, "species"), savedPathInt(value, "planet")
	l.CountryID = l.CountryRef.Value
	if l.CountryID != nil {
		l.Country = *l.CountryID
	}
	if l.SpeciesRef.Value != nil {
		l.Species = *l.SpeciesRef.Value
	}
	if l.PlanetRef.Value != nil {
		l.Planet = *l.PlanetRef.Value
	}
	l.SavedExperience = savedFieldString(value, "experience")
	if l.SavedExperience.Recorded && !l.SavedExperience.Unsupported {
		l.Experience, _ = strconv.ParseFloat(l.SavedExperience.Value, 64)
	}
	if uniqueSavedFields(value, "location") != nil {
		l.Location = LeaderLocation{Unsupported: true, Recorded: true}
	}
	if uniqueSavedFields(value, "council_location") != nil {
		l.CouncilLocation = LeaderLocation{Unsupported: true, Recorded: true}
	}
	l.LocationAmbiguous = l.Location.Unsupported
	for _, pair := range value.Object.Pairs {
		switch pair.Key {
		case "tier", "date", "date_added", "death_date", "days_frozen", "leader_terms", "background_planet", "available_trait", "available_trait_selections", "cooldown":
			l.SavedDetails = append(l.SavedDetails, pair)
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
	Recorded    bool   `clausewitz:"-"`
	Unsupported bool   `clausewitz:"-"`
}

func (l *LeaderLocation) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields LeaderLocation
	var decoded fields
	if uniqueSavedFields(value, "type", "assignment", "id", "position") != nil || clausewitz.DecodeValue(value, &decoded) != nil {
		*l = LeaderLocation{Recorded: true, Unsupported: true}
		return nil
	}
	*l = LeaderLocation(decoded)
	l.Recorded = true
	return nil
}
