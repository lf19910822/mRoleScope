package jdanalysis

import (
	"fmt"
	"strings"
)

// AISemanticProjection is the transient, normalized semantic result produced
// after schema, reference, policy, and remote-location validation. It carries
// no raw model response and must not cross a logging or status-output seam.
type AISemanticProjection struct {
	CandidateMentions            []AICandidateMentionProjection
	Experience                   AIExperienceProjection
	MinimumEducation             AIMinimumEducationProjection
	PendingUnresolvedSourceTerms []AIPendingUnresolvedSourceTermProjection
	RequirementGroups            []AIRequirementGroupProjection
}

// AICandidateMentionProjection remains transient until the existing remote
// dictionary-candidate governance path revalidates and persists it.
type AICandidateMentionProjection struct {
	CandidateText string
	Evidence      AISemanticEvidence
	Necessity     string
	ReviewTier    string
	Role          string
}

// AIPendingUnresolvedSourceTermProjection retains the exact source term only
// when the model suggested a different normalization. It must never become a
// dictionary candidate, technology identity, matching input, or requirement
// option without separate human governance.
type AIPendingUnresolvedSourceTermProjection struct {
	Evidence   AISemanticEvidence
	SourceTerm string
}

// AIConditionalQualificationProjection preserves one explicitly stated
// education/experience branch without inventing unmentioned variants.
type AIConditionalQualificationProjection struct {
	EducationLevel string
	Evidence       []AISemanticEvidence
	MinimumYears   int
	ReviewTier     string
	SubjectKey     string
	SubjectKind    string
}

// AIExperienceItemProjection is one validated explicit experience lower bound.
type AIExperienceItemProjection struct {
	Evidence     []AISemanticEvidence
	MinimumYears int
	ReviewTier   string
	SubjectKey   string
	SubjectKind  string
}

// AIExperienceProjection retains business status separately from execution
// state. A nil BaseYear is never converted to a zero-year requirement.
type AIExperienceProjection struct {
	BaseYear                  *int
	ConditionalQualifications []AIConditionalQualificationProjection
	Evidence                  []AISemanticEvidence
	Items                     []AIExperienceItemProjection
	ReviewTier                string
	Status                    string
}

// AIMinimumEducationProjection is the normalized single minimum-education
// conclusion. Level is empty only for not_configured or uncertain statuses.
type AIMinimumEducationProjection struct {
	Evidence   []AISemanticEvidence
	Level      string
	ReviewTier string
	Status     string
}

// AIRequirementGroupProjection preserves the model's explicit relationship.
// CandidateRequirement is false for responsibility, mention, and ambiguous
// groups, so those groups cannot become hard candidate constraints.
type AIRequirementGroupProjection struct {
	CandidateRequirement bool
	Evidence             []AISemanticEvidence
	LocalID              string
	MinSatisfied         int
	Necessity            string
	Operator             string
	Options              []AIRequirementOptionProjection
	ReviewTier           string
	Role                 string
}

// AIRequirementOptionProjection refers only to verified deterministic
// technology/evidence identifiers; it never introduces a candidate string as
// an active technology identity.
type AIRequirementOptionProjection struct {
	MentionEvidenceIDs []string
	Proficiency        *string
	TechnologyKey      string
}

// ValidateAndMapResponse is the semantic-validation seam used after a model
// adapter returns bytes. It drops raw bytes on every failure path.
func (input AIModelInput) ValidateAndMapResponse(raw []byte) (AISemanticProjection, error) {
	response, err := input.ValidateResponse(raw)
	if err != nil {
		return AISemanticProjection{}, err
	}
	return input.MapValidatedResponse(response)
}

// MapValidatedResponse converts a response already accepted by the strict
// contract into conservative, persistence-ready semantic values.
func (input AIModelInput) MapValidatedResponse(response AISemanticResponse) (AISemanticProjection, error) {
	experience, err := input.mapExperience(response.Experience)
	if err != nil {
		return AISemanticProjection{}, semanticPolicyError(err)
	}
	education, err := input.mapMinimumEducation(response.MinimumEducation)
	if err != nil {
		return AISemanticProjection{}, semanticPolicyError(err)
	}
	groups, err := input.mapRequirementGroups(response.TechnologyRequirementGroups)
	if err != nil {
		return AISemanticProjection{}, semanticPolicyError(err)
	}
	candidates, unresolvedSourceTerms := input.mapCandidateMentions(response.CandidateMentions)
	return AISemanticProjection{
		CandidateMentions:            candidates,
		Experience:                   experience,
		MinimumEducation:             education,
		PendingUnresolvedSourceTerms: unresolvedSourceTerms,
		RequirementGroups:            groups,
	}, nil
}

