package gamestate

import (
	"archive/zip"
	"fmt"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

// LoadGalaxyRecordsFromArchive retains all root galaxy records in saved order.
func LoadGalaxyRecordsFromArchive(archive *zip.Reader) ([]clausewitz.Value, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return nil, err
	}
	root, err := clausewitz.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse gamestate: %w", err)
	}
	var records []clausewitz.Value
	for _, pair := range root.Pairs {
		if pair.Key == "galaxy" {
			records = append(records, pair.Value)
		}
	}
	return records, nil
}
