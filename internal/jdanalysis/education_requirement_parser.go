package jdanalysis

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const deterministicEducationParserVersionV1 = "deterministic-education-parser-v1"

type EducationLevel string

const (
	EducationLevelAssociateOrLower EducationLevel = "associate_or_lower"
	EducationLevelBachelor         EducationLevel = "bachelor"
	EducationLevelMaster           EducationLevel = "master"
	EducationLevelDoctor           EducationLevel = "doctor"
)

type EducationRequirementStatus string

const (
	EducationRequirementConfigured    EducationRequirementStatus = "configured"
	EducationRequirementNotConfigured EducationRequirementStatus = "not_configured"
	EducationRequirementUncertain     EducationRequirementStatus = "uncertain"
)

type EducationRequirementModality string

const (
	EducationRequirementModalityRequired     EducationRequirementModality = "required"
	EducationRequirementModalityPreferred    EducationRequirementModality = "preferred"
	EducationRequirementModalityUnrestricted EducationRequirementModality = "unrestricted"
)

// EducationParserInput contains only normalized JD body fields. The parser
// never reads a title, source metadata, or an AI projection.
type EducationParserInput struct {
	Description string
	JobRequire  string
	Addition    string
}

type EducationRequirementEvidence struct {
	SourceField    string
	RuleKey        string
	MatchedAlias   string
	CodePointRange CodePointRange
	Modality       EducationRequirementModality
}

type EducationRequirement struct {
	Status          EducationRequirementStatus
	LowerBoundLevel EducationLevel
	UpperBoundLevel EducationLevel
	Evidence        []EducationRequirementEvidence
}

type educationLexiconAsset struct {
	Version string                 `json:"version"`
	Rules   []educationLexiconRule `json:"rules"`
}

type educationLexiconRule struct {
	RuleKey              string         `json:"rule_key"`
	Level                EducationLevel `json:"level"`
	Alias                string         `json:"alias"`
	ASCIICaseInsensitive bool           `json:"ascii_case_insensitive"`
}

type educationMatch struct {
	rule      educationLexiconRule
	byteStart int
	byteEnd   int
}

type educationConstraintDirection string

const (
	educationConstraintLower educationConstraintDirection = "lower"
	educationConstraintUpper educationConstraintDirection = "upper"
)

type educationClause struct {
	text      string
	byteStart int
}

type educationConstraint struct {
	evidence       EducationRequirementEvidence
	level          EducationLevel
	direction      educationConstraintDirection
	clauseIndex    int
	isAlternative  bool
	isHardRequired bool
}

//go:embed education_lexicon_v1.json
var deterministicEducationLexiconAsset []byte

var deterministicEducationLexicon struct {
	once   sync.Once
	asset  educationLexiconAsset
	digest string
	err    error
}

// EducationRequirementParserVersion is the immutable parser implementation
// identity stored with every durable deterministic education result.
func EducationRequirementParserVersion() string {
	return deterministicEducationParserVersionV1
}

// EducationRequirementLexiconVersion identifies the exact embedded lexical
// policy used by the parser.
func EducationRequirementLexiconVersion() string {
	asset, _ := loadDeterministicEducationLexicon()
	return asset.Version
}

// EducationRequirementLexiconDigest returns the SHA-256 digest of the exact
// embedded asset bytes, so a lexical-policy change receives a new identity.
func EducationRequirementLexiconDigest() string {
	_, digest := loadDeterministicEducationLexicon()
	return digest
}

// ParseEducationRequirement creates an evidence-backed deterministic result
// from normalized JD qualification fields. It performs no model call and does
// not read an AI projection.
func ParseEducationRequirement(input EducationParserInput) (EducationRequirement, error) {
	asset, _ := loadDeterministicEducationLexicon()
	if deterministicEducationLexicon.err != nil {
		return EducationRequirement{}, deterministicEducationLexicon.err
	}

	jobConstraints, err := parseEducationField(asset.Rules, SourceFieldJobRequire, input.JobRequire, false)
	if err != nil {
		return EducationRequirement{}, err
	}
	descriptionConstraints, err := parseEducationField(asset.Rules, SourceFieldDescription, input.Description, false)
	if err != nil {
		return EducationRequirement{}, err
	}
	additionConstraints, err := parseEducationField(asset.Rules, SourceFieldAddition, input.Addition, true)
	if err != nil {
		return EducationRequirement{}, err
	}
	constraints := append(jobConstraints, descriptionConstraints...)
	constraints = append(constraints, additionConstraints...)
	result := EducationRequirement{Status: EducationRequirementNotConfigured, Evidence: evidenceFromConstraints(constraints)}

	if hasUnrestrictedEducation(input.JobRequire) || hasUnrestrictedQualificationDescription(input.Description) {
		return result, nil
	}
	lower, lowerConflict := resolveEducationBound(constraints, educationConstraintLower)
	upper, upperConflict := resolveEducationBound(constraints, educationConstraintUpper)
	if lowerConflict || upperConflict || (lower != "" && upper != "" && educationLevelRank(lower) > educationLevelRank(upper)) {
		result.Status = EducationRequirementUncertain
		return result, nil
	}
	if lower != "" || upper != "" {
		result.Status = EducationRequirementConfigured
		result.LowerBoundLevel = lower
		result.UpperBoundLevel = upper
	}
	return result, nil
}

