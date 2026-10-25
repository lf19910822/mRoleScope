package jdanalysis

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// AISemanticProjection is the transient, normalized semantic result produced
// after schema, reference, policy, and remote-location validation. It carries
// no raw model response and must not cross a logging or status-output seam.
type AISemanticProjection struct {
	CandidateMentions []AICandidateMentionProjection
	Experience        AIExperienceProjection
	MinimumEducation  AIMinimumEducationProjection
	RequirementGroups []AIRequirementGroupProjection
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
	if input.validatorVersion == AIValidatorVersionV2 {
		return input.mapValidatedResponseV2(response), nil
	}
	if input.validatorVersion != "" && input.validatorVersion != AIValidatorVersionV1 {
		return AISemanticProjection{}, semanticPolicyError(fmt.Errorf("validator version is not supported"))
	}
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
	candidates, err := input.mapCandidateMentions(response.CandidateMentions)
	if err != nil {
		return AISemanticProjection{}, semanticPolicyError(err)
	}
	return AISemanticProjection{
		CandidateMentions: candidates,
		Experience:        experience,
		MinimumEducation:  education,
		RequirementGroups: groups,
	}, nil
}

// mapValidatedResponseV2 makes model defects local to their semantic field.
// The response has already passed the non-negotiable JSON, request identity,
// and opaque-reference checks. Consequently a malformed business conclusion
// must not discard independently verified conclusions from the same JD.
func (input AIModelInput) mapValidatedResponseV2(response AISemanticResponse) AISemanticProjection {
	return AISemanticProjection{
		CandidateMentions: input.mapCandidateMentionsV2(response.CandidateMentions),
		Experience:        input.mapExperienceV2(response.Experience),
		MinimumEducation:  input.mapMinimumEducationV2(response.MinimumEducation),
		RequirementGroups: input.mapRequirementGroupsV2(response.TechnologyRequirementGroups),
	}
}

func (input AIModelInput) mapCandidateMentions(candidates []AICandidateMentionResponse) ([]AICandidateMentionProjection, error) {
	projections := make([]AICandidateMentionProjection, 0, len(candidates))
	for _, candidate := range candidates {
		evidence, err := input.LocateCandidate(candidate.CandidateText, candidate.SpanID)
		if err != nil {
			return nil, err
		}
		projections = append(projections, AICandidateMentionProjection{
			CandidateText: candidate.CandidateText,
			Evidence:      evidence,
			Necessity:     candidate.Necessity,
			ReviewTier:    forcedRequiredReviewTier(),
			Role:          candidate.Role,
		})
	}
	return projections, nil
}

