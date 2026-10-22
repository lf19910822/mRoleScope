package jdanalysis

import (
	"fmt"
	"sort"
	"strings"
)

const (
	SourceFieldDescription = "description"
	SourceFieldJobRequire  = "jobRequire"
	SourceFieldAddition    = "addition"
)

func AllowedSourceField(field string) bool {
	return field == SourceFieldDescription || field == SourceFieldJobRequire || field == SourceFieldAddition
}

// ByteMatch is an intermediate matching result. Byte coordinates never leave
// this matcher: evidence persistence converts them to Unicode code-point
// coordinates through the single conversion path added beside this package.
type ByteMatch struct {
	SourceField     string
	TechnologyKey   string
	CanonicalName   string
	RuleKey         string
	MatchedAlias    string
	ByteStart       int
	ByteEnd         int
	OverlapPriority int
}

// Matcher contains only rules that are explicitly enabled and approved. It
// has no candidate heuristics, fuzzy matching, source-tag input, or implicit
// ecosystem expansion.
type Matcher struct {
	rules []DictionaryRule
}

func NewMatcher(rules []DictionaryRule) (Matcher, error) {
	executable := make([]DictionaryRule, 0, len(rules))
	for _, rule := range rules {
		if err := rule.Validate(); err != nil {
			return Matcher{}, err
		}
		if !rule.Executable() {
			continue
		}
		if rule.CaseMode == CaseInsensitive && !isASCII(rule.Alias) {
			return Matcher{}, fmt.Errorf("case-insensitive aliases must be ASCII until a Unicode case-coordinate rule is explicitly introduced")
		}
		executable = append(executable, rule)
	}
	return Matcher{rules: executable}, nil
}

// MatchField analyzes exactly one of the three approved body fields. Passing
// title, jobSummary, source tags, or any other source field is rejected so an
// accidental caller cannot silently broaden the analysis scope.
func (matcher Matcher) MatchField(sourceField, normalizedText string) ([]ByteMatch, error) {
	if !AllowedSourceField(sourceField) {
		return nil, fmt.Errorf("source field is outside the approved JD body scope")
	}
	candidates := make([]ByteMatch, 0)
	for _, rule := range matcher.rules {
		for start := 0; start+len(rule.Alias) <= len(normalizedText); start++ {
			end := start + len(rule.Alias)
			if !ruleMatchesAt(rule, normalizedText, start, end) || !hasBoundary(rule.BoundaryMode, normalizedText, start, end) {
				continue
			}
			candidates = append(candidates, ByteMatch{
				SourceField: sourceField, TechnologyKey: rule.TechnologyKey, CanonicalName: rule.CanonicalName,
				RuleKey: rule.RuleKey, MatchedAlias: rule.Alias, ByteStart: start, ByteEnd: end,
				OverlapPriority: rule.OverlapPriority,
			})
		}
	}
	return selectLongestNonOverlapping(candidates), nil
}

func (matcher Matcher) MatchBodyFields(fields map[string]string) ([]ByteMatch, error) {
	for field := range fields {
		if !AllowedSourceField(field) {
			return nil, fmt.Errorf("source field is outside the approved JD body scope")
		}
	}
	results := make([]ByteMatch, 0)
	for _, field := range []string{SourceFieldDescription, SourceFieldJobRequire, SourceFieldAddition} {
		text, exists := fields[field]
		if !exists {
			continue
		}
		matches, err := matcher.MatchField(field, text)
		if err != nil {
			return nil, err
		}
		results = append(results, matches...)
	}
	return results, nil
}

func ruleMatchesAt(rule DictionaryRule, text string, start, end int) bool {
	segment := text[start:end]
	if rule.CaseMode == CaseSensitive {
		return segment == rule.Alias
	}
	return isASCII(segment) && strings.EqualFold(segment, rule.Alias)
}

func hasBoundary(mode BoundaryMode, text string, start, end int) bool {
	switch mode {
	case BoundaryExact:
		return true
	case BoundaryASCIIIdentifier, BoundaryCustom:
		return !isTechnicalIdentifierByte(byteBefore(text, start)) && !isTechnicalIdentifierByte(byteAfter(text, end))
	default:
		return false
	}
}

func byteBefore(text string, offset int) byte {
	if offset == 0 {
		return 0
	}
	return text[offset-1]
}

func byteAfter(text string, offset int) byte {
	if offset >= len(text) {
		return 0
	}
	return text[offset]
}

// Hyphen is included intentionally: it prevents a language rule for Go from
// matching inside go-zero, and TensorRT from matching inside TensorRT-LLM.
func isTechnicalIdentifierByte(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9') || value == '_' || value == '-'
}

func isASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] > 0x7f {
			return false
		}
	}
	return true
}

func selectLongestNonOverlapping(candidates []ByteMatch) []ByteMatch {
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].ByteStart != candidates[right].ByteStart {
			return candidates[left].ByteStart < candidates[right].ByteStart
		}
		leftLength := candidates[left].ByteEnd - candidates[left].ByteStart
		rightLength := candidates[right].ByteEnd - candidates[right].ByteStart
		if leftLength != rightLength {
			return leftLength > rightLength
		}
		if candidates[left].OverlapPriority != candidates[right].OverlapPriority {
			return candidates[left].OverlapPriority > candidates[right].OverlapPriority
		}
		return candidates[left].RuleKey < candidates[right].RuleKey
	})
	selected := make([]ByteMatch, 0, len(candidates))
	for _, candidate := range candidates {
		overlaps := false
		for _, existing := range selected {
			if candidate.ByteStart < existing.ByteEnd && existing.ByteStart < candidate.ByteEnd {
				overlaps = true
				break
			}
		}
		if !overlaps {
			selected = append(selected, candidate)
		}
	}
	sort.Slice(selected, func(left, right int) bool {
		if selected[left].SourceField != selected[right].SourceField {
			return selected[left].SourceField < selected[right].SourceField
		}
		if selected[left].ByteStart != selected[right].ByteStart {
			return selected[left].ByteStart < selected[right].ByteStart
		}
		return selected[left].RuleKey < selected[right].RuleKey
	})
	return selected
}
