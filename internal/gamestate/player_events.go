package gamestate

import (
	"archive/zip"
	"fmt"
	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type PlayerEventRecord struct {
	Path         string
	Kind         string
	Value        clausewitz.Value
	Country      OptionalIntRef
	EventID      OptionalIntRef
	HistoryEntry bool
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
			record := PlayerEventRecord{Path: path, Kind: "event", Value: pair.Value}
			if pair.Value.IsObject() {
				record.Country = savedPathInt(pair.Value, "country")
				record.EventID = savedPathInt(pair.Value, "id")
				if record.Country.Value != nil {
					ids[*record.Country.Value] = true
				}
			}
			result.Records = append(result.Records, record)
		} else if pair.Key == "open_player_event_selection_history" {
			if !pair.Value.IsObject() || len(pair.Value.Object.Pairs) == 0 {
				result.Records = append(result.Records, PlayerEventRecord{Path: path, Kind: "history", Value: pair.Value})
				continue
			}
			for fieldIndex, field := range pair.Value.Object.Pairs {
				fieldPath := fmt.Sprintf("%s/field[%d] key=%q", path, fieldIndex, field.Key)
				if field.Key == "selected" && field.Value.IsList() && len(field.Value.List) > 0 {
					for i, item := range field.Value.List {
						record := PlayerEventRecord{Path: fmt.Sprintf("%s/item[%d]", fieldPath, i), Kind: "history", HistoryEntry: true, Value: item}
						if item.IsObject() {
							record.EventID = savedPathInt(item, "player_event")
						}
						result.Records = append(result.Records, record)
					}
				} else {
					result.Records = append(result.Records, PlayerEventRecord{Path: fieldPath, Kind: "history", Value: field.Value})
				}
			}
		}
	}
	result.Countries = systemSubsetTable[SpaceObjectCandidate](clausewitz.Value{Object: root}, ids, "country")
	return result, nil
}
