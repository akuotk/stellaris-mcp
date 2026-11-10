package gamestate

import (
	"archive/zip"
	"context"
	"fmt"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type SavedRawCountry struct {
	CountryID, EntryIndex int
	Path                  string
	Node                  *clausewitz.RawNode
	source                []byte
}

func LoadCountrySavedFieldsFromArchive(ctx context.Context, archive *zip.Reader, requestedCountry *int) (SavedRawCountry, error) {
	return loadRawSavedCountry(ctx, archive, requestedCountry, true)
}

func loadRawSavedCountry(ctx context.Context, archive *zip.Reader, requestedCountry *int, allowMixedRecord bool) (SavedRawCountry, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return SavedRawCountry{}, fmt.Errorf("cannot read selected gamestate")
	}
	root, err := clausewitz.ParseRaw(ctx, data)
	if err != nil {
		return SavedRawCountry{}, fmt.Errorf("cannot parse selected gamestate")
	}

	// Project only the fields needed by the selector, keeping their positional indexes.
	doc := &clausewitz.Object{Pairs: make([]clausewitz.Pair, len(root.Children))}
	for i, node := range root.Children {
		if node.Key == nil {
			continue
		}
		pair := &doc.Pairs[i]
		pair.Key = node.Key.Text
		switch pair.Key {
		case "player":
			if requestedCountry == nil {
				pair.Value = projectRawCountrySelection(node, 3)
			}
		case "country":
			pair.Value = projectRawCountrySelection(node, 1)
			if allowMixedRecord && pair.Value.IsObject() {
				// A mixed record is browsable as raw data if its table key is unique.
				for j, record := range node.Children {
					if record.Kind == "mixed" {
						pair.Value.Object.Pairs[j].Value = clausewitz.Value{Object: &clausewitz.Object{}}
					}
				}
			}
		}
	}
	id, _, path, err := selectSavedCountry(doc, requestedCountry)
	if err != nil {
		return SavedRawCountry{}, err
	}
	var rootIndex, countryIndex int
	if _, err := fmt.Sscanf(path, "/%d/%d", &rootIndex, &countryIndex); err != nil {
		return SavedRawCountry{}, fmt.Errorf("invalid selected country position")
	}
	for i, f := range archive.File {
		if f.Name == "gamestate" {
			return SavedRawCountry{id, i, path, root.Children[rootIndex].Children[countryIndex], data}, nil
		}
	}
	return SavedRawCountry{}, fmt.Errorf("cannot locate selected gamestate")
}

func projectRawCountrySelection(node *clausewitz.RawNode, depth int) clausewitz.Value {
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
			items[i] = projectRawCountrySelection(child, depth-1)
		}
		return clausewitz.Value{List: items}
	default:
		obj := &clausewitz.Object{}
		if depth > 0 {
			obj.Pairs = make([]clausewitz.Pair, len(node.Children))
			for i, child := range node.Children {
				obj.Pairs[i] = clausewitz.Pair{Key: child.Key.Text, Value: projectRawCountrySelection(child, depth-1)}
			}
		}
		return clausewitz.Value{Object: obj}
	}
}
