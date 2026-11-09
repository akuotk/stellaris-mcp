package gamestate

import (
	"archive/zip"
	"fmt"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type SavedFlagEntry struct {
	Path            string
	Value           clausewitz.Value
	Key             *string
	RepeatedKey     bool
	UnsupportedRoot bool
}

type SavedFlags struct {
	Roots   int
	Entries []SavedFlagEntry
}

func LoadSavedFlagsFromArchive(archive *zip.Reader) (SavedFlags, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return SavedFlags{}, err
	}
	root, err := clausewitz.Parse(data)
	if err != nil {
		return SavedFlags{}, fmt.Errorf("parse gamestate: %w", err)
	}
	var flags SavedFlags
	for i, pair := range root.Pairs {
		if pair.Key != "flags" {
			continue
		}
		flags.Roots++
		path := fmt.Sprintf("root[%d] key=%q", i, pair.Key)
		value := pair.Value
		if !value.IsObject() || len(value.Object.Pairs) == 0 {
			flags.Entries = append(flags.Entries, SavedFlagEntry{Path: path, Value: value, UnsupportedRoot: !value.IsObject()})
			continue
		}
		counts := map[string]int{}
		for _, field := range value.Object.Pairs {
			counts[field.Key]++
		}
		for j, field := range value.Object.Pairs {
			flags.Entries = append(flags.Entries, SavedFlagEntry{
				Path:  fmt.Sprintf("%s/field[%d] key=%q", path, j, field.Key),
				Value: field.Value, Key: &field.Key, RepeatedKey: counts[field.Key] > 1,
			})
		}
	}
	return flags, nil
}