func parseEducationField(rules []educationLexiconRule, sourceField, text string, forcePreferred bool) ([]educationConstraint, error) {
	if text == "" {
		return nil, nil
	}
	clauses := splitEducationClauses(text)
	matches := matchEducationAliases(rules, text)
	constraints := make([]educationConstraint, 0, len(matches))
	for matchIndex, match := range matches {
		clauseIndex := educationClauseIndex(clauses, match)
		clause := clauses[clauseIndex]
		if sourceField == SourceFieldDescription && !isQualificationDescriptionClause(clause.text) {
			continue
		}
		if excludedEducationMatch(sourceField, clause, match) {
			continue
		}
		modality := educationMatchModality(sourceField, forcePreferred, clause, match, matches[matchIndex+1:])
		interval, err := CodePointRangeFromByteOffsets(text, match.byteStart, match.byteEnd)
		if err != nil {
			return nil, fmt.Errorf("convert education evidence coordinate: %w", err)
		}
		constraints = append(constraints, educationConstraint{
			evidence: EducationRequirementEvidence{
				SourceField:    sourceField,
				RuleKey:        match.rule.RuleKey,
				MatchedAlias:   text[match.byteStart:match.byteEnd],
				CodePointRange: interval,
				Modality:       modality,
			},
			level:          match.rule.Level,
			direction:      educationMatchDirection(clause, match, matches[matchIndex+1:]),
			clauseIndex:    clauseIndex,
			isAlternative:  educationMatchIsAlternative(clause, match, matches[matchIndex+1:]),
			isHardRequired: modality == EducationRequirementModalityRequired,
		})
	}
	return constraints, nil
}

func isQualificationDescriptionClause(text string) bool {
	for _, heading := range []string{"任职资格", "任职要求", "职位要求", "岗位要求", "学历要求", "资格要求"} {
		if strings.Contains(text, heading) {
			return true
		}
	}
	return false
}

func hasUnrestrictedQualificationDescription(text string) bool {
	for _, clause := range splitEducationClauses(text) {
		if isQualificationDescriptionClause(clause.text) && hasUnrestrictedEducation(clause.text) {
			return true
		}
	}
	return false
}

func excludedEducationMatch(sourceField string, clause educationClause, match educationMatch) bool {
	if sourceField == SourceFieldJobRequire && isResponsibilityOnlyEducationClause(clause.text) {
		return true
	}
	matchEnd := match.byteEnd - clause.byteStart
	if matchEnd < 0 || matchEnd > len(clause.text) {
		return true
	}
	following := clause.text[matchEnd:]
	if match.rule.Level == EducationLevelDoctor && strings.HasPrefix(following, "后") {
		return true
	}
	return strings.Contains(match.rule.Alias, "研究生") && strings.HasPrefix(following, "项目")
}

func isResponsibilityOnlyEducationClause(text string) bool {
	hasResponsibilityMarker := strings.Contains(text, "岗位职责") || strings.Contains(text, "工作职责") || strings.Contains(text, "职责：")
	return hasResponsibilityMarker && (strings.Contains(text, "负责") || strings.Contains(text, "工作内容") || strings.Contains(text, "日常"))
}

func evidenceFromConstraints(constraints []educationConstraint) []EducationRequirementEvidence {
	evidence := make([]EducationRequirementEvidence, 0, len(constraints))
	for _, constraint := range constraints {
		evidence = append(evidence, constraint.evidence)
	}
	return evidence
}

