package gamestate

import (
	"archive/zip"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type SystemContents struct {
	Systems          ConnectionRecordTable[SavedSystemContents]
	Planets          ConnectionRecordTable[SavedSystemPlanet]
	Deposits         ConnectionRecordTable[Deposit]
	NebulaCandidates []SavedNebulaCandidate
	NebulaUnresolved int
}

type SavedNebulaCandidate struct {
	Position int
	Matches  int
	Value    clausewitz.Value
}

type SavedSystemContents struct {
	Name   LocalizedName
	Value  clausewitz.Value
	Bodies []SystemBodyReference
}

type SystemBodyReference struct {
	Value clausewitz.Value
	ID    OptionalIntRef
}

type SavedSystemPlanet struct {
	Name             LocalizedName
	Class            OptionalString
	Orbit            OptionalString
	IsMoon           OptionalString
	Surveyed         OptionalString
	SurveyedBy       OptionalIntRef
	Owner            OptionalIntRef
	CoordinateOrigin OptionalIntRef
	MoonOf           OptionalIntRef
	Moons            SurfaceRefList
	Deposits         SurfaceRefList
	Value            clausewitz.Value
}

func LoadSystemContentsFromArchive(archive *zip.Reader, id int) (*SystemContents, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return nil, err
	}
	root, err := clausewitz.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse gamestate: %w", err)
	}
	return decodeSystemContents(clausewitz.Value{Object: root}, id), nil
}

func decodeSystemContents(root clausewitz.Value, id int) *SystemContents {
	result := &SystemContents{Systems: systemSubsetTable[SavedSystemContents](root, map[int]bool{id: true}, "galactic_object")}
	if root.IsObject() {
		position := 0
		for _, pair := range root.Object.Pairs {
			if pair.Key != "nebula" {
				continue
			}
			if !pair.Value.IsObject() {
				result.NebulaUnresolved++
				position++
				continue
			}
			matches := 0
			for _, field := range pair.Value.Object.Pairs {
				if field.Key != "galactic_object" {
					continue
				}
				var ref OptionalIntRef
				ref.UnmarshalClausewitz(field.Value)
				if ref.Unsupported || ref.Value == nil {
					result.NebulaUnresolved++
				} else if *ref.Value == id {
					matches++
				}
			}
			if matches > 0 {
				result.NebulaCandidates = append(result.NebulaCandidates, SavedNebulaCandidate{Position: position, Matches: matches, Value: pair.Value})
			}
			position++
		}
	}
	system, found := result.Systems.Items[id]
	if !found {
		return result
	}
	wanted := map[int]bool{}
	for _, body := range system.Bodies {
		if body.ID.Value != nil {
			wanted[*body.ID.Value] = true
		}
	}
	result.Planets = systemSubsetTable[SavedSystemPlanet](root, wanted, "planets", "planet")
	depositIDs := map[int]bool{}
	for _, planet := range result.Planets.Items {
		for _, id := range planet.Deposits.Values {
			depositIDs[id] = true
		}
	}
	result.Deposits = systemSubsetTable[Deposit](root, depositIDs, "deposit")
	return result
}

func systemSubsetTable[T any](root clausewitz.Value, wanted map[int]bool, path ...string) ConnectionRecordTable[T] {
	var table ConnectionRecordTable[T]
	value, recorded, unsupported := savedFieldPath(root, path...)
	if !recorded {
		return table
	}
	if unsupported || !value.IsObject() {
		table.Recorded, table.TableIssue = true, true
		return table
	}
	selected := &clausewitz.Object{}
	invalidKeys := false
	for _, pair := range value.Object.Pairs {
		id, err := strconv.Atoi(pair.Key)
		if err != nil {
			invalidKeys = true
		} else if wanted == nil || wanted[id] {
			selected.Pairs = append(selected.Pairs, pair)
		}
	}
	table.UnmarshalClausewitz(clausewitz.Value{Object: selected})
	table.TableIssue = table.TableIssue || invalidKeys
	return table
}

func (s *SavedSystemContents) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsObject() {
		return fmt.Errorf("unsupported saved system shape")
	}
	*s = SavedSystemContents{Value: value, Name: savedLocalizedName(value, "name")}
	var references []clausewitz.Value
	for _, pair := range value.Object.Pairs {
		if pair.Key == "planet" {
			references = append(references, pair.Value)
		}
	}
	if len(references) == 1 {
		switch value := references[0]; {
		case value.IsList():
			references = value.List
		case value.IsObject() && len(value.Object.Pairs) == 0:
			s.Bodies = []SystemBodyReference{}
			return nil
		}
	}
	for _, value := range references {
		ref := SystemBodyReference{Value: value}
		ref.ID.UnmarshalClausewitz(value)
		s.Bodies = append(s.Bodies, ref)
	}
	return nil
}

func (p *SavedSystemPlanet) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsObject() {
		return fmt.Errorf("unsupported saved planet shape")
	}
	*p = SavedSystemPlanet{
		Value: value, Name: savedLocalizedName(value, "name"), Class: savedFieldString(value, "planet_class"),
		Orbit: savedFieldString(value, "orbit"), IsMoon: savedFieldString(value, "is_moon"), Surveyed: savedFieldString(value, "surveyed"),
		SurveyedBy: savedPathInt(value, "surveyed_by"), Owner: savedPathInt(value, "owner"),
		CoordinateOrigin: savedPathInt(value, "coordinate", "origin"), MoonOf: savedPathInt(value, "moon_of"),
		Moons: systemPlanetList(value, "moons"), Deposits: systemPlanetList(value, "deposits"),
	}
	return nil
}

func systemPlanetList(value clausewitz.Value, key string) SurfaceRefList {
	field, recorded, duplicate := savedFieldPath(value, key)
	refs := SurfaceRefList{OptionalIntList: OptionalIntList{Recorded: recorded}, Duplicate: duplicate}
	if recorded && !duplicate {
		refs.UnmarshalClausewitz(field)
	}
	return refs
}
