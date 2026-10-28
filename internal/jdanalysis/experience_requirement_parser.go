package jdanalysis

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const deterministicExperienceParserVersionV1 = "deterministic-experience-parser-v1"

type ExperienceRequirementStatus string

const (
	ExperienceRequirementConfigured    ExperienceRequirementStatus = "configured"
	ExperienceRequirementNotConfigured ExperienceRequirementStatus = "not_configured"
	ExperienceRequirementUncertain     ExperienceRequirementStatus = "uncertain"
)

type ExperienceRequirementModality string

const (
	ExperienceRequirementModalityRequired    ExperienceRequirementModality = "required"
	ExperienceRequirementModalityPreferred   ExperienceRequirementModality = "preferred"
	ExperienceRequirementModalityConditional ExperienceRequirementModality = "conditional"
	ExperienceRequirementModalityUncertain   ExperienceRequirementModality = "uncertain"
)

// ExperienceParserInput contains only normalized JD body fields. It does not
// accept source metadata, AI projections, model output, or provider settings.
type ExperienceParserInput struct {
	Description string
	JobRequire  string
	Addition    string
}

type ExperienceRequirementEvidence struct {
	SourceField    string
	RuleKey        string
	MatchedText    string
	MinimumYears   int
	CodePointRange CodePointRange
	Modality       ExperienceRequirementModality
}

type ExperienceRequirement struct {
	Status   ExperienceRequirementStatus
	BaseYear int
	Evidence []ExperienceRequirementEvidence
}

type experienceLexiconAsset struct {
	Version                   string         `json:"version"`
	ChineseNumbers            map[string]int `json:"chinese_numbers"`
	ExperienceAnchors         []string       `json:"experience_anchors"`
	PriorityMarkers           []string       `json:"priority_markers"`
	UnrestrictedMarkers       []string       `json:"unrestricted_markers"`
	EducationConditionMarkers []string       `json:"education_condition_markers"`
}

type experienceCandidate struct {
	byteStart int
	byteEnd   int
	years     int
	ruleKey   string
	priority  int
}

type experienceClause struct {
	text      string
	byteStart int
}

//go:embed experience_lexicon_v1.json
var deterministicExperienceLexiconAsset []byte

var deterministicExperienceLexicon struct {
	once   sync.Once
	asset  experienceLexiconAsset
	digest string
	err    error
}

var (
	experienceArabicRangePattern        = regexp.MustCompile(`([1-9][0-9]?)\s*[-—–~～至]\s*[1-9][0-9]?\s*年(?:以上|及以上)?[^；;。\n]{0,48}?(?:经验|经历|从业)`)
	experienceArabicMinimumPattern      = regexp.MustCompile(`(?:至少|不少于|不低于)\s*([1-9][0-9]?)\s*年(?:以上|及以上)?[^；;。\n]{0,48}?(?:经验|经历|从业)`)
	experienceArabicYearsOrMorePattern  = regexp.MustCompile(`([1-9][0-9]?)\s*年(?:以上|及以上)[^；;。\n]{0,48}?(?:经验|经历|从业)`)
	experienceArabicPlainPattern        = regexp.MustCompile(`([1-9][0-9]?)\s*年[^；;。\n]{0,48}?(?:经验|经历|从业)`)
	experienceChineseYearsOrMorePattern = regexp.MustCompile(`([一二三四五六七八九十两])\s*年(?:以上|及以上)[^；;。\n]{0,48}?(?:经验|经历|从业)`)
)

// ExperienceRequirementParserVersion identifies the immutable parser
// implementation used to produce a formal deterministic experience result.
func ExperienceRequirementParserVersion() string {
	return deterministicExperienceParserVersionV1
}

// ExperienceRequirementLexiconVersion identifies the reviewed lexical policy
// embedded in this binary.
func ExperienceRequirementLexiconVersion() string {
	asset, _ := loadDeterministicExperienceLexicon()
	return asset.Version
}

// ExperienceRequirementLexiconDigest returns the SHA-256 digest of the exact
// embedded experience lexicon bytes.
func ExperienceRequirementLexiconDigest() string {
	_, digest := loadDeterministicExperienceLexicon()
	return digest
}