func (input AIModelInput) mapCandidateMentions(candidates []AICandidateMentionResponse) ([]AICandidateMentionProjection, []AIPendingUnresolvedSourceTermProjection) {
	const maximumModelRankedCandidates = 3
	projections := make([]AICandidateMentionProjection, 0, min(len(candidates), maximumModelRankedCandidates))
	unresolvedSourceTerms := make([]AIPendingUnresolvedSourceTermProjection, 0, maximumModelRankedCandidates)
	for rank, candidate := range candidates {
		if rank >= maximumModelRankedCandidates {
			break
		}
		if candidate.Role != "qualification" || (candidate.Necessity != "required" && candidate.Necessity != "preferred") {
			continue
		}
		sourceTerm := candidate.CandidateText
		if candidate.SourceTerm != "" {
			sourceTerm = candidate.SourceTerm
		}
		if !isCandidateTechnicalNameOrAbbreviation(sourceTerm) {
			continue
		}
		evidence, err := input.LocateCandidate(sourceTerm, candidate.SpanID)
		if err != nil {
			// Candidate governance is advisory. A malformed, non-source-exact,
			// or otherwise unrecoverable candidate cannot invalidate independent
			// experience, education, or verified technology conclusions.
			continue
		}
		if candidate.SourceTerm != "" && candidate.CandidateText != sourceTerm {
			// A v3 normalization is not a source-exact candidate. Preserve the
			// original term as a distinct review artifact, but never allow the
			// suggested normalized value to enter dictionary governance.
			unresolvedSourceTerms = append(unresolvedSourceTerms, AIPendingUnresolvedSourceTermProjection{
				Evidence:   evidence,
				SourceTerm: sourceTerm,
			})
			continue
		}
		projections = append(projections, AICandidateMentionProjection{
			CandidateText: sourceTerm,
			Evidence:      evidence,
			Necessity:     candidate.Necessity,
			ReviewTier:    forcedRequiredReviewTier(),
			Role:          candidate.Role,
		})
	}
	return projections, unresolvedSourceTerms
}

// isCandidateTechnicalNameOrAbbreviation deliberately uses a narrow,
// explainable first-release boundary: candidates must contain an ASCII letter
// or digit, which admits technical proper names/abbreviations such as DFMEA,
// LLM, FreeRTOS, UE4/5, and ToolX while excluding ordinary Chinese concepts.
// The model's top-three ordering and qualification role are enforced by the
// caller; dictionary approval remains a separate human workflow.
func isCandidateTechnicalNameOrAbbreviation(candidate string) bool {
	for _, value := range candidate {
		if value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9' {
			return true
		}
	}
	return false
}

