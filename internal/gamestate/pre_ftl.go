package gamestate

import (
	"archive/zip"
	"fmt"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

const observationStationClass = "shipclass_observation_station"

type PreFTLState struct {
	Countries ConnectionRecordTable[PreFTLCountry]
	Planets   ConnectionRecordTable[PreFTLPlanet]
	Fleets    ConnectionRecordTable[ObservationFleet]
	Ships     ConnectionRecordTable[ObservationShip]
}

type PreFTLCountry struct {
	Name            LocalizedName
	Type            OptionalString
	Age             OptionalString
	Awareness       OptionalString
	OldAwareness    OptionalString
	Capital         OptionalIntRef
	Fleets          FleetsManager
	PrimitiveMarker bool
}

func (c PreFTLCountry) Relevant() bool { return c.PrimitiveMarker || c.Age.Recorded }

type PreFTLPlanet struct {
	Name           LocalizedName
	Owner          OptionalIntRef
	Controller     OptionalIntRef
	OrbitalStation OptionalIntRef
}

type ObservationFleet struct {
	Recorded       bool
	Name           LocalizedName
	Class          OptionalString
	OrbitPlanet    OptionalIntRef
	Orbitable      clausewitz.Value
	OrbitableSaved bool
	Origin         OptionalIntRef
	Ships          OptionalIntList
}

type ObservationShip struct{ Fleet OptionalIntRef }

func LoadPreFTLFromArchive(archive *zip.Reader) (*PreFTLState, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return nil, err
	}
	return loadPreFTL(data)
}

func loadPreFTL(data []byte) (*PreFTLState, error) {
	root, err := clausewitz.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse gamestate: %w", err)
	}
	value := clausewitz.Value{Object: root}
	return &PreFTLState{
		Countries: preFTLTable[PreFTLCountry](value, "country"),
		Planets:   preFTLTable[PreFTLPlanet](value, "planets", "planet"),
		Fleets:    preFTLTable[ObservationFleet](value, "fleet"),
		Ships:     preFTLTable[ObservationShip](value, "ships"),
	}, nil
}

func preFTLTable[T any](value clausewitz.Value, path ...string) ConnectionRecordTable[T] {
	field, recorded, unsupported := preFTLField(value, path...)
	var table ConnectionRecordTable[T]
	if unsupported {
		table.Recorded, table.TableIssue = true, true
	} else if recorded {
		table.UnmarshalClausewitz(field)
	}
	return table
}

func (c *PreFTLCountry) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsObject() {
		return fmt.Errorf("unsupported saved country shape")
	}
	*c = PreFTLCountry{
		Name: preFTLName(value, "adjective"), Type: preFTLString(value, "type"), Age: preFTLString(value, "preftl_age"),
		Awareness: preFTLString(value, "awareness"), OldAwareness: preFTLString(value, "old_awareness"), Capital: preFTLInt(value, "capital"),
	}
	for _, pair := range value.Object.Pairs {
		if pair.Key == "type" && pair.Value.IsScalar() && pair.Value.Scalar == "primitive" {
			c.PrimitiveMarker = true
			break
		}
	}
	if field, recorded, unsupported := preFTLField(value, "fleets_manager"); recorded {
		if unsupported || clausewitz.DecodeValue(field, &c.Fleets) != nil {
			c.Fleets.OwnershipAmbiguous = true
		}
	}
	return nil
}

func (p *PreFTLPlanet) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsObject() {
		return fmt.Errorf("unsupported saved planet shape")
	}
	*p = PreFTLPlanet{
		Name: preFTLName(value, "name"), Owner: preFTLInt(value, "owner"), Controller: preFTLInt(value, "controller"),
		OrbitalStation: preFTLInt(value, "shipclass_orbital_station"),
	}
	return nil
}

func (f *ObservationFleet) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsObject() {
		return fmt.Errorf("unsupported saved fleet shape")
	}
	*f = ObservationFleet{}
	for _, pair := range value.Object.Pairs {
		if pair.Key == "ship_class" && pair.Value.IsScalar() && pair.Value.Scalar == observationStationClass {
			f.Recorded = true
			break
		}
	}
	if !f.Recorded {
		return nil
	}
	f.Name = preFTLName(value, "name")
	f.Class = preFTLString(value, "ship_class")
	f.Origin = preFTLInt(value, "movement_manager", "coordinate", "origin")
	orbitable, recorded, unsupported := preFTLField(value, "movement_manager", "orbit", "orbitable")
	f.Orbitable, f.OrbitableSaved = orbitable, recorded && !unsupported
	switch {
	case unsupported || recorded && (!orbitable.IsObject() || len(orbitable.Object.Pairs) != 1):
		f.OrbitPlanet = OptionalIntRef{Recorded: true, Unsupported: true}
	case recorded:
		f.OrbitPlanet = preFTLInt(orbitable, "planet")
		if !f.OrbitPlanet.Recorded {
			f.OrbitPlanet = OptionalIntRef{Recorded: true, Unsupported: true}
		}
	}
	ships, recorded, unsupported := preFTLField(value, "ships")
	f.Ships = OptionalIntList{Recorded: recorded, Unsupported: unsupported}
	if recorded && !unsupported {
		f.Ships.UnmarshalClausewitz(ships)
	}
	return nil
}

func (s *ObservationShip) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsObject() {
		return fmt.Errorf("unsupported saved ship shape")
	}
	s.Fleet = preFTLInt(value, "fleet")
	return nil
}

func preFTLInt(value clausewitz.Value, path ...string) OptionalIntRef {
	field, recorded, unsupported := preFTLField(value, path...)
	ref := OptionalIntRef{Recorded: recorded, Unsupported: unsupported}
	if recorded && !unsupported {
		ref.UnmarshalClausewitz(field)
	}
	return ref
}

func preFTLString(value clausewitz.Value, key string) OptionalString {
	field, recorded, unsupported := preFTLField(value, key)
	ref := OptionalString{Recorded: recorded, Unsupported: unsupported}
	if recorded && !unsupported {
		ref.UnmarshalClausewitz(field)
	}
	return ref
}

func preFTLName(value clausewitz.Value, key string) LocalizedName {
	var name LocalizedName
	if field, recorded, unsupported := preFTLField(value, key); recorded && !unsupported && uniqueSavedFields(field, "key", "variables") == nil {
		if clausewitz.DecodeValue(field, &name) != nil {
			return LocalizedName{}
		}
	}
	return name
}

func preFTLField(value clausewitz.Value, path ...string) (clausewitz.Value, bool, bool) {
	for _, key := range path {
		if !value.IsObject() {
			return value, true, true
		}
		var next clausewitz.Value
		found := false
		for _, pair := range value.Object.Pairs {
			if pair.Key != key {
				continue
			}
			if found {
				return clausewitz.Value{}, true, true
			}
			next, found = pair.Value, true
		}
		if !found {
			return clausewitz.Value{}, false, false
		}
		value = next
	}
	return value, true, false
}