// ParseExperienceRequirement derives a provider-free formal lower bound from
// the normalized qualification field. Initial parsing is intentionally limited
// to explicit year-plus-candidate-experience forms.
func ParseExperienceRequirement(input ExperienceParserInput) (ExperienceRequirement, error) {
	asset, _ := loadDeterministicExperienceLexicon()
	if deterministicExperienceLexicon.err != nil {
		return ExperienceRequirement{}, deterministicExperienceLexicon.err
	}
	result := ExperienceRequirement{Status: ExperienceRequirementNotConfigured}
	fields := []experienceParserField{
		{sourceField: SourceFieldJobRequire, text: input.JobRequire},
		{sourceField: SourceFieldDescription, text: input.Description, requiresQualificationHeading: true},
		{sourceField: SourceFieldAddition, text: input.Addition, forcePreferred: true},
	}
	hasUncertain := false
	for _, field := range fields {
		candidates := parseExperienceCandidates(field.text, asset)
		clauses := splitExperienceClauses(field.text)
		unrestricted := hasUnrestrictedExperience(field.text, asset)
		for _, candidate := range candidates {
			clause, found := experienceCandidateClause(clauses, candidate)
			if !found || (field.requiresQualificationHeading && !isQualificationExperienceCandidate(field.text, clause, candidate)) {
				continue
			}
			interval, err := CodePointRangeFromByteOffsets(field.text, candidate.byteStart, candidate.byteEnd)
			if err != nil {
				return ExperienceRequirement{}, fmt.Errorf("convert experience evidence coordinate: %w", err)
			}
			modality := experienceCandidateModality(clause, candidate, candidates, field.forcePreferred, asset)
			result.Evidence = append(result.Evidence, ExperienceRequirementEvidence{
				SourceField:    field.sourceField,
				RuleKey:        candidate.ruleKey,
				MatchedText:    field.text[candidate.byteStart:candidate.byteEnd],
				MinimumYears:   candidate.years,
				CodePointRange: interval,
				Modality:       modality,
			})
			if modality == ExperienceRequirementModalityUncertain {
				hasUncertain = true
			}
			if !unrestricted && modality == ExperienceRequirementModalityRequired && candidate.years > result.BaseYear {
				result.BaseYear = candidate.years
			}
		}
	}
	if hasUncertain {
		result.Status = ExperienceRequirementUncertain
		result.BaseYear = 0
	} else if result.BaseYear > 0 {
		result.Status = ExperienceRequirementConfigured
	}
	return result, nil
}

type experienceParserField struct {
	sourceField                  string
	text                         string
	forcePreferred               bool
	requiresQualificationHeading bool
}

func splitExperienceClauses(text string) []experienceClause {
	clauses := make([]experienceClause, 0, 1)
	start := 0
	for index, character := range text {
		if !strings.ContainsRune("；;。\n", character) {
			continue
		}
		if index > start {
			clauses = append(clauses, experienceClause{text: text[start:index], byteStart: start})
		}
		start = index + len(string(character))
	}
	if start < len(text) {
		clauses = append(clauses, experienceClause{text: text[start:], byteStart: start})
	}
	if len(clauses) == 0 {
		return []experienceClause{{text: text}}
	}
	return clauses
}

func experienceCandidateClause(clauses []experienceClause, candidate experienceCandidate) (experienceClause, bool) {
	for _, clause := range clauses {
		clauseEnd := clause.byteStart + len(clause.text)
		if candidate.byteStart >= clause.byteStart && candidate.byteEnd <= clauseEnd {
			return clause, true
		}
	}
	return experienceClause{}, false
}