func (input AIModelInput) mapExperience(response AIExperienceResponse) (AIExperienceProjection, error) {
	projection := AIExperienceProjection{Status: response.Status}
	evidence, err := input.ResolveSpanIDs(response.EvidenceSpanIDs)
	if err != nil {
		return AIExperienceProjection{}, err
	}
	projection.Evidence = evidence
	projection.ReviewTier = response.ReviewRecommendation

	maximumYears := 0
	for _, item := range response.Items {
		itemEvidence, err := input.ResolveSpanIDs(item.EvidenceSpanIDs)
		if err != nil {
			return AIExperienceProjection{}, err
		}
		subjectKind, subjectKey, err := mapExperienceScope(item.ScopeType, item.ScopeKey)
		if err != nil {
			return AIExperienceProjection{}, err
		}
		if subjectKind == "domain" && !input.isSingleSourceExactExperienceDomain(subjectKey, item.EvidenceSpanIDs) {
			// The first-release domain structure has no operator. Omitting only
			// this unrepresentable item preserves the independently evidenced
			// overall experience requirement without turning alternatives into
			// simultaneous hard requirements.
			continue
		}
		projection.Items = append(projection.Items, AIExperienceItemProjection{
			Evidence:     itemEvidence,
			MinimumYears: item.MinYears,
			ReviewTier:   item.ReviewRecommendation,
			SubjectKey:   subjectKey,
			SubjectKind:  subjectKind,
		})
		if item.MinYears > maximumYears {
			maximumYears = item.MinYears
		}
	}
	for _, conditional := range response.ConditionalQualifications {
		conditionalEvidence, err := input.ResolveSpanIDs(conditional.EvidenceSpanIDs)
		if err != nil {
			return AIExperienceProjection{}, err
		}
		educationLevel, _, exists := normalizedEducationLevel(conditional.WhenEducation)
		if !exists {
			return AIExperienceProjection{}, fmt.Errorf("conditional education level is invalid")
		}
		subjectKind, subjectKey, err := mapExperienceScope(conditional.ExperienceScope, conditional.ExperienceScopeKey)
		if err != nil {
			return AIExperienceProjection{}, err
		}
		projection.ConditionalQualifications = append(projection.ConditionalQualifications, AIConditionalQualificationProjection{
			EducationLevel: educationLevel,
			Evidence:       conditionalEvidence,
			MinimumYears:   conditional.MinYears,
			ReviewTier:     forcedRequiredReviewTier(),
			SubjectKey:     subjectKey,
			SubjectKind:    subjectKind,
		})
		if conditional.MinYears > maximumYears {
			maximumYears = conditional.MinYears
		}
	}

	switch response.Status {
	case "configured":
		if response.BaseYear == nil || len(evidence) == 0 || (maximumYears != 0 && *response.BaseYear != maximumYears) {
			return AIExperienceProjection{}, fmt.Errorf("configured experience is incomplete")
		}
		projection.BaseYear = response.BaseYear
	case "not_configured":
		if response.BaseYear != nil || len(response.Items) != 0 || len(response.ConditionalQualifications) != 0 {
			return AIExperienceProjection{}, fmt.Errorf("not configured experience contains a business value")
		}
	case "uncertain":
		if response.BaseYear != nil || len(response.Items) != 0 || len(response.ConditionalQualifications) != 0 {
			return AIExperienceProjection{}, fmt.Errorf("uncertain experience contains a business value")
		}
		projection.ReviewTier = forcedRequiredReviewTier()
	default:
		return AIExperienceProjection{}, fmt.Errorf("experience status is invalid")
	}
	return projection, nil
}

func (input AIModelInput) mapMinimumEducation(response AIMinimumEducationResponse) (AIMinimumEducationProjection, error) {
	evidence, err := input.ResolveSpanIDs(response.EvidenceSpanIDs)
	if err != nil {
		return AIMinimumEducationProjection{}, err
	}
	projection := AIMinimumEducationProjection{
		Evidence:   evidence,
		ReviewTier: response.ReviewRecommendation,
		Status:     response.Status,
	}
	switch response.Status {
	case "configured":
		if response.Level == nil || response.LevelCode == nil || len(evidence) == 0 {
			return AIMinimumEducationProjection{}, fmt.Errorf("configured education is incomplete")
		}
		level, expectedCode, exists := normalizedEducationLevel(*response.Level)
		if !exists || expectedCode != *response.LevelCode {
			return AIMinimumEducationProjection{}, fmt.Errorf("education level and code are inconsistent")
		}
		projection.Level = level
	case "not_configured":
		if response.Level != nil || response.LevelCode != nil {
			return AIMinimumEducationProjection{}, fmt.Errorf("not configured education contains a business value")
		}
	case "uncertain":
		if response.Level != nil || response.LevelCode != nil {
			return AIMinimumEducationProjection{}, fmt.Errorf("uncertain education contains a business value")
		}
		projection.ReviewTier = forcedRequiredReviewTier()
	default:
		return AIMinimumEducationProjection{}, fmt.Errorf("education status is invalid")
	}
	return projection, nil
}

