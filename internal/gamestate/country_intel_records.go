package gamestate

import (
	"archive/zip"
	"context"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

const (
	CountryIntelManagerKind = "intel_manager"
	CountryIntelLevelKind   = "intel_level"
	CountryHighestIntelKind = "highest_intel_level"
)

type CountryIntelRow struct {
	Path, Role, ContainerPath, ContainerKind string
	Index                                    int
	Node                                     *clausewitz.RawNode
	HeadID                                   *int
}

type SavedCountryIntelRecords struct {
	CountryID, EntryIndex, FieldOccurrences, IntelOccurrences int
	CountryPath                                               string
	Rows                                                      []CountryIntelRow
	HeadOccurrences                                           map[int]int
	Tables                                                    []CountryObservationTable
	indexes                                                   []controlledPlanetTable
}

func LoadCountryIntelRecordsFromArchive(ctx context.Context, archive *zip.Reader, requestedCountry *int, kind string) (SavedCountryIntelRecords, error) {
	saved, err := loadRawSavedCountry(ctx, archive, requestedCountry, false)
	if err != nil {
		return SavedCountryIntelRecords{}, err
	}
	out := SavedCountryIntelRecords{
		CountryID: saved.CountryID, EntryIndex: saved.EntryIndex, CountryPath: saved.Path,
		HeadOccurrences: make(map[int]int),
	}
	for i, field := range saved.Node.Children {
		if field.Key == nil || field.Key.Text != kind {
			continue
		}
		out.FieldOccurrences++
		path := saved.Path + "/" + strconv.Itoa(i)
		if kind == CountryIntelManagerKind && (field.Kind == "object" || field.Kind == "mixed") {
			for j, child := range field.Children {
				childPath := path + "/" + strconv.Itoa(j)
				if child.Key != nil && child.Key.Text == "intel" {
					out.IntelOccurrences++
					out.readList(child, childPath, path, field.Kind, j, true)
				} else {
					out.Rows = append(out.Rows, CountryIntelRow{
						Path: childPath, Role: "manager-field", ContainerPath: path, ContainerKind: field.Kind, Index: j, Node: child,
					})
				}
			}
		} else if kind == CountryIntelManagerKind {
			role := "unsupported-manager"
			if field.Kind == "empty_block" {
				role = "empty-manager"
			}
			out.Rows = append(out.Rows, CountryIntelRow{
				Path: path, Role: role, ContainerPath: saved.Path, ContainerKind: saved.Node.Kind, Index: i, Node: field,
			})
		} else {
			out.readList(field, path, saved.Path, saved.Node.Kind, i, false)
		}
	}
	if kind == CountryIntelManagerKind {
		out.indexCandidateTables(saved.root)
	}
	return out, nil
}

func (out *SavedCountryIntelRecords) readList(node *clausewitz.RawNode, path, parentPath, parentKind string, index int, pairs bool) {
	if len(node.Children) == 0 {
		role := "unsupported-list"
		if node.Kind == "empty_block" {
			role = "empty-list"
		}
		out.Rows = append(out.Rows, CountryIntelRow{
			Path: path, Role: role, ContainerPath: parentPath, ContainerKind: parentKind, Index: index, Node: node,
		})
		return
	}
	for i, child := range node.Children {
		row := CountryIntelRow{
			Path: path + "/" + strconv.Itoa(i), ContainerPath: path, ContainerKind: node.Kind, Index: i, Node: child,
			Role: "unsupported-list-entry",
		}
		if pairs {
			row.Role = "unsupported-pair"
			if child.Key == nil && child.Kind == "list" && len(child.Children) == 2 &&
				child.Children[0].IsScalar() && child.Children[1].Kind == "object" {
				if id, err := strconv.Atoi(child.Children[0].Scalar.Text); err == nil && id >= 0 {
					row.HeadID, row.Role = &id, "pair"
					out.HeadOccurrences[id]++
				}
			}
		} else if node.Kind == "list" && child.IsScalar() {
			row.Role = "array-element"
		}
		out.Rows = append(out.Rows, row)
	}
}

func (out *SavedCountryIntelRecords) indexCandidateTables(root *clausewitz.RawNode) {
	doc := &clausewitz.Object{Pairs: make([]clausewitz.Pair, len(root.Children))}
	for i, child := range root.Children {
		if child.Key != nil && (child.Key.Text == "country" || child.Key.Text == "spy_networks") {
			doc.Pairs[i] = clausewitz.Pair{Key: child.Key.Text, Value: projectRawCountrySelection(child, 1)}
		}
	}
	value := clausewitz.Value{Object: doc}
	for _, key := range []string{"country", "spy_networks"} {
		table := indexControlledPlanetTable(value, key)
		status := CountryObservationTable{Label: key, ControlledPlanetTableStatus: table.ControlledPlanetTableStatus}
		if table.Status == "duplicate" {
			status.DuplicatePaths = observationDuplicateTablePaths(value, []string{key})
		}
		out.Tables = append(out.Tables, status)
		out.indexes = append(out.indexes, table)
	}
}

func (out *SavedCountryIntelRecords) Candidates(id int) []CountryObservationCandidate {
	candidates := make([]CountryObservationCandidate, 0, len(out.indexes))
	for i, table := range out.indexes {
		record, status, occurrences := table.lookup(id)
		candidate := CountryObservationCandidate{Table: out.Tables[i].Label, Status: status, Occurrences: occurrences}
		if record != nil {
			candidate.Path = record.path
		}
		switch status {
		case "unique":
			candidate.Status = "numeric-candidate"
		case "duplicate":
			candidate.DuplicatePaths = table.candidatePaths(id)
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func ValidateCountryIntelKind(kind string) error {
	switch kind {
	case CountryIntelManagerKind, CountryIntelLevelKind, CountryHighestIntelKind:
		return nil
	default:
		return fmt.Errorf("kind must be intel_manager, intel_level or highest_intel_level")
	}
}
