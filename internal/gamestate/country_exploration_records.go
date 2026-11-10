package gamestate

import (
	"archive/zip"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

const (
	VisitedObjectsKind      = "visited_objects"
	HyperlaneSystemsKind    = "hyperlane_systems"
	TerraIncognitaKind      = "terra_incognita"
	CountryExplorationLimit = 20
)

type CountryExplorationRow struct {
	Path, ContainerPath, ContainerKind, Role string
	Index                                    int
	Node                                     *clausewitz.RawNode
	NumericCandidates                        []CountryExplorationCandidate
}

type CountryExplorationCandidate struct {
	Table          string   `json:"table"`
	Status         string   `json:"status"`
	RecordStatus   string   `json:"record_status,omitempty"`
	Path           string   `json:"path,omitempty"`
	Occurrences    int      `json:"occurrences"`
	DuplicatePaths []string `json:"duplicate_paths_preview,omitempty"`
}

type CountryExplorationTable struct {
	Table          string   `json:"table"`
	SavedTable     string   `json:"saved_table"`
	Status         string   `json:"status"`
	Path           string   `json:"path"`
	Unindexed      int      `json:"unindexed_entries"`
	Occurrences    int      `json:"table_occurrences"`
	DuplicatePaths []string `json:"duplicate_paths_preview,omitempty"`
	byID           map[int][]explorationCandidateRecord
}

type explorationCandidateRecord struct {
	index int
	node  *clausewitz.RawNode
}

type SavedCountryExplorationRecords struct {
	CountryID, EntryIndex, FieldOccurrences, RowCount int
	CountryPath                                       string
	Rows                                              []CountryExplorationRow
	Tables                                            []CountryExplorationTable
}

func ValidateCountryExplorationKind(kind string) error {
	switch kind {
	case VisitedObjectsKind, HyperlaneSystemsKind, TerraIncognitaKind:
		return nil
	default:
		return fmt.Errorf("kind must be visited_objects, hyperlane_systems or terra_incognita")
	}
}

func LoadCountryExplorationRecordsFromArchive(ctx context.Context, archive *zip.Reader, requestedCountry *int, kind string, offset int) (SavedCountryExplorationRecords, error) {
	saved, err := loadRawSavedCountry(ctx, archive, requestedCountry, false)
	if err != nil {
		return SavedCountryExplorationRecords{}, err
	}
	out := SavedCountryExplorationRecords{
		CountryID: saved.CountryID, EntryIndex: saved.EntryIndex, CountryPath: saved.Path,
		Rows: make([]CountryExplorationRow, 0, CountryExplorationLimit),
	}
	terra := kind == TerraIncognitaKind
	for i, field := range saved.Node.Children {
		if field.Key == nil || field.Key.Text != kind {
			continue
		}
		out.FieldOccurrences++
		path := saved.Path + "/" + strconv.Itoa(i)
		if len(field.Children) == 0 || (terra && field.Kind == "list") {
			role := "unsupported-field"
			if field.Kind == "empty_block" {
				role = "empty-field"
			}
			if out.RowCount >= offset && len(out.Rows) < CountryExplorationLimit {
				out.Rows = append(out.Rows, CountryExplorationRow{
					Path: path, ContainerPath: saved.Path, ContainerKind: saved.Node.Kind, Index: i, Role: role, Node: field,
				})
			}
			out.RowCount++
			continue
		}
		if err := out.walkField(ctx, field, path, terra, offset); err != nil {
			return SavedCountryExplorationRecords{}, err
		}
	}
	if offset > out.RowCount {
		return SavedCountryExplorationRecords{}, fmt.Errorf("offset exceeds saved row count")
	}
	out.Tables = []CountryExplorationTable{
		explorationTable(saved.root, "planet", "planets", "planet"),
		explorationTable(saved.root, "system", "galactic_object"),
		explorationTable(saved.root, "bypass", "bypasses"),
	}
	for i := range out.Rows {
		row := &out.Rows[i]
		if row.Node.Key != nil || !row.Node.IsScalar() || (row.ContainerKind != "list" && row.ContainerKind != "mixed") {
			continue
		}
		if id, err := strconv.Atoi(row.Node.Scalar.Text); err == nil && id >= 0 {
			for _, table := range out.Tables {
				row.NumericCandidates = append(row.NumericCandidates, table.candidate(id))
			}
		}
	}
	return out, nil
}

func (out *SavedCountryExplorationRecords) walkField(ctx context.Context, field *clausewitz.RawNode, path string, terra bool, offset int) error {
	type frame struct {
		node *clausewitz.RawNode
		next int
	}
	stack := []frame{{node: field}}
	var buffer []byte
	for len(stack) > 0 {
		parent := &stack[len(stack)-1]
		if parent.next == len(parent.node.Children) {
			stack = stack[:len(stack)-1]
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		i := parent.next
		child := parent.node.Children[i]
		parent.next++
		if out.RowCount >= offset && len(out.Rows) < CountryExplorationLimit {
			buffer = append(buffer[:0], path...)
			for _, ancestor := range stack[:len(stack)-1] {
				buffer = append(buffer, '/')
				buffer = strconv.AppendInt(buffer, int64(ancestor.next-1), 10)
			}
			container := string(buffer)
			role := "unsupported-list-entry"
			switch {
			case parent.node.Kind == "mixed":
				role = "mixed-entry"
			case parent.node.Kind == "list" && terra:
				role = "list-element"
			case parent.node.Kind == "list" && child.IsScalar():
				if id, err := strconv.Atoi(child.Scalar.Text); err == nil && id >= 0 {
					role = "list-element"
				}
			case terra:
				role = "terra-field"
			}
			out.Rows = append(out.Rows, CountryExplorationRow{
				Path: container + "/" + strconv.Itoa(i), ContainerPath: container, ContainerKind: parent.node.Kind,
				Index: i, Role: role, Node: child,
			})
		}
		out.RowCount++
		if terra && len(child.Children) > 0 {
			stack = append(stack, frame{node: child})
		}
	}
	return nil
}

func explorationTable(root *clausewitz.RawNode, label string, fields ...string) CountryExplorationTable {
	table := CountryExplorationTable{Table: label, SavedTable: strings.Join(fields, "."), Status: "recorded"}
	node := root
	for _, field := range fields {
		if node.Kind == "mixed" {
			table.Status = "partial"
		} else if node.Kind != "object" && node.Kind != "empty_block" {
			table.Status = "unsupported"
			return table
		}
		var paths []string
		var selected *clausewitz.RawNode
		table.Occurrences = 0
		for i, child := range node.Children {
			if child.Key != nil && child.Key.Text == field {
				table.Occurrences++
				selected = child
				if len(paths) < CountryExplorationLimit {
					paths = append(paths, table.Path+"/"+strconv.Itoa(i))
				}
			}
		}
		switch table.Occurrences {
		case 0:
			if table.Status != "partial" {
				table.Status = "not-recorded"
			}
			return table
		case 1:
			table.Path, node = paths[0], selected
		default:
			table.Status, table.DuplicatePaths = "duplicate", paths
			return table
		}
	}
	switch node.Kind {
	case "mixed":
		table.Status = "partial"
	case "object", "empty_block":
	default:
		table.Status = "unsupported"
		return table
	}
	table.byID = make(map[int][]explorationCandidateRecord)
	for i, row := range node.Children {
		if row.Key != nil {
			if id, err := strconv.Atoi(row.Key.Text); err == nil && id >= 0 {
				table.byID[id] = append(table.byID[id], explorationCandidateRecord{i, row})
				continue
			}
		}
		table.Unindexed++
		table.Status = "partial"
	}
	return table
}

func (t CountryExplorationTable) candidate(id int) CountryExplorationCandidate {
	rows := t.byID[id]
	result := CountryExplorationCandidate{Table: t.Table, Occurrences: len(rows)}
	switch {
	case t.Status != "recorded" && t.Status != "partial":
		result.Status = "table-" + t.Status
	case len(rows) > 1:
		result.Status = "duplicate"
		for _, row := range rows[:min(CountryExplorationLimit, len(rows))] {
			result.DuplicatePaths = append(result.DuplicatePaths, t.Path+"/"+strconv.Itoa(row.index))
		}
	case len(rows) == 0:
		result.Status = "missing"
		if t.Status == "partial" {
			result.Status = "unresolved"
		}
	default:
		row := rows[0]
		result.Path = t.Path + "/" + strconv.Itoa(row.index)
		switch {
		case row.node.Kind == "object":
			result.RecordStatus = "object"
		case row.node.Kind == "empty_block":
			result.RecordStatus = "empty-block"
		case row.node.IsScalar() && row.node.Scalar.Text == "none" && !row.node.Scalar.Quoted:
			result.RecordStatus = "none"
		default:
			result.RecordStatus = "unsupported-record"
		}
		result.Status = result.RecordStatus
		if t.Status == "partial" {
			result.Status = "observed"
		} else if result.RecordStatus == "object" {
			result.Status = "numeric-candidate"
		}
	}
	return result
}