func (input AIModelInput) mapRequirementGroups(groups []AITechnologyRequirementGroupResponse) ([]AIRequirementGroupProjection, error) {
	projections := make([]AIRequirementGroupProjection, 0, len(groups))
	seenLocalIDs := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if _, duplicate := seenLocalIDs[group.LocalID]; duplicate {
			return nil, fmt.Errorf("requirement group is duplicated")
		}
		seenLocalIDs[group.LocalID] = struct{}{}
		evidence, err := input.ResolveSpanIDs(group.EvidenceSpanIDs)
		if err != nil || len(evidence) == 0 {
			return nil, fmt.Errorf("requirement group lacks evidence")
		}
		projection := AIRequirementGroupProjection{
			CandidateRequirement: group.Role == "qualification",
			Evidence:             evidence,
			LocalID:              group.LocalID,
			Necessity:            group.Necessity,
			ReviewTier:           group.ReviewRecommendation,
			Role:                 group.Role,
		}
		if group.Operator != nil {
			projection.Operator = *group.Operator
		}
		if group.MinSatisfied != nil {
			projection.MinSatisfied = *group.MinSatisfied
		}
		if group.Role == "qualification" && (projection.Operator != "at_least" || projection.MinSatisfied < 1 || projection.MinSatisfied > len(group.Options)) {
			return nil, fmt.Errorf("qualification group is not a valid explicit relation")
		}
		if group.Role == "unspecified" || group.Necessity == "unspecified" {
			projection.CandidateRequirement = false
			projection.ReviewTier = forcedRequiredReviewTier()
		}
		for _, option := range group.Options {
			if len(option.MentionEvidenceIDs) == 0 {
				return nil, fmt.Errorf("technology option lacks deterministic evidence")
			}
			for _, mentionEvidenceID := range option.MentionEvidenceIDs {
				reference, exists := input.responseContract.knownMentionReferences[mentionEvidenceID]
				if !exists || reference.technologyKey != option.TechnologyKey {
					return nil, fmt.Errorf("technology option evidence is invalid")
				}
			}
			proficiency := normalizedProficiency(option.Proficiency)
			projection.Options = append(projection.Options, AIRequirementOptionProjection{
				MentionEvidenceIDs: append([]string(nil), option.MentionEvidenceIDs...),
				Proficiency:        proficiency,
				TechnologyKey:      option.TechnologyKey,
			})
		}
		projections = append(projections, input.splitIndependentPositiveQualificationGroup(group, projection)...)
	}
	return projections, nil
}

// isSingleSourceExactExperienceDomain admits only a literal domain term from
// one cited source span. The current storage has no domain relation operator,
// so a composite or alternative source expression must be omitted rather than
// being presented as several simultaneous requirements.
func (input AIModelInput) isSingleSourceExactExperienceDomain(domainKey string, evidenceSpanIDs []string) bool {
	if domainKey == "" || strings.TrimSpace(domainKey) != domainKey || containsCompositeDomainMarker(domainKey) {
		return false
	}
	matchCount := 0
	for _, spanID := range evidenceSpanIDs {
		sentenceText, err := input.sentenceTextForSpan(spanID)
		if err != nil || containsCompositeDomainMarker(sentenceText) {
			return false
		}
		if _, err := input.LocateCandidate(domainKey, spanID); err == nil {
			matchCount++
		}
	}
	return matchCount == 1
}

