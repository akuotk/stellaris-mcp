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
	field, recorded, unsupported := savedFieldPath(value, path...)
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
		Name: savedLocalizedName(value, "adjective"), Type: savedFieldString(value, "type"), Age: savedFieldString(value, "preftl_age"),
		Awareness: savedFieldString(value, "awareness"), OldAwareness: savedFieldString(value, "old_awareness"), Capital: savedPathInt(value, "capital"),
	}
	for _, pair := range value.Object.Pairs {
		if pair.Key == "type" && pair.Value.IsScalar() && pair.Value.Scalar == "primitive" {
			c.PrimitiveMarker = true
			break
		}
	}
	if field, recorded, unsupported := savedFieldPath(value, "fleets_manager"); recorded {
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
		Name: savedLocalizedName(value, "name"), Owner: savedPathInt(value, "owner"), Controller: savedPathInt(value, "controller"),
		OrbitalStation: savedPathInt(value, "shipclass_orbital_station"),
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
	f.Name = savedLocalizedName(value, "name")
	f.Class = savedFieldString(value, "ship_class")
	f.Origin = savedPathInt(value, "movement_manager", "coordinate", "origin")
	orbitable, recorded, unsupported := savedFieldPath(value, "movement_manager", "orbit", "orbitable")
	f.Orbitable, f.OrbitableSaved = orbitable, recorded && !unsupported
	switch {
	case unsupported || recorded && (!orbitable.IsObject() || len(orbitable.Object.Pairs) != 1):
		f.OrbitPlanet = OptionalIntRef{Recorded: true, Unsupported: true}
	case recorded:
		f.OrbitPlanet = savedPathInt(orbitable, "planet")
		if !f.OrbitPlanet.Recorded {
			f.OrbitPlanet = OptionalIntRef{Recorded: true, Unsupported: true}
		}
	}
	ships, recorded, unsupported := savedFieldPath(value, "ships")
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
	s.Fleet = savedPathInt(value, "fleet")
	return nil
}
