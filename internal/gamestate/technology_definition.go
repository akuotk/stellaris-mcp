package gamestate

import (
	"archive/zip"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

const (
	MaxTechnologyKeyBytes = 256
	maxTechnologyRecords  = 8192
	maxTechnologyNodes    = 150000
	maxTechnologyText     = 8 << 20
)

type TechnologyDefinition struct {
	Source string
	Value  *clausewitz.RawNode
}

type TechnologyDefinitionIndex struct {
	Status, Fingerprint string
	byKey               map[string][]TechnologyDefinition
}

func (index TechnologyDefinitionIndex) Definitions(key string) []TechnologyDefinition {
	return index.byKey[key]
}

type TechnologyDefinitionSource func() TechnologyDefinitionIndex

func NewTechnologyDefinitionSource(localizationDir string) TechnologyDefinitionSource {
	return sync.OnceValue(func() TechnologyDefinitionIndex {
		index := TechnologyDefinitionIndex{byKey: map[string][]TechnologyDefinition{}}
		var records, nodes, textBytes int
		index.Status, index.Fingerprint = scanInstalledScripts(localizationDir, "technology-definitions-v1", []string{"common/technology"}, technologyZipMember,
			func(ctx context.Context, data []byte, _ string, archived bool) bool {
				doc, err := clausewitz.ParseRaw(ctx, data)
				if err != nil {
					return false
				}
				source := "common/technology"
				if archived {
					source = "dlc-zip"
				}
				complete := true
				for _, node := range doc.Children {
					if ctx.Err() != nil || records == maxTechnologyRecords {
						return false
					}
					if node.Key == nil {
						complete = false
						continue
					}
					key := node.Key.Text
					if strings.HasPrefix(key, "@") {
						continue
					}
					if key == "" || key == "inline_script" || len(key) > MaxTechnologyKeyBytes || strings.ContainsAny(key, "$[]") {
						complete = false
						continue
					}
					def := TechnologyDefinition{Source: source}
					n, b, fits := technologyTreeSize(node, maxTechnologyNodes-nodes, maxTechnologyText-textBytes)
					if fits {
						nodes += n
						textBytes += b
						def.Value = node
					} else {
						complete = false
					}
					if node.Kind != "object" && node.Kind != "empty_block" {
						complete = false
					}
					index.byKey[key] = append(index.byKey[key], def)
					records++
				}
				return complete
			})
		if records == 0 && index.Status == "complete" {
			index.Status = "unavailable"
		}
		return index
	})
}

func technologyZipMember(name string) bool {
	return strings.Contains("/"+strings.ToLower(name), "/common/technology/")
}

func technologyTreeSize(root *clausewitz.RawNode, nodes, bytes int) (int, int, bool) {
	visited, text := 0, 0
	stack := []*clausewitz.RawNode{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		visited++
		if node.Key != nil {
			text += len(node.Key.Text)
		}
		if node.Scalar != nil {
			text += len(node.Scalar.Text)
		}
		if visited+len(stack)+len(node.Children) > nodes || text > bytes {
			return 0, 0, false
		}
		stack = append(stack, node.Children...)
	}
	return visited, text, true
}

var technologyResearchAreas = [...]string{"physics", "society", "engineering"}

type TechnologySaveMatch struct {
	Field                       string
	Recorded                    bool
	Partial                     bool
	Empty                       bool
	Count                       int
	SpecialProjectOnly          int
	TechnologyAndSpecialProject int
}

func LoadTechnologySaveMatches(archive *zip.Reader, countryID *int, technologyID string) ([]TechnologySaveMatch, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return nil, fmt.Errorf("cannot read selected gamestate")
	}
	doc, err := clausewitz.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse selected gamestate")
	}
	_, country, _, err := selectSavedCountry(doc, countryID)
	if err != nil {
		return nil, err
	}
	status, recorded, unsupported := savedFieldPath(country, "tech_status")
	if !recorded || unsupported || !status.IsObject() {
		return []TechnologySaveMatch{{Field: "tech_status", Recorded: recorded, Partial: recorded}}, nil
	}

	completed := TechnologySaveMatch{Field: "completed"}
	for _, pair := range status.Object.Pairs {
		if pair.Key == "technology" {
			completed.addScalar(pair.Value, technologyID)
		}
	}
	results := []TechnologySaveMatch{completed}
	for _, area := range technologyResearchAreas {
		name := area + "_queue"
		value, present, duplicate := savedFieldPath(status, name)
		queue := TechnologySaveMatch{Field: name, Recorded: present, Partial: duplicate}
		if present && !duplicate {
			switch {
			case value.IsList():
				queue.Empty = len(value.List) == 0
				for _, item := range value.List {
					queue.addQueueItem(item, technologyID)
				}
			case value.IsObject():
				queue.Empty = len(value.Object.Pairs) == 0
				if !queue.Empty {
					queue.addQueueItem(value, technologyID)
				}
			default:
				queue.Partial = true
			}
		}
		results = append(results, queue)
	}
	for _, area := range technologyResearchAreas {
		value, present, duplicate := savedFieldPath(status, "alternatives", area)
		match := TechnologySaveMatch{Field: "alternatives." + area, Recorded: present, Partial: duplicate}
		if present && !duplicate {
			switch {
			case value.IsList():
				for _, item := range value.List {
					match.addScalar(item, technologyID)
				}
			case value.IsScalar():
				match.addScalar(value, technologyID)
			case !value.IsObject() || len(value.Object.Pairs) != 0:
				match.Partial = true
			}
		}
		results = append(results, match)
	}
	return results, nil
}

func (match *TechnologySaveMatch) addScalar(value clausewitz.Value, technologyID string) {
	match.Recorded = true
	if !value.IsScalar() {
		match.Partial = true
	} else if value.Scalar == technologyID {
		match.Count++
	}
}

func (match *TechnologySaveMatch) addQueueItem(value clausewitz.Value, technologyID string) {
	tech, recorded, unsupported := savedFieldPath(value, "technology")
	if unsupported {
		match.Partial = true
		return
	}
	project := savedPathInt(value, "special_project")
	validProject := !project.Unsupported && project.Value != nil && *project.Value >= 0
	match.Partial = match.Partial || (project.Recorded && !validProject)
	switch {
	case recorded:
		if !tech.IsScalar() || strings.TrimSpace(tech.Scalar) == "" {
			match.Partial = true
			return
		}
		match.addScalar(tech, technologyID)
		if validProject {
			match.TechnologyAndSpecialProject++
		}
	case validProject:
		match.SpecialProjectOnly++
	default:
		match.Partial = true
	}
}