func experienceCandidateModality(clause experienceClause, candidate experienceCandidate, allCandidates []experienceCandidate, forcePreferred bool, asset experienceLexiconAsset) ExperienceRequirementModality {
	if forcePreferred {
		return ExperienceRequirementModalityPreferred
	}
	atomStart, atomEnd, ok := experienceClauseAtomBounds(clause, candidate)
	if !ok {
		return ExperienceRequirementModalityUncertain
	}
	atom := clause.text[atomStart:atomEnd]
	if containsExperienceMarker(atom, asset.EducationConditionMarkers) {
		return ExperienceRequirementModalityConditional
	}
	if !containsExperienceMarker(atom, asset.PriorityMarkers) {
		return ExperienceRequirementModalityRequired
	}
	matchingCandidates := 0
	for _, other := range allCandidates {
		if other.byteStart >= clause.byteStart+atomStart && other.byteEnd <= clause.byteStart+atomEnd {
			matchingCandidates++
		}
	}
	if matchingCandidates != 1 {
		return ExperienceRequirementModalityUncertain
	}
	return ExperienceRequirementModalityPreferred
}

func experienceClauseAtomBounds(clause experienceClause, candidate experienceCandidate) (int, int, bool) {
	start := candidate.byteStart - clause.byteStart
	end := candidate.byteEnd - clause.byteStart
	if start < 0 || end > len(clause.text) || start >= end {
		return 0, 0, false
	}
	atomStart := 0
	for _, delimiter := range []string{"，", ","} {
		if index := strings.LastIndex(clause.text[:start], delimiter); index >= 0 && index+len(delimiter) > atomStart {
			atomStart = index + len(delimiter)
		}
	}
	atomEnd := len(clause.text)
	for _, delimiter := range []string{"，", ","} {
		if index := strings.Index(clause.text[end:], delimiter); index >= 0 && end+index < atomEnd {
			atomEnd = end + index
		}
	}
	return atomStart, atomEnd, true
}

var qualificationExperienceHeadings = []string{
	"任职资格", "任职要求", "职位要求", "岗位要求", "资格要求",
}

// descriptionNonQualificationHeadings are explicit section labels which end
// a preceding qualification section. The deterministic parser deliberately
// errs on the side of excluding a description candidate when its section is
// not clear, because responsibilities are not candidate requirements.
var descriptionNonQualificationHeadings = []string{
	"岗位职责", "工作职责", "工作内容", "职位描述", "岗位描述", "职责描述",
}

func isQualificationExperienceClause(text string) bool {
	for _, heading := range qualificationExperienceHeadings {
		if strings.Contains(text, heading) {
			return true
		}
	}
	return false
}

// isQualificationExperienceCandidate permits a qualification heading on an
// earlier description line, which is the usual JD layout. A later explicit
// responsibility/description heading ends that scope; arbitrary earlier JD
// text can never make an unrelated description expression eligible.
func isQualificationExperienceCandidate(text string, clause experienceClause, candidate experienceCandidate) bool {
	if isQualificationExperienceClause(clause.text) {
		return true
	}
	if candidate.byteStart <= 0 || candidate.byteStart > len(text) {
		return false
	}
	priorText := text[:candidate.byteStart]
	qualificationOffset := lastExperienceHeadingOffset(priorText, qualificationExperienceHeadings)
	if qualificationOffset < 0 {
		return false
	}
	return lastExperienceHeadingOffset(priorText, descriptionNonQualificationHeadings) < qualificationOffset
}

func lastExperienceHeadingOffset(text string, headings []string) int {
	lastOffset := -1
	for _, heading := range headings {
		if offset := strings.LastIndex(text, heading); offset > lastOffset {
			lastOffset = offset
		}
	}
	return lastOffset
}

func hasUnrestrictedExperience(text string, asset experienceLexiconAsset) bool {
	return containsExperienceMarker(text, asset.UnrestrictedMarkers)
}

