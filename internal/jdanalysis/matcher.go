package jdanalysis

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
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
	SourceField         string
	SourceTechnologyKey string
	SourceCanonicalName string
	TechnologyKey       string
	CanonicalName       string
	TechnologyKind      string
	EmissionRelation    string
	RuleKey             string
	MatchedAlias        string
	ByteStart           int
	ByteEnd             int
	OverlapPriority     int
}

// Matcher contains only rules that are explicitly enabled and approved. It
// has no candidate heuristics, fuzzy matching, source-tag input, or implicit
// ecosystem expansion.
type Matcher struct {
	rules      []DictionaryRule
	exclusions map[string][]DictionaryExclusionPattern
}

func NewMatcher(rules []DictionaryRule) (Matcher, error) {
	return NewMatcherWithExclusions(rules, nil)
}

func NewMatcherWithExclusions(rules []DictionaryRule, exclusions []DictionaryExclusionPattern) (Matcher, error) {
	executable := make([]DictionaryRule, 0, len(rules))
	rulesByKey := make(map[string]DictionaryRule, len(rules))
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
		rulesByKey[rule.RuleKey] = rule
	}
	exclusionMap := make(map[string][]DictionaryExclusionPattern)
	for _, exclusion := range exclusions {
		if err := exclusion.Validate(); err != nil {
			return Matcher{}, err
		}
		if _, ok := rulesByKey[exclusion.SourceRuleKey]; !ok {
			return Matcher{}, fmt.Errorf("exclusion references an unavailable source rule")
		}
		if exclusion.State != RuleEnabled {
			continue
		}
		if exclusion.CaseMode == CaseInsensitive && !isASCII(exclusion.Pattern) {
			return Matcher{}, fmt.Errorf("case-insensitive exclusion patterns must be ASCII")
		}
		exclusionMap[exclusion.SourceRuleKey] = append(exclusionMap[exclusion.SourceRuleKey], exclusion)
	}
	return Matcher{rules: executable, exclusions: exclusionMap}, nil
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
			if !ruleMatchesAt(rule, normalizedText, start, end) || !hasBoundary(rule.BoundaryMode, normalizedText, start, end) || matcher.isExcluded(rule.RuleKey, normalizedText, start, end) {
				continue
			}
			candidates = append(candidates, ByteMatch{
				SourceField: sourceField, SourceTechnologyKey: rule.TechnologyKey, SourceCanonicalName: rule.CanonicalName,
				TechnologyKey: rule.TechnologyKey, CanonicalName: rule.CanonicalName, TechnologyKind: rule.TechnologyKind,
				EmissionRelation: "self",
				RuleKey:          rule.RuleKey, MatchedAlias: rule.Alias, ByteStart: start, ByteEnd: end,
				OverlapPriority: rule.OverlapPriority,
			})
		}
	}
	selected := selectLongestNonOverlapping(candidates)
	expanded := make([]ByteMatch, 0, len(selected))
	for _, sourceMatch := range selected {
		var sourceRule DictionaryRule
		for _, rule := range matcher.rules {
			if rule.RuleKey == sourceMatch.RuleKey {
				sourceRule = rule
				break
			}
		}
		for _, emission := range sourceRule.EffectiveEmissions() {
			match := sourceMatch
			match.TechnologyKey = emission.TechnologyKey
			match.CanonicalName = emission.CanonicalName
			match.TechnologyKind = emission.TechnologyKind
			match.EmissionRelation = emission.RelationType
			expanded = append(expanded, match)
		}
	}
	return expanded, nil
}

func (matcher Matcher) isExcluded(sourceRuleKey, text string, start, end int) bool {
	for _, exclusion := range matcher.exclusions[sourceRuleKey] {
		if exclusionMatchesAt(exclusion, text, start, end) {
			return true
		}
	}
	return false
}

func exclusionMatchesAt(exclusion DictionaryExclusionPattern, text string, start, end int) bool {
	if start < 0 || end <= start || end > len(text) || !utf8.ValidString(text) {
		return false
	}
	sourceAlias := text[start:end]
	runes := []rune(text)
	startRune := utf8.RuneCountInString(text[:start])
	endRune := utf8.RuneCountInString(text[:end])
	segmentStart, segmentEnd := strongPunctuationSegment(runes, startRune)
	window := exclusion.WindowCodePoints
	if window <= 0 {
		window = 96
	}
	windowStart := maxInt(segmentStart, startRune-window)
	windowEnd := minInt(segmentEnd, endRune+window)
	local := runes[windowStart:windowEnd]
	patternRunes := []rune(exclusion.Pattern)
	if len(patternRunes) == 0 || len(patternRunes) > len(local) {
		return false
	}
	for offset := 0; offset+len(patternRunes) <= len(local); offset++ {
		candidate := string(local[offset : offset+len(patternRunes)])
		if exclusion.CaseMode == CaseSensitive {
			if candidate != exclusion.Pattern {
				continue
			}
		} else if !strings.EqualFold(candidate, exclusion.Pattern) {
			continue
		}
		patternStart := windowStart + offset
		patternEnd := patternStart + len(patternRunes)
		if !hasRuneBoundary(exclusion.BoundaryMode, runes, patternStart, patternEnd) {
			continue
		}
		if !patternContainsSourceAlias(exclusion, sourceAlias) {
			return true
		}
		if patternStart <= startRune && patternEnd >= endRune {
			return true
		}
		if patternStart >= startRune && patternStart < endRune {
			return true
		}
	}
	return false
}

func patternContainsSourceAlias(exclusion DictionaryExclusionPattern, sourceAlias string) bool {
	aliasRunes := []rune(sourceAlias)
	patternRunes := []rune(exclusion.Pattern)
	if len(aliasRunes) == 0 || len(aliasRunes) > len(patternRunes) {
		return false
	}
	for start := 0; start+len(aliasRunes) <= len(patternRunes); start++ {
		candidate := string(patternRunes[start : start+len(aliasRunes)])
		if exclusion.CaseMode == CaseSensitive {
			if candidate == sourceAlias {
				return true
			}
			continue
		}
		if strings.EqualFold(candidate, sourceAlias) {
			return true
		}
	}
	return false
}

func strongPunctuationSegment(runes []rune, at int) (int, int) {
	start := at
	for start > 0 && !isStrongPunctuation(runes[start-1]) {
		start--
	}
	end := at
	for end < len(runes) && !isStrongPunctuation(runes[end]) {
		end++
	}
	return start, end
}

func isStrongPunctuation(value rune) bool {
	switch value {
	case '。', '！', '？', '；', '.', '!', '?', ';':
		return true
	default:
		return false
	}
}

func hasRuneBoundary(mode BoundaryMode, text []rune, start, end int) bool {
	if mode == BoundaryExact {
		return true
	}
	var before, after byte
	if start > 0 && text[start-1] <= 0x7f {
		before = byte(text[start-1])
	}
	if end < len(text) && text[end] <= 0x7f {
		after = byte(text[end])
	}
	return !isTechnicalIdentifierByte(before) && !isTechnicalIdentifierByte(after)
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
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
