package gamestate

import (
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type CouncilGovernment struct {
	Recorded      bool
	Agenda        OptionalString
	Progress      OptionalString
	UnlockedSlots OptionalString
	Heir          OptionalIntRef
	Cooldowns     []CouncilAgendaCooldown
	Unsupported   bool
}

type CouncilAgendaCooldown struct {
	Agenda      OptionalString `clausewitz:"council_agenda"`
	StartDate   OptionalString `clausewitz:"start_date"`
	Unsupported bool
}

func (v *CouncilGovernment) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = CouncilGovernment{Recorded: true}
	if !value.IsObject() {
		v.Unsupported = true
		return nil
	}
	v.Heir = savedPathInt(value, "heir")
	for _, pair := range value.Object.Pairs {
		var field *OptionalString
		switch pair.Key {
		case "council_agenda":
			field = &v.Agenda
		case "council_agenda_progress":
			field = &v.Progress
		case "unlocked_civic_council_slots":
			field = &v.UnlockedSlots
		case "council_agenda_cooldowns":
			var cooldown CouncilAgendaCooldown
			if uniqueSavedFields(pair.Value, "council_agenda", "start_date") != nil || clausewitz.DecodeValue(pair.Value, &cooldown) != nil {
				cooldown = CouncilAgendaCooldown{Unsupported: true}
			}
			v.Cooldowns = append(v.Cooldowns, cooldown)
			continue
		default:
			continue
		}
		if field.Recorded {
			*field = OptionalString{Recorded: true, Unsupported: true}
		} else {
			field.UnmarshalClausewitz(pair.Value)
		}
	}
	return nil
}

type CouncilPositionTable struct {
	Recorded     bool
	Items        map[int]CouncilPosition
	DecodeErrors map[int]string
	TableIssue   bool
}

type CouncilPosition struct {
	Country *int    `clausewitz:"country"`
	Leader  *int    `clausewitz:"leader"`
	Type    *string `clausewitz:"type"`
}

func (v *CouncilPositionTable) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = CouncilPositionTable{Recorded: true, Items: map[int]CouncilPosition{}, DecodeErrors: map[int]string{}}
	if !value.IsObject() || uniqueSavedFields(value, "council_positions") != nil {
		v.TableIssue = true
		return nil
	}
	var table *clausewitz.Object
	for _, pair := range value.Object.Pairs {
		if pair.Key != "council_positions" {
			continue
		}
		if pair.Value.IsObject() {
			table = pair.Value.Object
		}
		break
	}
	if table == nil {
		v.TableIssue = len(value.Object.Pairs) > 0
		return nil
	}
	seen := map[int]bool{}
	for _, pair := range table.Pairs {
		id, err := strconv.Atoi(pair.Key)
		if err != nil {
			v.TableIssue = true
			continue
		}
		if seen[id] {
			delete(v.Items, id)
			v.DecodeErrors[id] = "duplicate council position ID"
			continue
		}
		seen[id] = true
		if pair.Value.IsScalar() && pair.Value.Scalar == "none" {
			continue
		}
		var position CouncilPosition
		if err := uniqueSavedFields(pair.Value, "country", "leader", "type"); err != nil {
			v.DecodeErrors[id] = err.Error()
			continue
		}
		if err := clausewitz.DecodeValue(pair.Value, &position); err != nil {
			v.DecodeErrors[id] = err.Error()
			continue
		}
		v.Items[id] = position
	}
	return nil
}
