package gamestate

import (
	"archive/zip"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

const (
	SensorRangeFleetsKind      = "sensor_range_fleets"
	SurveyedDepositHoldersKind = "surveyed_deposit_holders"
)

type CountryObservationEntry struct {
	Path, Kind                     string
	Value                          clausewitz.Value
	Identifier, HolderType         ControlledPlanetField
	IDOccurrences, PairOccurrences int
}

type CountryObservationTable struct {
	Label string
	ControlledPlanetTableStatus
	DuplicatePaths []string
}

type CountryObservationCandidate struct {
	Table, Status, Path string
	Occurrences         int
	DuplicatePaths      []string
}

type CountryObservationComparison struct {
	Candidates       []CountryObservationCandidate
	PlanetSurveyedBy ControlledPlanetField
}

type SavedCountryObservationRefs struct {
	CountryID, FieldOccurrences, UnsupportedEntries int
	Entries                                         []CountryObservationEntry
	Tables                                          []CountryObservationTable
	Comparisons                                     map[int]CountryObservationComparison
}

func LoadCountryObservationRefsFromArchive(archive *zip.Reader, requestedCountry *int, kind string) (SavedCountryObservationRefs, error) {
	if kind != SensorRangeFleetsKind && kind != SurveyedDepositHoldersKind {
		return SavedCountryObservationRefs{}, fmt.Errorf("unsupported country observation kind")
	}
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return SavedCountryObservationRefs{}, fmt.Errorf("cannot read selected gamestate")
	}
	doc, err := clausewitz.Parse(data)
	if err != nil {
		return SavedCountryObservationRefs{}, fmt.Errorf("cannot parse selected gamestate")
	}
	id, country, path, err := selectSavedCountry(doc, requestedCountry)
	if err != nil {
		return SavedCountryObservationRefs{}, err
	}
	result := SavedCountryObservationRefs{CountryID: id, Comparisons: map[int]CountryObservationComparison{}}
	for i, pair := range country.Object.Pairs {
		if pair.Key == kind {
			result.readObservationList(path+"/"+strconv.Itoa(i), pair.Value, kind)
		}
	}
	counts, pairs := map[int]int{}, map[[2]int]int{}
	for _, entry := range result.Entries {
		if entry.Identifier.ID != nil {
			counts[*entry.Identifier.ID]++
			if entry.HolderType.ID != nil {
				pairs[[2]int{*entry.Identifier.ID, *entry.HolderType.ID}]++
			}
		}
	}
	root := clausewitz.Value{Object: doc}
	for _, target := range []struct {
		label  string
		fields []string
	}{
		{"fleet", []string{"fleet"}},
		{"planet", []string{"planets", "planet"}},
		{"deposit", []string{"deposit"}},
		{"country", []string{"country"}},
	} {
		table := indexControlledPlanetTable(root, target.fields...)
		status := CountryObservationTable{Label: target.label, ControlledPlanetTableStatus: table.ControlledPlanetTableStatus}
		if table.Status == "duplicate" {
			status.DuplicatePaths = observationDuplicateTablePaths(root, target.fields)
		}
		result.Tables = append(result.Tables, status)
		for ref := range counts {
			record, state, occurrences := table.lookup(ref)
			candidate := CountryObservationCandidate{Table: target.label, Status: state, Occurrences: occurrences}
			if state == "unique" {
				candidate.Status = "numeric-candidate"
			}
			if record != nil {
				candidate.Path = record.path
			}
			if state == "duplicate" {
				candidate.DuplicatePaths = table.candidatePaths(ref)
			}
			comparison := result.Comparisons[ref]
			comparison.Candidates = append(comparison.Candidates, candidate)
			if kind == SurveyedDepositHoldersKind && target.label == "planet" && state == "unique" {
				comparison.PlanetSurveyedBy = controlledPlanetField(record.value, record.path, "surveyed_by")
			}
			result.Comparisons[ref] = comparison
		}
	}
	for i := range result.Entries {
		entry := &result.Entries[i]
		if entry.Identifier.ID != nil {
			entry.IDOccurrences = counts[*entry.Identifier.ID]
			if entry.HolderType.ID != nil {
				entry.PairOccurrences = pairs[[2]int{*entry.Identifier.ID, *entry.HolderType.ID}]
			}
		}
	}
	return result, nil
}

func (result *SavedCountryObservationRefs) readObservationList(path string, value clausewitz.Value, kind string) {
	result.FieldOccurrences++
	switch {
	case value.IsList():
		for i, item := range value.List {
			itemPath := path + "/" + strconv.Itoa(i)
			entry := CountryObservationEntry{Path: itemPath, Value: item, Kind: "unsupported-item"}
			if kind == SensorRangeFleetsKind {
				if id := controlledPlanetID(item); id != nil {
					entry.Kind = "sensor-reference"
					entry.Identifier = ControlledPlanetField{Status: "integer", ID: id, Path: itemPath, Value: item}
				}
			} else if item.IsObject() {
				entry.Kind = "holder-reference"
				entry.Identifier = controlledPlanetField(item, itemPath, "id")
				entry.HolderType = controlledPlanetField(item, itemPath, "type")
			}
			if entry.Identifier.ID == nil || (kind == SurveyedDepositHoldersKind && entry.HolderType.ID == nil) {
				result.UnsupportedEntries++
			}
			result.Entries = append(result.Entries, entry)
		}
	case value.IsObject() && len(value.Object.Pairs) == 0:
		result.Entries = append(result.Entries, CountryObservationEntry{Path: path, Kind: "empty-list", Value: value})
	default:
		result.UnsupportedEntries++
		result.Entries = append(result.Entries, CountryObservationEntry{Path: path, Kind: "unsupported-list", Value: value})
	}
}

func observationDuplicateTablePaths(value clausewitz.Value, fields []string) []string {
	path := ""
	for _, key := range fields {
		field := controlledPlanetRawField(value, path, key)
		if field.Status == "duplicate" {
			var paths []string
			for i, pair := range value.Object.Pairs {
				if pair.Key == key {
					paths = append(paths, path+"/"+strconv.Itoa(i))
					if len(paths) == 20 {
						break
					}
				}
			}
			return paths
		}
		if field.Status != "unique" {
			return nil
		}
		value, path = field.Value, field.Path
	}
	return nil
}
