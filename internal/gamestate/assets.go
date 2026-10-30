package gamestate

type AgreementStore struct {
	Items map[int]Agreement `clausewitz:"agreements"`
}
type Agreement struct {
	Owner     *int   `clausewitz:"owner"`
	Target    *int   `clausewitz:"target"`
	Status    string `clausewitz:"active_status"`
	DateAdded string `clausewitz:"date_added"`
	Terms     struct {
		Preset string `clausewitz:"agreement_preset"`
	} `clausewitz:"term_data"`
}
type SituationStore struct {
	Items map[int]Situation `clausewitz:"situations"`
}
type Situation struct {
	Country           *int    `clausewitz:"country"`
	Type              string  `clausewitz:"type"`
	Progress          float64 `clausewitz:"progress"`
	LastMonthProgress float64 `clausewitz:"last_month_progress"`
	Approach          string  `clausewitz:"approach"`
}
type Megastructure struct {
	Owner      *int   `clausewitz:"owner"`
	Type       string `clausewitz:"type"`
	Coordinate struct {
		Origin *int `clausewitz:"origin"`
	} `clausewitz:"coordinate"`
}
type GalacticSystem struct {
	Name    LocalizedName `clausewitz:"name"`
	Planets []int         `clausewitz:"planet"`
}