func containsExperienceMarker(text string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func parseExperienceCandidates(text string, asset experienceLexiconAsset) []experienceCandidate {
	candidates := make([]experienceCandidate, 0)
	candidates = append(candidates, candidatesFromPattern(experienceArabicRangePattern, text, "experience.arabic.range", 1, func(parts []string) (int, bool) {
		return positiveArabicYear(parts[1])
	})...)
	candidates = append(candidates, candidatesFromPattern(experienceArabicMinimumPattern, text, "experience.arabic.explicit_minimum", 2, func(parts []string) (int, bool) {
		return positiveArabicYear(parts[1])
	})...)
	candidates = append(candidates, candidatesFromPattern(experienceArabicYearsOrMorePattern, text, "experience.arabic.years_or_more", 3, func(parts []string) (int, bool) {
		return positiveArabicYear(parts[1])
	})...)
	candidates = append(candidates, candidatesFromPattern(experienceChineseYearsOrMorePattern, text, "experience.chinese.years_or_more", 4, func(parts []string) (int, bool) {
		years, ok := asset.ChineseNumbers[parts[1]]
		return years, ok && years > 0
	})...)
	candidates = append(candidates, candidatesFromPattern(experienceArabicPlainPattern, text, "experience.arabic.plain", 5, func(parts []string) (int, bool) {
		return positiveArabicYear(parts[1])
	})...)

	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].byteStart != candidates[right].byteStart {
			return candidates[left].byteStart < candidates[right].byteStart
		}
		leftLength := candidates[left].byteEnd - candidates[left].byteStart
		rightLength := candidates[right].byteEnd - candidates[right].byteStart
		if leftLength != rightLength {
			return leftLength > rightLength
		}
		if candidates[left].priority != candidates[right].priority {
			return candidates[left].priority < candidates[right].priority
		}
		return candidates[left].ruleKey < candidates[right].ruleKey
	})
	selected := make([]experienceCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		overlaps := false
		for _, existing := range selected {
			if candidate.byteStart < existing.byteEnd && existing.byteStart < candidate.byteEnd {
				overlaps = true
				break
			}
		}
		if !overlaps {
			selected = append(selected, candidate)
		}
	}
	return selected
}

func candidatesFromPattern(pattern *regexp.Regexp, text, ruleKey string, priority int, decode func([]string) (int, bool)) []experienceCandidate {
	matches := pattern.FindAllStringSubmatchIndex(text, -1)
	candidates := make([]experienceCandidate, 0, len(matches))
	for _, indexes := range matches {
		if len(indexes) < 4 || indexes[0] < 0 || indexes[1] <= indexes[0] || indexes[2] < 0 || indexes[3] <= indexes[2] {
			continue
		}
		parts := make([]string, (len(indexes)-2)/2+1)
		parts[0] = text[indexes[0]:indexes[1]]
		for group := 1; group < len(parts); group++ {
			start, end := indexes[group*2], indexes[group*2+1]
			if start >= 0 && end >= start {
				parts[group] = text[start:end]
			}
		}
		years, ok := decode(parts)
		if !ok {
			continue
		}
		candidates = append(candidates, experienceCandidate{byteStart: indexes[0], byteEnd: indexes[1], years: years, ruleKey: ruleKey, priority: priority})
	}
	return candidates
}

func positiveArabicYear(value string) (int, bool) {
	years, err := strconv.Atoi(value)
	return years, err == nil && years > 0
}

func loadDeterministicExperienceLexicon() (experienceLexiconAsset, string) {
	deterministicExperienceLexicon.once.Do(func() {
		asset := experienceLexiconAsset{}
		if err := json.Unmarshal(deterministicExperienceLexiconAsset, &asset); err != nil {
			deterministicExperienceLexicon.err = fmt.Errorf("decode deterministic experience lexicon: %w", err)
			return
		}
		if err := validateExperienceLexicon(asset); err != nil {
			deterministicExperienceLexicon.err = err
			return
		}
		digest := sha256.Sum256(deterministicExperienceLexiconAsset)
		deterministicExperienceLexicon.asset = asset
		deterministicExperienceLexicon.digest = hex.EncodeToString(digest[:])
	})
	return deterministicExperienceLexicon.asset, deterministicExperienceLexicon.digest
}

func validateExperienceLexicon(asset experienceLexiconAsset) error {
	if asset.Version == "" || len(asset.ChineseNumbers) == 0 || len(asset.ExperienceAnchors) == 0 {
		return fmt.Errorf("deterministic experience lexicon is incomplete")
	}
	for alias, years := range asset.ChineseNumbers {
		if strings.TrimSpace(alias) == "" || years <= 0 {
			return fmt.Errorf("deterministic experience lexicon has invalid Chinese number")
		}
	}
	return nil
}
