package gamestate

import (
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type HyperlaneList struct {
	Recorded    bool
	Items       []SavedHyperlane
	Unsupported bool
}

type SavedHyperlane struct {
	To          *int    `clausewitz:"to"`
	Length      *string `clausewitz:"length"`
	Bridge      *string `clausewitz:"bridge"`
	Unsupported bool
}

func (v *HyperlaneList) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = HyperlaneList{Recorded: true}
	if !value.IsList() && (!value.IsObject() || len(value.Object.Pairs) != 0) {
		v.Unsupported = true
		return nil
	}
	if err := clausewitz.DecodeValue(value, &v.Items); err != nil {
		v.Items = nil
		v.Unsupported = true
	}
	return nil
}

func (v *SavedHyperlane) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = SavedHyperlane{}
	type fields SavedHyperlane
	var decoded fields
	if uniqueSavedFields(value, "to", "length", "bridge") != nil || clausewitz.DecodeValue(value, &decoded) != nil {
		v.Unsupported = true
		return nil
	}
	*v = SavedHyperlane(decoded)
	return nil
}

type SavedBypass struct {
	Type              *string           `clausewitz:"type"`
	Active            *string           `clausewitz:"active"`
	ManualLock        *string           `clausewitz:"is_manual_lock"`
	LockCountry       *int              `clausewitz:"lock_country"`
	LockRemainingDays *string           `clausewitz:"lock_remaining_days"`
	LinkedTo          *int              `clausewitz:"linked_to"`
	Owner             *SavedBypassOwner `clausewitz:"owner"`
	Connections       OptionalIntList   `clausewitz:"connections"`
	ActiveConnections OptionalIntList   `clausewitz:"active_connections"`
}

type SavedBypassOwner struct {
	Type *string `clausewitz:"type"`
	ID   *int    `clausewitz:"id"`
}

func (v *SavedBypass) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "type", "active", "is_manual_lock", "lock_country", "lock_remaining_days", "linked_to", "owner", "connections", "active_connections"); err != nil {
		return err
	}
	type fields SavedBypass
	return clausewitz.DecodeValue(value, (*fields)(v))
}

func (v *SavedBypassOwner) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "type", "id"); err != nil {
		return err
	}
	type fields SavedBypassOwner
	return clausewitz.DecodeValue(value, (*fields)(v))
}

type SavedNaturalWormhole struct {
	Bypass     *int               `clausewitz:"bypass"`
	Coordinate OptionalCoordinate `clausewitz:"coordinate"`
}

func (v *SavedNaturalWormhole) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "bypass", "coordinate"); err != nil {
		return err
	}
	type fields SavedNaturalWormhole
	return clausewitz.DecodeValue(value, (*fields)(v))
}

type ConnectionRecordTable[T any] struct {
	Recorded     bool
	Items        map[int]T
	DecodeErrors map[int]string
	TableIssue   bool
}

func (v *ConnectionRecordTable[T]) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = ConnectionRecordTable[T]{Recorded: true, Items: map[int]T{}, DecodeErrors: map[int]string{}}
	if !value.IsObject() {
		v.TableIssue = true
		return nil
	}
	seen := map[int]bool{}
	for _, pair := range value.Object.Pairs {
		id, err := strconv.Atoi(pair.Key)
		if err != nil {
			v.TableIssue = true
			continue
		}
		if seen[id] {
			delete(v.Items, id)
			v.DecodeErrors[id] = "duplicate connection record ID"
			continue
		}
		seen[id] = true
		if pair.Value.IsScalar() && pair.Value.Scalar == "none" {
			continue
		}
		var record T
		if err := clausewitz.DecodeValue(pair.Value, &record); err != nil {
			v.DecodeErrors[id] = err.Error()
			continue
		}
		v.Items[id] = record
	}
	return nil
}