func resolveEducationBound(constraints []educationConstraint, direction educationConstraintDirection) (EducationLevel, bool) {
	byClause := make(map[int][]educationConstraint)
	clauseOrder := make([]int, 0)
	for _, constraint := range constraints {
		if !constraint.isHardRequired || constraint.direction != direction {
			continue
		}
		if _, exists := byClause[constraint.clauseIndex]; !exists {
			clauseOrder = append(clauseOrder, constraint.clauseIndex)
		}
		byClause[constraint.clauseIndex] = append(byClause[constraint.clauseIndex], constraint)
	}
	if len(clauseOrder) == 0 {
		return "", false
	}
	sort.Ints(clauseOrder)
	var resolved EducationLevel
	for _, clauseIndex := range clauseOrder {
		clauseConstraints := byClause[clauseIndex]
		level, conflict := resolveEducationClauseBound(clauseConstraints, direction)
		if conflict {
			return "", true
		}
		if resolved == "" {
			resolved = level
			continue
		}
		if resolved != level {
			return "", true
		}
	}
	return resolved, false
}

func resolveEducationClauseBound(constraints []educationConstraint, direction educationConstraintDirection) (EducationLevel, bool) {
	if len(constraints) == 1 {
		return constraints[0].level, false
	}
	allAlternatives := true
	for _, constraint := range constraints {
		if !constraint.isAlternative {
			allAlternatives = false
			break
		}
	}
	if !allAlternatives {
		first := constraints[0].level
		for _, constraint := range constraints[1:] {
			if constraint.level != first {
				return "", true
			}
		}
		return first, false
	}
	selected := constraints[0].level
	for _, constraint := range constraints[1:] {
		if direction == educationConstraintLower && educationLevelRank(constraint.level) < educationLevelRank(selected) {
			selected = constraint.level
		}
		if direction == educationConstraintUpper && educationLevelRank(constraint.level) > educationLevelRank(selected) {
			selected = constraint.level
		}
	}
	return selected, false
}

func educationLevelRank(level EducationLevel) int {
	switch level {
	case EducationLevelAssociateOrLower:
		return 1
	case EducationLevelBachelor:
		return 2
	case EducationLevelMaster:
		return 3
	case EducationLevelDoctor:
		return 4
	default:
		return 0
	}
}

func splitEducationClauses(text string) []educationClause {
	clauses := make([]educationClause, 0, 1)
	start := 0
	for index, character := range text {
		if !strings.ContainsRune("；;。\n", character) {
			continue
		}
		if index > start {
			clauses = append(clauses, educationClause{text: text[start:index], byteStart: start})
		}
		start = index + len(string(character))
	}
	if start < len(text) {
		clauses = append(clauses, educationClause{text: text[start:], byteStart: start})
	}
	if len(clauses) == 0 {
		return []educationClause{{text: text}}
	}
	return clauses
}

func educationClauseIndex(clauses []educationClause, match educationMatch) int {
	for index, clause := range clauses {
		clauseEnd := clause.byteStart + len(clause.text)
		if match.byteStart >= clause.byteStart && match.byteEnd <= clauseEnd {
			return index
		}
	}
	return 0
}

func educationMatchModality(sourceField string, forcePreferred bool, clause educationClause, match educationMatch, following []educationMatch) EducationRequirementModality {
	if forcePreferred || sourceField == SourceFieldAddition {
		return EducationRequirementModalityPreferred
	}
	matchStart := match.byteStart - clause.byteStart
	matchEnd := match.byteEnd - clause.byteStart
	if hasLocalEducationPriorityMarker(clause.text, matchStart, matchEnd, following, clause.byteStart) {
		return EducationRequirementModalityPreferred
	}
	return EducationRequirementModalityRequired
}

func hasLocalEducationPriorityMarker(clauseText string, matchStart, matchEnd int, following []educationMatch, clauseByteStart int) bool {
	trailingEnd := len(clauseText)
	if len(following) > 0 && following[0].byteStart >= clauseByteStart && following[0].byteStart < clauseByteStart+len(clauseText) {
		trailingEnd = following[0].byteStart - clauseByteStart
	}
	leadingStart := 0
	for index := len(following) - 1; index >= 0; index-- {
		if following[index].byteEnd <= clauseByteStart+matchStart {
			leadingStart = following[index].byteEnd - clauseByteStart
			break
		}
	}
	return containsEducationPriorityMarker(clauseText[leadingStart:matchStart]) || containsEducationPriorityMarker(clauseText[matchEnd:trailingEnd])
}

