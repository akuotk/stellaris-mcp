package gamestate

import (
	"archive/zip"
	"fmt"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type SavedCompletedTechnology struct {
	Path, Kind, Key string
	Value           clausewitz.Value
	AdjacentLevel   *clausewitz.Value
	LevelPath       string
	Level           *int
	RepeatedKey     bool
}

type SavedCompletedTechnologies struct {
	CountryID        int
	TechStatusCount  int
	UnsupportedCount int
	TechnologyCount  int
	Entries          []SavedCompletedTechnology
}

func LoadCompletedTechnologiesFromArchive(archive *zip.Reader, requestedCountry *int) (SavedCompletedTechnologies, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return SavedCompletedTechnologies{}, fmt.Errorf("cannot read selected gamestate")
	}
	doc, err := clausewitz.Parse(data)
	if err != nil {
		return SavedCompletedTechnologies{}, fmt.Errorf("cannot parse selected gamestate")
	}
	countryID, country, path, err := selectSavedCountry(doc, requestedCountry)
	if err != nil {
		return SavedCompletedTechnologies{}, err
	}
	result := SavedCompletedTechnologies{CountryID: countryID}
	counts := map[string]int{}
	for i, field := range country.Object.Pairs {
		if field.Key != "tech_status" {
			continue
		}
		result.TechStatusCount++
		statusPath := path + "/" + strconv.Itoa(i)
		status := field.Value
		switch {
		case !status.IsObject():
			result.UnsupportedCount++
			result.Entries = append(result.Entries, SavedCompletedTechnology{Path: statusPath, Kind: "unsupported-tech-status", Value: status})
		case len(status.Object.Pairs) == 0:
			result.Entries = append(result.Entries, SavedCompletedTechnology{Path: statusPath, Kind: "empty-tech-status", Value: status})
		default:
			start := len(result.Entries)
			for j, pair := range status.Object.Pairs {
				entryPath := statusPath + "/" + strconv.Itoa(j)
				if pair.Key == "level" && (j == 0 || status.Object.Pairs[j-1].Key != "technology") {
					result.Entries = append(result.Entries, SavedCompletedTechnology{Path: entryPath, Kind: "unpaired-level", Value: pair.Value})
				}
				if pair.Key != "technology" {
					continue
				}
				result.TechnologyCount++
				entry := SavedCompletedTechnology{Path: entryPath, Kind: "technology", Value: pair.Value}
				if pair.Value.IsScalar() && strings.TrimSpace(pair.Value.Scalar) != "" {
					entry.Key = pair.Value.Scalar
					counts[entry.Key]++
				}
				if j+1 < len(status.Object.Pairs) && status.Object.Pairs[j+1].Key == "level" {
					level := status.Object.Pairs[j+1].Value
					entry.AdjacentLevel = &level
					entry.LevelPath = statusPath + "/" + strconv.Itoa(j+1)
					if entry.Key != "" && level.IsScalar() {
						if number, err := strconv.Atoi(level.Scalar); err == nil {
							entry.Level = &number
						}
					}
				}
				result.Entries = append(result.Entries, entry)
			}
			if len(result.Entries) == start {
				result.Entries = append(result.Entries, SavedCompletedTechnology{Path: statusPath, Kind: "no-completed-fields"})
			}
		}
	}
	for i := range result.Entries {
		entry := &result.Entries[i]
		entry.RepeatedKey = entry.Key != "" && counts[entry.Key] > 1
	}
	return result, nil
}
