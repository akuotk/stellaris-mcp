package gamestate

// Parallel arrays preserve each save's IDs and raw population amounts without inferred scaling.
type PopEthos struct {
	Ethics []string `clausewitz:"ethics"`
	Pops   []int    `clausewitz:"pops"`
}

type PopFactionDistribution struct {
	Objects []int `clausewitz:"objects"`
	Pops    []int `clausewitz:"pops"`
}

type Species struct {
	Name     LocalizedName `clausewitz:"name"`
	Class    string        `clausewitz:"class"`
	Portrait string        `clausewitz:"portrait"`
	Traits   struct {
		Trait []string `clausewitz:"trait"`
	} `clausewitz:"traits"`
}

type PopFaction struct {
	Country        *int          `clausewitz:"country"`
	Type           string        `clausewitz:"type"`
	Name           LocalizedName `clausewitz:"name"`
	SupportPercent float64       `clausewitz:"support_percent"`
	Approval       float64       `clausewitz:"faction_approval"`
	LastMonthPops  int           `clausewitz:"last_month_pops"`
	PopsChange     int           `clausewitz:"pops_change"`
}
