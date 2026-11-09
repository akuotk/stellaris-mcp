package gamestate

import (
	"archive/zip"
	"context"
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type SavedCountryTimedModifiers struct {
	CountryID, WrapperCount, ItemsCount, ItemCount int
	DuplicateItemsWrappers                         int
	Entries                                        []SavedCountryTimedModifierEntry
	Source                                         []byte
	KeyCounts                                      map[string]int
}

type SavedCountryTimedModifierEntry struct {
	Path, Kind, ContainerKind string
	Node                      *clausewitz.RawNode
}

func LoadCountryTimedModifiersFromArchive(ctx context.Context, archive *zip.Reader, requestedCountry *int) (SavedCountryTimedModifiers, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return SavedCountryTimedModifiers{}, fmt.Errorf("cannot read selected gamestate")
	}
	root, err := clausewitz.ParseRaw(ctx, data)
	if err != nil {
		return SavedCountryTimedModifiers{}, fmt.Errorf("cannot parse selected gamestate")
	}

	// Project only the fields needed by the shared country selector, retaining root and table positions.
	doc := &clausewitz.Object{Pairs: make([]clausewitz.Pair, len(root.Children))}
	for i, node := range root.Children {
		if node.Key == nil {
			continue
		}
		pair := &doc.Pairs[i]
		pair.Key = node.Key.Text
		switch pair.Key {
		case "player":
			pair.Value = projectTimedModifierCountrySelection(node, 3)
		case "country":
			pair.Value = projectTimedModifierCountrySelection(node, 1)
		}
	}
	id, _, path, err := selectSavedCountry(doc, requestedCountry)
	if err != nil {
		return SavedCountryTimedModifiers{}, err
	}
	var rootIndex, countryIndex int
	if _, err := fmt.Sscanf(path, "/%d/%d", &rootIndex, &countryIndex); err != nil {
		return SavedCountryTimedModifiers{}, fmt.Errorf("invalid selected country position")
	}
	country := root.Children[rootIndex].Children[countryIndex]
	out := SavedCountryTimedModifiers{CountryID: id, Source: data, KeyCounts: make(map[string]int)}
	add := func(path, kind, containerKind string, node *clausewitz.RawNode) {
		out.Entries = append(out.Entries, SavedCountryTimedModifierEntry{Path: path, Kind: kind, ContainerKind: containerKind, Node: node})
	}
	for i, wrapper := range country.Children {
		if wrapper.Key == nil || wrapper.Key.Text != "timed_modifier" {
			continue
		}
		out.WrapperCount++
		wrapperPath := path + "/" + strconv.Itoa(i)
		if len(wrapper.Children) == 0 {
			add(wrapperPath, "timed_modifier", country.Kind, wrapper)
			continue
		}
		itemFields := 0
		for j, field := range wrapper.Children {
			fieldPath := wrapperPath + "/" + strconv.Itoa(j)
			if field.Key == nil || field.Key.Text != "items" {
				add(fieldPath, "wrapper_field", wrapper.Kind, field)
				continue
			}
			itemFields++
			out.ItemsCount++
			if len(field.Children) == 0 {
				add(fieldPath, "items", wrapper.Kind, field)
				continue
			}
			for k, item := range field.Children {
				add(fieldPath+"/"+strconv.Itoa(k), "item", field.Kind, item)
				out.ItemCount++
				for _, child := range item.Children {
					if child.Key != nil && child.Key.Text == "modifier" && child.Scalar != nil {
						out.KeyCounts[child.Scalar.Text]++
					}
				}
			}
		}
		if itemFields > 1 {
			out.DuplicateItemsWrappers++
		}
	}
	return out, nil
}

func projectTimedModifierCountrySelection(node *clausewitz.RawNode, depth int) clausewitz.Value {
	switch node.Kind {
	case "scalar":
		return clausewitz.Value{Scalar: node.Scalar.Text}
	case "mixed":
		return clausewitz.Value{Mixed: true}
	case "list":
		if depth == 0 {
			return clausewitz.Value{List: []clausewitz.Value{}}
		}
		items := make([]clausewitz.Value, len(node.Children))
		for i, child := range node.Children {
			items[i] = projectTimedModifierCountrySelection(child, depth-1)
		}
		return clausewitz.Value{List: items}
	default:
		obj := &clausewitz.Object{}
		if depth > 0 {
			obj.Pairs = make([]clausewitz.Pair, len(node.Children))
			for i, child := range node.Children {
				obj.Pairs[i] = clausewitz.Pair{Key: child.Key.Text, Value: projectTimedModifierCountrySelection(child, depth-1)}
			}
		}
		return clausewitz.Value{Object: obj}
	}
}
