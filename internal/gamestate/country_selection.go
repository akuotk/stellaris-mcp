package gamestate

import (
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type savedCountryPosition struct {
	index             int
	duplicate, object bool
}

type savedCountryIndex struct {
	rootIndex int
	issue     string
	byID      map[int]savedCountryPosition
}

func indexSavedCountries(doc *clausewitz.Object) *savedCountryIndex {
	result := &savedCountryIndex{rootIndex: -1}
	var table clausewitz.Value
	for i, field := range doc.Pairs {
		if field.Key != "country" {
			continue
		}
		if result.rootIndex >= 0 {
			result.issue = "saved country table is ambiguous"
			return result
		}
		table, result.rootIndex = field.Value, i
	}
	if !table.IsObject() {
		result.issue = "saved country table is unavailable"
		return result
	}
	result.byID = make(map[int]savedCountryPosition, len(table.Object.Pairs))
	for i, field := range table.Object.Pairs {
		id, err := strconv.Atoi(field.Key)
		if err != nil {
			continue
		}
		_, duplicate := result.byID[id]
		result.byID[id] = savedCountryPosition{index: i, duplicate: duplicate, object: field.Value.IsObject()}
	}
	return result
}

func (countries *savedCountryIndex) position(id int) (int, error) {
	if countries.issue != "" {
		return 0, fmt.Errorf("%s", countries.issue)
	}
	entry, exists := countries.byID[id]
	switch {
	case entry.duplicate:
		return 0, fmt.Errorf("selected saved country is ambiguous")
	case !exists || !entry.object:
		return 0, fmt.Errorf("selected saved country is unavailable")
	default:
		return entry.index, nil
	}
}

func readSavedPlayers(doc *clausewitz.Object) (players []Player, incomplete bool) {
	emptyRoot := false
	for _, field := range doc.Pairs {
		if field.Key != "player" {
			continue
		}
		if players == nil {
			players = []Player{}
		}
		var records []clausewitz.Value
		switch {
		case field.Value.IsList():
			records = field.Value.List
		case field.Value.IsObject():
			if len(field.Value.Object.Pairs) == 0 {
				emptyRoot = true
			} else {
				records = []clausewitz.Value{field.Value}
			}
		default:
			incomplete = true
		}
		for _, record := range records {
			if !record.IsObject() {
				incomplete = true
				continue
			}
			var player Player
			for _, pair := range record.Object.Pairs {
				if pair.Key == "name" {
					if pair.Value.IsScalar() {
						player.Name = pair.Value.Scalar
					}
					break
				}
			}
			ref := savedPathInt(record, "country")
			if ref.Unsupported || ref.Value == nil || *ref.Value < 0 {
				incomplete = true
			} else {
				player.CountryID, player.Country = ref.Value, *ref.Value
			}
			players = append(players, player)
		}
	}
	return players, incomplete || (emptyRoot && len(players) > 0)
}

func selectCountryID(players []Player, incomplete bool, requested *int) (int, error) {
	if requested != nil {
		if *requested < 0 {
			return 0, fmt.Errorf("country_id must be nonnegative")
		}
		return *requested, nil
	}
	if !incomplete && len(players) > 0 && players[0].CountryID != nil && *players[0].CountryID >= 0 {
		id := *players[0].CountryID
		for _, player := range players[1:] {
			if player.CountryID == nil || *player.CountryID != id {
				return 0, fmt.Errorf("cannot determine a unique player country; specify country_id")
			}
		}
		return id, nil
	}
	return 0, fmt.Errorf("cannot determine a unique player country; specify country_id")
}

func (gs *GameState) SelectCountry(requested *int) (int, *Country, error) {
	id, err := selectCountryID(gs.Player, gs.playerRecordsIncomplete, requested)
	if err != nil {
		return 0, nil, err
	}
	if gs.savedCountryIndex != nil {
		if _, err := gs.savedCountryIndex.position(id); err != nil {
			return 0, nil, err
		}
	}
	country, ok := gs.Country[id]
	if !ok {
		return 0, nil, fmt.Errorf("country %d not found", id)
	}
	return id, &country, nil
}

func selectSavedCountry(doc *clausewitz.Object, requested *int) (int, clausewitz.Value, string, error) {
	var players []Player
	var incomplete bool
	if requested == nil {
		players, incomplete = readSavedPlayers(doc)
	}
	id, err := selectCountryID(players, incomplete, requested)
	if err != nil {
		return 0, clausewitz.Value{}, "", err
	}
	countries := indexSavedCountries(doc)
	position, err := countries.position(id)
	if err != nil {
		return 0, clausewitz.Value{}, "", err
	}
	record := doc.Pairs[countries.rootIndex].Value.Object.Pairs[position].Value
	return id, record, fmt.Sprintf("/%d/%d", countries.rootIndex, position), nil
}
