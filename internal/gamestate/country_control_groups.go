package gamestate

import (
	"archive/zip"
	"context"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

const (
	CountryControlGroupsPageSize = 20
	controlGroupFieldPreviewSize = 4
)

type ControlGroupFieldEntry struct {
	Path string
	Node *clausewitz.RawNode
}

type ControlGroupField struct {
	Status      string
	Occurrences int
	Preview     []ControlGroupFieldEntry
	number      int
}

type CountryControlGroupRow struct {
	Path, ContainerPath, ContainerKind, Role, SlotPath string
	Index                                              int
	Node                                               *clausewitz.RawNode
	ID, Type                                           ControlGroupField
	NumericCandidates                                  []CountryExplorationCandidate
}

type SavedCountryControlGroups struct {
	CountryID, EntryIndex, FieldOccurrences, RowCount int
	CountryPath                                       string
	Rows                                              []CountryControlGroupRow
	Tables                                            []CountryExplorationTable
}

func LoadCountryControlGroupsFromArchive(ctx context.Context, archive *zip.Reader, requestedCountry *int, offset int) (SavedCountryControlGroups, error) {
	saved, err := loadRawSavedCountry(ctx, archive, requestedCountry, false)
	if err != nil {
		return SavedCountryControlGroups{}, err
	}
	out := SavedCountryControlGroups{
		CountryID: saved.CountryID, EntryIndex: saved.EntryIndex, CountryPath: saved.Path,
		Rows: make([]CountryControlGroupRow, 0, CountryControlGroupsPageSize),
	}
	add := func(node *clausewitz.RawNode, parentPath, parentKind, slotPath, role string, index int) {
		if out.RowCount >= offset && len(out.Rows) < CountryControlGroupsPageSize {
			out.Rows = append(out.Rows, CountryControlGroupRow{
				Path: parentPath + "/" + strconv.Itoa(index), ContainerPath: parentPath, ContainerKind: parentKind,
				Role: role, SlotPath: slotPath, Index: index, Node: node,
			})
		}
		out.RowCount++
	}
	for i, field := range saved.Node.Children {
		if field.Key == nil || field.Key.Text != "control_groups" {
			continue
		}
		out.FieldOccurrences++
		fieldPath := saved.Path + "/" + strconv.Itoa(i)
		if len(field.Children) == 0 {
			role := "unsupported-control-groups"
			if field.Kind == "empty_block" {
				role = "empty-control-groups"
			}
			add(field, saved.Path, saved.Node.Kind, "", role, i)
			continue
		}
		for j, slot := range field.Children {
			if err := ctx.Err(); err != nil {
				return SavedCountryControlGroups{}, err
			}
			slotPath := fieldPath + "/" + strconv.Itoa(j)
			role := "slot"
			if slot.Key == nil {
				role = "anonymous-slot"
			}
			add(slot, fieldPath, field.Kind, slotPath, role, j)
			if slot.Kind != "list" && slot.Kind != "mixed" {
				continue
			}
			for k, item := range slot.Children {
				if err := ctx.Err(); err != nil {
					return SavedCountryControlGroups{}, err
				}
				role := "unsupported-item"
				if item.Key == nil {
					switch item.Kind {
					case "object":
						role = "item"
					case "mixed":
						role = "mixed-item"
					case "empty_block":
						role = "empty-item"
					}
				}
				add(item, slotPath, slot.Kind, slotPath, role, k)
			}
		}
	}
	if offset > out.RowCount {
		return SavedCountryControlGroups{}, fmt.Errorf("offset exceeds saved row count")
	}

	out.Tables = []CountryExplorationTable{
		explorationTable(saved.root, "fleet", "fleet"),
		explorationTable(saved.root, "ships", "ships"),
		explorationTable(saved.root, "ship_design", "ship_design"),
	}
	for i := range out.Rows {
		row := &out.Rows[i]
		if row.Role != "item" && row.Role != "mixed-item" {
			continue
		}
		row.ID = readControlGroupField(row.Node, row.Path, "id")
		row.Type = readControlGroupField(row.Node, row.Path, "type")
		if row.ID.Status == "integer" {
			for _, table := range out.Tables {
				row.NumericCandidates = append(row.NumericCandidates, table.candidate(row.ID.number))
			}
		}
	}
	return out, nil
}

func readControlGroupField(node *clausewitz.RawNode, path, key string) ControlGroupField {
	field := ControlGroupField{Status: "not-recorded"}
	for i, child := range node.Children {
		if child.Key == nil || child.Key.Text != key {
			continue
		}
		field.Occurrences++
		if len(field.Preview) < controlGroupFieldPreviewSize {
			field.Preview = append(field.Preview, ControlGroupFieldEntry{Path: path + "/" + strconv.Itoa(i), Node: child})
		}
	}
	switch field.Occurrences {
	case 0:
	case 1:
		field.Status = "unsupported"
		if node := field.Preview[0].Node; node.IsScalar() {
			if number, err := strconv.Atoi(node.Scalar.Text); err == nil && (key == "type" || number >= 0) {
				field.Status, field.number = "integer", number
			}
		}
	default:
		field.Status = "duplicate"
	}
	return field
}
