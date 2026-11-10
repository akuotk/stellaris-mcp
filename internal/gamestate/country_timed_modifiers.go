package gamestate

import (
	"archive/zip"
	"context"
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
	saved, err := loadRawSavedCountry(ctx, archive, requestedCountry, false)
	if err != nil {
		return SavedCountryTimedModifiers{}, err
	}
	country := saved.Node
	out := SavedCountryTimedModifiers{CountryID: saved.CountryID, Source: saved.source, KeyCounts: make(map[string]int)}
	add := func(path, kind, containerKind string, node *clausewitz.RawNode) {
		out.Entries = append(out.Entries, SavedCountryTimedModifierEntry{Path: path, Kind: kind, ContainerKind: containerKind, Node: node})
	}
	for i, wrapper := range country.Children {
		if wrapper.Key == nil || wrapper.Key.Text != "timed_modifier" {
			continue
		}
		out.WrapperCount++
		wrapperPath := saved.Path + "/" + strconv.Itoa(i)
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
