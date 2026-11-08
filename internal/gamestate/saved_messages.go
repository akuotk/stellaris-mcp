package gamestate

import (
	"archive/zip"
	"fmt"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type SavedMessage struct {
	RootIndex int
	Value     clausewitz.Value
}

func LoadSavedMessagesFromArchive(archive *zip.Reader) ([]SavedMessage, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return nil, err
	}
	root, err := clausewitz.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse gamestate: %w", err)
	}
	var messages []SavedMessage
	for i, pair := range root.Pairs {
		if pair.Key == "message" {
			messages = append(messages, SavedMessage{RootIndex: i, Value: pair.Value})
		}
	}
	return messages, nil
}
