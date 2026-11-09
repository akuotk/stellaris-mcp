package gamestate

import (
	"context"
	"strings"
	"sync"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

const maxTechnologyReferenceLinks = 32768

type technologyReferenceCategory struct {
	name    string
	wrapped bool
}

var technologyReferenceCategories = [...]technologyReferenceCategory{
	{"armies", false}, {"buildings", false}, {"component_sets", true}, {"component_templates", true},
	{"decisions", false}, {"districts", false}, {"edicts", false}, {"megastructures", false},
	{"section_templates", true}, {"ship_sizes", false}, {"starbase_buildings", false},
	{"starbase_levels", false}, {"starbase_modules", false},
}

type TechnologyReference struct {
	Category, Source, DefinitionKey, WrapperKey string
	FieldIndex, FieldOccurrences, Matches       int
	DirectLiteralCount                          int
	Shape                                       string
	Prerequisites                               *clausewitz.RawNode
}

type TechnologyReferenceIndex struct {
	Status, Fingerprint      string
	InlineScriptObserved     bool
	UnresolvedFieldsObserved bool
	byTechnology             map[string][]TechnologyReference
	definitionOccurrences    map[technologyReferenceIdentity]int
}

type technologyReferenceIdentity struct {
	category, key string
}

func (index TechnologyReferenceIndex) References(key string) []TechnologyReference {
	return index.byTechnology[key]
}

func (index TechnologyReferenceIndex) DefinitionOccurrences(reference TechnologyReference) int {
	return index.definitionOccurrences[technologyReferenceIdentity{reference.Category, reference.DefinitionKey}]
}

type TechnologyReferenceSource func() TechnologyReferenceIndex

func NewTechnologyReferenceSource(localizationDir string) TechnologyReferenceSource {
	return sync.OnceValue(func() TechnologyReferenceIndex {
		index := TechnologyReferenceIndex{
			byTechnology:          map[string][]TechnologyReference{},
			definitionOccurrences: map[technologyReferenceIdentity]int{},
		}
		dirs := make([]string, 0, len(technologyReferenceCategories))
		for _, category := range technologyReferenceCategories {
			dirs = append(dirs, "common/"+category.name)
		}
		budget := technologyReferenceBudget{}
		index.Status, index.Fingerprint = scanInstalledScripts(localizationDir, "technology-references-v1", dirs,
			func(name string) bool { return technologyReferenceCategoryFor(name).name != "" },
			func(ctx context.Context, data []byte, name string, archived bool) bool {
				category := technologyReferenceCategoryFor(name)
				source := "common"
				if archived {
					source = "dlc-zip"
				}
				return index.indexTechnologyReferenceFile(ctx, data, category, source, &budget)
			})
		return index
	})
}

func technologyReferenceCategoryFor(name string) technologyReferenceCategory {
	parts := strings.Split(strings.ToLower(name), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "common" {
			continue
		}
		for _, category := range technologyReferenceCategories {
			if parts[i+1] == category.name {
				return category
			}
		}
	}
	return technologyReferenceCategory{}
}

type technologyReferenceBudget struct {
	records, links, nodes, text int
}

func (index *TechnologyReferenceIndex) indexTechnologyReferenceFile(ctx context.Context, data []byte, category technologyReferenceCategory, source string, budget *technologyReferenceBudget) bool {
	doc, err := clausewitz.ParseRaw(ctx, data)
	if err != nil || category.name == "" {
		return false
	}
	complete := true
	for _, definition := range doc.Children {
		if ctx.Err() != nil || budget.records >= maxTechnologyRecords*2 {
			return false
		}
		if definition.Key == nil {
			index.UnresolvedFieldsObserved = true
			complete = false
			continue
		}
		key := definition.Key.Text
		if strings.HasPrefix(key, "@") {
			continue
		}
		if key == "inline_script" || technologyReferenceInline(ctx, definition) {
			index.InlineScriptObserved = true
			complete = false
		}
		if key == "inline_script" {
			continue
		}
		if definition.Kind != "object" && definition.Kind != "empty_block" && definition.Kind != "mixed" {
			index.UnresolvedFieldsObserved = true
			complete = false
			continue
		}
		budget.records++
		base := TechnologyReference{Category: category.name, Source: source}
		base.DefinitionKey, base.WrapperKey = technologyReferenceDefinitionKey(definition, category.wrapped)
		if base.DefinitionKey == "" || definition.Kind == "mixed" {
			index.UnresolvedFieldsObserved = true
			complete = false
		}
		if base.DefinitionKey != "" {
			index.definitionOccurrences[technologyReferenceIdentity{category.name, base.DefinitionKey}]++
		}
		for _, field := range definition.Children {
			if field.Key != nil && field.Key.Text == "prerequisites" {
				base.FieldOccurrences++
			}
		}
		for i, field := range definition.Children {
			if field.Key == nil || field.Key.Text != "prerequisites" {
				continue
			}
			matches, supported := technologyPrerequisiteLiterals(field)
			if !supported || base.FieldOccurrences > 1 {
				index.UnresolvedFieldsObserved = true
				complete = false
			}
			if len(matches) == 0 {
				continue
			}
			if budget.links+len(matches) > maxTechnologyReferenceLinks {
				return false
			}
			budget.links += len(matches)
			reference := base
			reference.FieldIndex, reference.Shape = i, field.Kind
			for _, count := range matches {
				reference.DirectLiteralCount += count
			}
			n, b, fits := technologyTreeSize(field, maxTechnologyNodes-budget.nodes, maxTechnologyText-budget.text)
			if fits {
				budget.nodes += n
				budget.text += b
				reference.Prerequisites = field
			} else {
				complete = false
			}
			for tech, count := range matches {
				reference.Matches = count
				index.byTechnology[tech] = append(index.byTechnology[tech], reference)
			}
		}
	}
	return complete && ctx.Err() == nil
}

func technologyReferenceInline(ctx context.Context, definition *clausewitz.RawNode) bool {
	stack := []*clausewitz.RawNode{definition}
	for len(stack) > 0 && ctx.Err() == nil {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.Key != nil && node.Key.Text == "inline_script" {
			return true
		}
		stack = append(stack, node.Children...)
	}
	return false
}

func technologyReferenceDefinitionKey(definition *clausewitz.RawNode, wrapped bool) (key, wrapper string) {
	rootKey := definition.Key.Text
	if !wrapped {
		if IsTechnologyReferenceLiteral(rootKey) {
			return rootKey, ""
		}
		return "", ""
	}
	if !IsTechnologyReferenceLiteral(rootKey) {
		return "", ""
	}
	wrapper = rootKey
	found := false
	for _, field := range definition.Children {
		if field.Key == nil || field.Key.Text != "key" {
			continue
		}
		if found || field.Scalar == nil || !IsTechnologyReferenceLiteral(field.Scalar.Text) {
			return "", wrapper
		}
		key, found = field.Scalar.Text, true
	}
	return key, wrapper
}

func technologyPrerequisiteLiterals(field *clausewitz.RawNode) (map[string]int, bool) {
	matches := map[string]int{}
	if field.IsScalar() {
		if IsTechnologyReferenceLiteral(field.Scalar.Text) {
			matches[field.Scalar.Text]++
		}
		return matches, false
	}
	supported := field.Kind == "list" || field.Kind == "empty_block"
	for _, child := range field.Children {
		if child.Key == nil && child.IsScalar() && IsTechnologyReferenceLiteral(child.Scalar.Text) {
			matches[child.Scalar.Text]++
		} else {
			supported = false
		}
	}
	return matches, supported
}

func IsTechnologyReferenceLiteral(value string) bool {
	if len(value) == 0 || len(value) > MaxTechnologyKeyBytes || strings.HasPrefix(value, "scope:") || strings.HasPrefix(value, "event_target:") {
		return false
	}
	switch value {
	case "yes", "no", "none":
		return false
	}
	for i, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_' ||
			(i > 0 && ((char >= '0' && char <= '9') || char == '.' || char == ':' || char == '-')) {
			continue
		}
		return false
	}
	return true
}
