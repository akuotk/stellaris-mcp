package gamestate

import (
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

// Armies preserves usable records when an optional record has an unsupported shape.
type Armies struct {
	Items        map[int]Army
	DecodeErrors map[int]string
	TableIssue   bool
}

type Army struct {
	Name              LocalizedName `clausewitz:"name"`
	Type              string        `clausewitz:"type"`
	Owner             *int          `clausewitz:"owner"`
	Species           *int          `clausewitz:"species"`
	Colony            *int          `clausewitz:"colony"`
	SpawningColony    *int          `clausewitz:"spawning_colony"`
	Planet            *int          `clausewitz:"planet"`
	Ship              *int          `clausewitz:"ship"`
	Leader            *int          `clausewitz:"leader"`
	Health            *float64      `clausewitz:"health"`
	MaxHealth         *float64      `clausewitz:"max_health"`
	Morale            *float64      `clausewitz:"morale"`
	Experience        *float64      `clausewitz:"experience"`
	JumpDriveCooldown string        `clausewitz:"jump_drive_cooldown"`
}

func (a *Armies) UnmarshalClausewitz(value clausewitz.Value) error {
	*a = Armies{Items: map[int]Army{}, DecodeErrors: map[int]string{}}
	if !value.IsObject() {
		a.TableIssue = true
		return nil
	}
	for _, pair := range value.Object.Pairs {
		id, err := strconv.Atoi(pair.Key)
		if err != nil {
			a.TableIssue = true
			continue
		}
		if pair.Value.IsScalar() && pair.Value.Scalar == "none" {
			continue
		}
		var army Army
		err = clausewitz.DecodeValue(pair.Value, &army)
		if _, duplicate := a.Items[id]; duplicate {
			err = fmt.Errorf("duplicate army ID")
		}
		a.Items[id] = army
		if err != nil {
			a.DecodeErrors[id] = err.Error()
		}
	}
	return nil
}
