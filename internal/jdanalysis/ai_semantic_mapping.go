package jdanalysis

import (
	"fmt"
	"strings"
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
		projections = append(projections, projection)
	}
	return projections, nil
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