func containsCompositeDomainMarker(value string) bool {
	if strings.ContainsAny(value, "/／|｜&＋+、") {
		return true
	}
	for _, marker := range []string{"或", "任一", "任选", "之一", "二选一", "至少一种", "以及", "与", "和", "及"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

// sentenceTextForSpan restores only the enclosing sentence in process memory.
// The span generator intentionally splits on commas, but a comma-separated
// domain list can still carry an alternative relation later in that sentence.
// This helper never persists, logs, or returns source text across a boundary.
func (input AIModelInput) sentenceTextForSpan(spanID string) (string, error) {
	span, found := input.spanIndex.Lookup(spanID)
	if !found {
		return "", NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	fieldText, found := input.fieldTextBySource[span.SourceField]
	if !found {
		return "", NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	runes := []rune(fieldText)
	start, end := span.CodePointRange.Start, span.CodePointRange.End
	for start > 0 && !aiSentenceBoundary(runes[start-1]) {
		start--
	}
	for end < len(runes) && !aiSentenceBoundary(runes[end]) {
		end++
	}
	return string(runes[start:end]), nil
}

func aiSentenceBoundary(value rune) bool {
	switch value {
	case '\n', '\r', '。', '！', '？', '；', ';', '!', '?':
		return true
	default:
		return false
	}
}

// splitIndependentPositiveQualificationGroup corrects the specific lossy
// model shape where separately evidenced, positive requirements are placed in
// one multi-option group. Each option must have its own cited source span;
// same-span alternatives remain the model's single explicit at_least group.
func (input AIModelInput) splitIndependentPositiveQualificationGroup(response AITechnologyRequirementGroupResponse, projection AIRequirementGroupProjection) []AIRequirementGroupProjection {
	if projection.Role != "qualification" || projection.Necessity != "required" || projection.Operator != "at_least" || len(projection.Options) < 2 || len(response.EvidenceSpanIDs) != len(projection.Options) {
		return []AIRequirementGroupProjection{projection}
	}
	evidenceSpanIDs := make(map[string]struct{}, len(response.EvidenceSpanIDs))
	for _, spanID := range response.EvidenceSpanIDs {
		if _, duplicate := evidenceSpanIDs[spanID]; duplicate {
			return []AIRequirementGroupProjection{projection}
		}
		evidenceSpanIDs[spanID] = struct{}{}
	}

	optionsBySpanID := make(map[string]AIRequirementOptionProjection, len(projection.Options))
	for _, option := range projection.Options {
		spanID := ""
		for _, mentionEvidenceID := range option.MentionEvidenceIDs {
			reference, exists := input.responseContract.knownMentionReferences[mentionEvidenceID]
			if !exists || reference.technologyKey != option.TechnologyKey {
				return []AIRequirementGroupProjection{projection}
			}
			if spanID == "" {
				spanID = reference.spanID
			} else if spanID != reference.spanID {
				return []AIRequirementGroupProjection{projection}
			}
		}
		if _, found := evidenceSpanIDs[spanID]; !found {
			return []AIRequirementGroupProjection{projection}
		}
		if _, duplicate := optionsBySpanID[spanID]; duplicate {
			return []AIRequirementGroupProjection{projection}
		}
		optionsBySpanID[spanID] = option
	}
	if len(optionsBySpanID) != len(projection.Options) {
		return []AIRequirementGroupProjection{projection}
	}

	split := make([]AIRequirementGroupProjection, 0, len(projection.Options))
	for position, spanID := range response.EvidenceSpanIDs {
		option, found := optionsBySpanID[spanID]
		if !found {
			return []AIRequirementGroupProjection{projection}
		}
		evidence, err := input.ResolveSpanIDs([]string{spanID})
		if err != nil {
			return []AIRequirementGroupProjection{projection}
		}
		part := projection
		part.Evidence = evidence
		part.LocalID = fmt.Sprintf("%s-part-%02d", projection.LocalID, position+1)
		part.MinSatisfied = 1
		part.Options = []AIRequirementOptionProjection{option}
		split = append(split, part)
	}
	return split
}

func mapExperienceScope(scopeType, scopeKey string) (string, string, error) {
	switch scopeType {
	case "base":
		return "overall", "", nil
	case "technology":
		return "technology", scopeKey, nil
	case "domain":
		return "domain", scopeKey, nil
	default:
		return "", "", fmt.Errorf("experience scope is invalid")
	}
}

func normalizedEducationLevel(level string) (string, int, bool) {
	switch level {
	case "junior_college_or_below":
		return "associate_or_lower", 1, true
	case "bachelor":
		return "bachelor", 2, true
	case "master":
		return "master", 3, true
	case "doctor":
		return "doctor", 4, true
	default:
		return "", 0, false
	}
}

func normalizedProficiency(proficiency *string) *string {
	if proficiency == nil {
		return nil
	}
	normalized := strings.ToLower(*proficiency)
	return &normalized
}

func forcedRequiredReviewTier() string {
	return "required_review"
}

func semanticPolicyError(err error) error {
	return NewSafeAIError(AISafeErrorPolicyRejected, err)
}
