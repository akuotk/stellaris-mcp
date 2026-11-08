package gamestate

import (
	"archive/zip"
	"fmt"
	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type PlayerEventRecord struct {
	Path    string
	Value   clausewitz.Value
	Country OptionalIntRef
}
type PlayerEventRecords struct {
	Records   []PlayerEventRecord
	Countries ConnectionRecordTable[SpaceObjectCandidate]
}

func LoadPlayerEventRecordsFromArchive(archive *zip.Reader) (PlayerEventRecords, error) {
	var result PlayerEventRecords
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return result, err
	}
	root, err := clausewitz.Parse(data)
	if err != nil {
		return result, fmt.Errorf("parse gamestate: %w", err)
	}
	ids := map[int]bool{}
	for rootIndex, pair := range root.Pairs {
		path := fmt.Sprintf("root[%d] key=%q", rootIndex, pair.Key)
		if pair.Key == "player_event" {
			record := PlayerEventRecord{Path: path, Value: pair.Value}
			if pair.Value.IsObject() {
				record.Country = savedPathInt(pair.Value, "country")
				if record.Country.Value != nil {
					ids[*record.Country.Value] = true
				}
			}
			result.Records = append(result.Records, record)
		} else if pair.Key == "open_player_event_selection_history" {
			if !pair.Value.IsObject() || len(pair.Value.Object.Pairs) == 0 {
				result.Records = append(result.Records, PlayerEventRecord{Path: path, Value: pair.Value})
				continue
			}
			for fieldIndex, field := range pair.Value.Object.Pairs {
				fieldPath := fmt.Sprintf("%s/field[%d] key=%q", path, fieldIndex, field.Key)
				if field.Key == "selected" && field.Value.IsList() && len(field.Value.List) > 0 {
					for i, item := range field.Value.List {
						result.Records = append(result.Records, PlayerEventRecord{Path: fmt.Sprintf("%s/item[%d]", fieldPath, i), Value: item})
					}
				} else {
					result.Records = append(result.Records, PlayerEventRecord{Path: fieldPath, Value: field.Value})
				}
			}
		}
	}
	result.Countries = systemSubsetTable[SpaceObjectCandidate](clausewitz.Value{Object: root}, ids, "country")
	return result, nil
}
