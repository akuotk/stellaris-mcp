package gamestate

import (
	"archive/zip"
	"fmt"
	"io"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

const SaveRootListPreview = 100

// SaveRootField is one direct child of a save entry's root, in saved order.
type SaveRootField struct {
	Index      int
	Key        string
	Shape      string // scalar, object, list, empty, mixed
	Scalar     string
	Values     []string // scalar list items, first SaveRootListPreview
	ListTotal  int
	Scalars    int  // scalar items in the list
	ListMixed  bool // list contains non-scalar items
	Children   int
	Repeated   bool
	Occurrence int
}

type SaveRootEntry struct {
	Name   string
	Issue  string
	Fields []SaveRootField
}

// LoadSaveRootFieldsFromArchive reads the root fields of the named entries (meta and
// gamestate when none are given); only the requested entries are parsed.
func LoadSaveRootFieldsFromArchive(archive *zip.Reader, names ...string) []SaveRootEntry {
	if len(names) == 0 {
		names = []string{"meta", "gamestate"}
	}
	var out []SaveRootEntry
	for _, name := range names {
		entry := SaveRootEntry{Name: name}
		var selected *zip.File
		count := 0
		for _, f := range archive.File {
			if f.Name == name {
				selected = f
				count++
			}
		}
		switch {
		case count == 0:
			entry.Issue = "entry not present"
		case count > 1:
			entry.Issue = "multiple entries with this name; use inspect_save_archive and raw tools"
		case selected.FileInfo().IsDir():
			entry.Issue = "entry is a directory"
		default:
			fields, err := readSaveRootFields(selected)
			if err != nil {
				entry.Issue = err.Error()
			}
			entry.Fields = fields
		}
		out = append(out, entry)
	}
	return out
}

func readSaveRootFields(f *zip.File) ([]SaveRootField, error) {
	stream, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open entry: %w", err)
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		return nil, fmt.Errorf("read entry: %w", err)
	}
	root, err := clausewitz.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse entry: unsupported syntax; use read_save_bytes")
	}
	total := map[string]int{}
	for _, pair := range root.Pairs {
		total[pair.Key]++
	}
	seen := map[string]int{}
	fields := make([]SaveRootField, 0, len(root.Pairs))
	for i, pair := range root.Pairs {
		seen[pair.Key]++
		field := SaveRootField{Index: i, Key: pair.Key, Repeated: total[pair.Key] > 1, Occurrence: seen[pair.Key]}
		value := pair.Value
		switch {
		case value.Mixed:
			field.Shape = "mixed"
		case value.IsObject() && len(value.Object.Pairs) == 0, value.IsList() && len(value.List) == 0:
			field.Shape = "empty"
		case value.IsObject():
			field.Shape, field.Children = "object", len(value.Object.Pairs)
		case value.IsList():
			field.Shape, field.ListTotal = "list", len(value.List)
			for _, item := range value.List {
				if !item.IsScalar() {
					field.ListMixed = true
					continue
				}
				field.Scalars++
				if len(field.Values) < SaveRootListPreview {
					field.Values = append(field.Values, item.Scalar)
				}
			}
		default:
			field.Shape, field.Scalar = "scalar", value.Scalar
		}
		fields = append(fields, field)
	}
	return fields, nil
}
