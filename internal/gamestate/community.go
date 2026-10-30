package gamestate

type GalacticCommunity struct {
	Members  []int  `clausewitz:"members"`
	Proposed []int  `clausewitz:"proposed"`
	Passed   []int  `clausewitz:"passed"`
	Formed   string `clausewitz:"community_formed"`
}

type Resolution struct {
	Type       string `clausewitz:"type"`
	Country    *int   `clausewitz:"country"`
	Supporters []int  `clausewitz:"supporters"`
	Opponents  []int  `clausewitz:"opponents"`
	PassDate   string `clausewitz:"pass_date"`
}