// mapCandidateMentionsV2 treats a non-locatable candidate as a defect in that
// one model claim. Its supplied span ID has already been checked globally;
// an exact-text mismatch is not a reason to fail the rest of a JD.
func (input AIModelInput) mapCandidateMentionsV2(candidates []AICandidateMentionResponse) []AICandidateMentionProjection {
	projections := make([]AICandidateMentionProjection, 0, len(candidates))
	for _, candidate := range candidates {
		if utf8.RuneCountInString(candidate.CandidateText) > 256 {
			continue
		}
		evidence, err := input.LocateCandidate(candidate.CandidateText, candidate.SpanID)
		if err != nil {
			continue
		}
		projections = append(projections, AICandidateMentionProjection{
			CandidateText: candidate.CandidateText,
			Evidence:      evidence,
			Necessity:     candidate.Necessity,
			ReviewTier:    forcedRequiredReviewTier(),
			Role:          candidate.Role,
		})
	}
	return projections
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
		subjectKind, subjectKey, err := input.mapExperienceScope(item.ScopeType, item.ScopeKey, item.EvidenceSpanIDs)
		if err != nil {
			return AIExperienceProjection{}, err
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
		if conditional.ExperienceScope != "base" || conditional.ExperienceScopeKey != "overall" {
			return AIExperienceProjection{}, fmt.Errorf("conditional experience scope is not representable")
		}
		subjectKind, subjectKey, err := input.mapExperienceScope(conditional.ExperienceScope, conditional.ExperienceScopeKey, conditional.EvidenceSpanIDs)
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
		if response.BaseYear == nil || len(evidence) == 0 || maximumYears == 0 || *response.BaseYear != maximumYears {
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

// mapExperienceV2 preserves every independently evidence-bound experience
// item it can represent. The summary becomes uncertain when the model's
// cross-field shape is contradictory, rather than turning a local omission
// into a failed_safely JD.
func (input AIModelInput) mapExperienceV2(response AIExperienceResponse) AIExperienceProjection {
	projection := AIExperienceProjection{
		ReviewTier: forcedRequiredReviewTier(),
		Status:     "uncertain",
	}

	evidence, evidenceErr := input.ResolveSpanIDs(response.EvidenceSpanIDs)
	if evidenceErr == nil {
		projection.Evidence = evidence
	}

	if response.Status != "configured" {
		if response.Status == "not_configured" && response.BaseYear == nil && len(response.Items) == 0 && len(response.ConditionalQualifications) == 0 && len(response.EvidenceSpanIDs) == 0 {
			projection.Status = "not_configured"
			projection.ReviewTier = response.ReviewRecommendation
		}
		return projection
	}

	maximumYears := 0
	for _, item := range response.Items {
		itemEvidence, err := input.ResolveSpanIDs(item.EvidenceSpanIDs)
		if err != nil || len(itemEvidence) == 0 {
			continue
		}
		subjectKind, subjectKey, err := input.mapExperienceScope(item.ScopeType, item.ScopeKey, item.EvidenceSpanIDs)
		if err != nil {
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
		if err != nil || len(conditionalEvidence) == 0 {
			continue
		}
		educationLevel, _, exists := normalizedEducationLevel(conditional.WhenEducation)
		if !exists || conditional.ExperienceScope != "base" || conditional.ExperienceScopeKey != "overall" {
			continue
		}
		projection.ConditionalQualifications = append(projection.ConditionalQualifications, AIConditionalQualificationProjection{
			EducationLevel: educationLevel,
			Evidence:       conditionalEvidence,
			MinimumYears:   conditional.MinYears,
			ReviewTier:     forcedRequiredReviewTier(),
			SubjectKey:     "",
			SubjectKind:    "overall",
		})
		if conditional.MinYears > maximumYears {
			maximumYears = conditional.MinYears
		}
	}

	if len(projection.Evidence) == 0 {
		projection.Evidence = evidenceFromExperienceProjection(projection)
	}
	if response.BaseYear == nil || len(projection.Evidence) == 0 {
		return projection
	}
	if len(projection.Items) == 0 {
		// A cited aggregate year is useful even when the model omitted the
		// redundant base/overall item required by the persistence shape.
		projection.Items = append(projection.Items, AIExperienceItemProjection{
			Evidence:     projection.Evidence,
			MinimumYears: *response.BaseYear,
			ReviewTier:   forcedRequiredReviewTier(),
			SubjectKind:  "overall",
		})
		maximumYears = *response.BaseYear
	}
	if maximumYears != *response.BaseYear {
		return projection
	}
	baseYear := *response.BaseYear
	projection.BaseYear = &baseYear
	projection.Status = "configured"
	if allExperienceFieldsWereRepresentable(response, projection) {
		projection.ReviewTier = response.ReviewRecommendation
	}
	return projection
}

func evidenceFromExperienceProjection(projection AIExperienceProjection) []AISemanticEvidence {
	evidence := make([]AISemanticEvidence, 0)
	seen := make(map[string]struct{})
	appendEvidence := func(values []AISemanticEvidence) {
		for _, value := range values {
			if _, exists := seen[value.SpanID]; exists {
				continue
			}
			seen[value.SpanID] = struct{}{}
			evidence = append(evidence, value)
		}
	}
	for _, item := range projection.Items {
		appendEvidence(item.Evidence)
	}
	for _, conditional := range projection.ConditionalQualifications {
		appendEvidence(conditional.Evidence)
	}
	return evidence
}

func allExperienceFieldsWereRepresentable(response AIExperienceResponse, projection AIExperienceProjection) bool {
	if response.BaseYear == nil || len(response.EvidenceSpanIDs) == 0 || len(response.Items) != len(projection.Items) || len(response.ConditionalQualifications) != len(projection.ConditionalQualifications) {
		return false
	}
	return true
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

// mapMinimumEducationV2 regards level_code as redundant transport data. A
// supported level remains useful when its numeric mirror is wrong, but it is
// always escalated for review. Missing level/evidence or contradictory status
// becomes an uncertain field rather than a whole-response rejection.
func (input AIModelInput) mapMinimumEducationV2(response AIMinimumEducationResponse) AIMinimumEducationProjection {
	projection := AIMinimumEducationProjection{
		ReviewTier: forcedRequiredReviewTier(),
		Status:     "uncertain",
	}
	evidence, err := input.ResolveSpanIDs(response.EvidenceSpanIDs)
	if err == nil {
		projection.Evidence = evidence
	}
	switch response.Status {
	case "not_configured":
		if response.Level == nil && response.LevelCode == nil && len(response.EvidenceSpanIDs) == 0 {
			projection.Status = "not_configured"
			projection.ReviewTier = response.ReviewRecommendation
		}
		return projection
	case "uncertain":
		return projection
	case "configured":
		if response.Level == nil || len(projection.Evidence) == 0 {
			return projection
		}
		level, expectedCode, exists := normalizedEducationLevel(*response.Level)
		if !exists {
			return projection
		}
		projection.Level = level
		projection.Status = "configured"
		if response.LevelCode != nil && *response.LevelCode == expectedCode {
			projection.ReviewTier = response.ReviewRecommendation
		}
		return projection
	default:
		return projection
	}
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
		projections = append(projections, projection)
	}
	return projections, nil
}

// mapRequirementGroupsV2 keeps evidence-bound options and unambiguous groups
// while converting an incomplete qualification relationship into a visible,
// non-candidate unspecified group. This avoids a fabricated hard filter and
// lets a reviewer see that the relationship needs attention.
func (input AIModelInput) mapRequirementGroupsV2(groups []AITechnologyRequirementGroupResponse) []AIRequirementGroupProjection {
	projections := make([]AIRequirementGroupProjection, 0, len(groups))
	seenLocalIDs := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if _, duplicate := seenLocalIDs[group.LocalID]; duplicate {
			continue
		}
		seenLocalIDs[group.LocalID] = struct{}{}
		evidence, err := input.ResolveSpanIDs(group.EvidenceSpanIDs)
		if err != nil || len(evidence) == 0 {
			continue
		}
		options, deduplicated := input.mapRequirementOptionsV2(group.Options)
		if len(options) == 0 {
			continue
		}
		projection := AIRequirementGroupProjection{
			CandidateRequirement: false,
			Evidence:             evidence,
			LocalID:              group.LocalID,
			Necessity:            group.Necessity,
			Options:              options,
			ReviewTier:           group.ReviewRecommendation,
			Role:                 group.Role,
		}
		if group.Role == "qualification" && group.Operator != nil && *group.Operator == "at_least" && group.MinSatisfied != nil && *group.MinSatisfied >= 1 && *group.MinSatisfied <= len(options) {
			projection.CandidateRequirement = true
			projection.Operator = *group.Operator
			projection.MinSatisfied = *group.MinSatisfied
		} else if group.Role != "responsibility" && group.Role != "mention" && group.Role != "unspecified" ||
			(group.Role != "qualification" && (group.Operator != nil || group.MinSatisfied != nil)) ||
			group.Role == "qualification" {
			projection.Necessity = "unspecified"
			projection.ReviewTier = forcedRequiredReviewTier()
			projection.Role = "unspecified"
		}
		if projection.Role == "unspecified" || projection.Necessity == "unspecified" {
			projection.CandidateRequirement = false
			projection.Operator = ""
			projection.MinSatisfied = 0
			projection.ReviewTier = forcedRequiredReviewTier()
		}
		if deduplicated {
			projection.ReviewTier = forcedRequiredReviewTier()
		}
		projections = append(projections, projection)
	}
	return projections
}

func (input AIModelInput) mapRequirementOptionsV2(options []AITechnologyRequirementOptionResponse) ([]AIRequirementOptionProjection, bool) {
	projections := make([]AIRequirementOptionProjection, 0, len(options))
	byTechnologyKey := make(map[string]int, len(options))
	deduplicated := false
	for _, option := range options {
		if len(option.MentionEvidenceIDs) == 0 {
			continue
		}
		valid := true
		for _, mentionEvidenceID := range option.MentionEvidenceIDs {
			reference, exists := input.responseContract.knownMentionReferences[mentionEvidenceID]
			if !exists || reference.technologyKey != option.TechnologyKey {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		if index, exists := byTechnologyKey[option.TechnologyKey]; exists {
			deduplicated = true
			projections[index].MentionEvidenceIDs = appendUniqueMentionEvidenceIDs(projections[index].MentionEvidenceIDs, option.MentionEvidenceIDs)
			if !sameProficiency(projections[index].Proficiency, option.Proficiency) {
				projections[index].Proficiency = nil
			}
			continue
		}
		byTechnologyKey[option.TechnologyKey] = len(projections)
		projections = append(projections, AIRequirementOptionProjection{
			MentionEvidenceIDs: append([]string(nil), option.MentionEvidenceIDs...),
			Proficiency:        normalizedProficiency(option.Proficiency),
			TechnologyKey:      option.TechnologyKey,
		})
	}
	return projections, deduplicated
}

func appendUniqueMentionEvidenceIDs(current, additional []string) []string {
	seen := make(map[string]struct{}, len(current)+len(additional))
	for _, evidenceID := range current {
		seen[evidenceID] = struct{}{}
	}
	for _, evidenceID := range additional {
		if _, exists := seen[evidenceID]; exists {
			continue
		}
		seen[evidenceID] = struct{}{}
		current = append(current, evidenceID)
	}
	return current
}

func sameProficiency(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (input AIModelInput) mapExperienceScope(scopeType, scopeKey string, evidenceSpanIDs []string) (string, string, error) {
	switch scopeType {
	case "base":
		return "overall", "", nil
	case "technology":
		return "technology", scopeKey, nil
	case "domain":
		if err := input.validateEvidenceBoundDomainScope(scopeKey, evidenceSpanIDs); err != nil {
			return "", "", err
		}
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