func containsEducationPriorityMarker(text string) bool {
	for _, marker := range []string{"优先考虑", "优先", "加分", "更佳"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func educationMatchDirection(clause educationClause, match educationMatch, following []educationMatch) educationConstraintDirection {
	matchStart := match.byteStart - clause.byteStart
	matchEnd := match.byteEnd - clause.byteStart
	trailingEnd := len(clause.text)
	if len(following) > 0 && following[0].byteStart >= clause.byteStart && following[0].byteStart < clause.byteStart+len(clause.text) {
		trailingEnd = following[0].byteStart - clause.byteStart
	}
	nearby := clause.text[maxEducationIndex(0, matchStart-12):matchStart] + clause.text[matchEnd:trailingEnd]
	for _, marker := range []string{"及以下", "不高于", "以下"} {
		if strings.Contains(nearby, marker) {
			return educationConstraintUpper
		}
	}
	return educationConstraintLower
}

func educationMatchIsAlternative(clause educationClause, match educationMatch, following []educationMatch) bool {
	_ = match
	_ = following
	return strings.Contains(clause.text, "或") || strings.Contains(clause.text, "/")
}

func hasUnrestrictedEducation(text string) bool {
	for _, expression := range []string{"学历不限", "不限学历", "不限制学历"} {
		if strings.Contains(text, expression) {
			return true
		}
	}
	return false
}

func maxEducationIndex(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func loadDeterministicEducationLexicon() (educationLexiconAsset, string) {
	deterministicEducationLexicon.once.Do(func() {
		asset := educationLexiconAsset{}
		if err := json.Unmarshal(deterministicEducationLexiconAsset, &asset); err != nil {
			deterministicEducationLexicon.err = fmt.Errorf("decode deterministic education lexicon: %w", err)
			return
		}
		if err := validateEducationLexicon(asset); err != nil {
			deterministicEducationLexicon.err = err
			return
		}
		digest := sha256.Sum256(deterministicEducationLexiconAsset)
		deterministicEducationLexicon.asset = asset
		deterministicEducationLexicon.digest = hex.EncodeToString(digest[:])
	})
	return deterministicEducationLexicon.asset, deterministicEducationLexicon.digest
}

func validateEducationLexicon(asset educationLexiconAsset) error {
	if asset.Version == "" || len(asset.Rules) == 0 {
		return fmt.Errorf("deterministic education lexicon is incomplete")
	}
	ruleKeys := make(map[string]struct{}, len(asset.Rules))
	for _, rule := range asset.Rules {
		if rule.RuleKey == "" || rule.Alias == "" || !validEducationLevel(rule.Level) {
			return fmt.Errorf("deterministic education lexicon has invalid rule")
		}
		if _, exists := ruleKeys[rule.RuleKey]; exists {
			return fmt.Errorf("deterministic education lexicon repeats a rule key")
		}
		if rule.ASCIICaseInsensitive && !isASCII(rule.Alias) {
			return fmt.Errorf("deterministic education lexicon has non-ASCII case-insensitive rule")
		}
		ruleKeys[rule.RuleKey] = struct{}{}
	}
	return nil
}

func validEducationLevel(level EducationLevel) bool {
	return level == EducationLevelAssociateOrLower || level == EducationLevelBachelor || level == EducationLevelMaster || level == EducationLevelDoctor
}

func matchEducationAliases(rules []educationLexiconRule, text string) []educationMatch {
	candidates := make([]educationMatch, 0)
	for _, rule := range rules {
		for byteStart := 0; byteStart+len(rule.Alias) <= len(text); byteStart++ {
			byteEnd := byteStart + len(rule.Alias)
			if !educationRuleMatchesAt(rule, text, byteStart, byteEnd) {
				continue
			}
			candidates = append(candidates, educationMatch{rule: rule, byteStart: byteStart, byteEnd: byteEnd})
		}
	}
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].byteStart != candidates[right].byteStart {
			return candidates[left].byteStart < candidates[right].byteStart
		}
		leftLength := candidates[left].byteEnd - candidates[left].byteStart
		rightLength := candidates[right].byteEnd - candidates[right].byteStart
		if leftLength != rightLength {
			return leftLength > rightLength
		}
		return candidates[left].rule.RuleKey < candidates[right].rule.RuleKey
	})
	selected := make([]educationMatch, 0, len(candidates))
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

func educationRuleMatchesAt(rule educationLexiconRule, text string, byteStart, byteEnd int) bool {
	segment := text[byteStart:byteEnd]
	if rule.ASCIICaseInsensitive {
		if !isASCII(segment) || !strings.EqualFold(segment, rule.Alias) {
			return false
		}
		return !isEducationASCIIIdentifierByte(byteBefore(text, byteStart)) && !isEducationASCIIIdentifierByte(byteAfter(text, byteEnd))
	}
	return segment == rule.Alias
}

func isEducationASCIIIdentifierByte(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9') || value == '_'
}
