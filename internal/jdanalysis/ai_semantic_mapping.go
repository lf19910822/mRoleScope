package jdanalysis

import (
	"fmt"
	"strings"
)

// AISemanticProjection is the transient, normalized semantic result produced
// after hard schema/request validation and local reference sanitation. It
// carries no raw model response and must not cross a logging or status-output
// seam.
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
		return AISemanticProjection{}, semanticMappingError(err)
	}
	education, err := input.mapMinimumEducation(response.MinimumEducation)
	if err != nil {
		return AISemanticProjection{}, semanticMappingError(err)
	}
	groups, err := input.mapRequirementGroups(response.TechnologyRequirementGroups)
	if err != nil {
		return AISemanticProjection{}, semanticMappingError(err)
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
		sourceTerm := candidate.CandidateText
		if candidate.SourceTerm != "" {
			sourceTerm = candidate.SourceTerm
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
			ReviewTier:    candidate.ReviewRecommendation,
			Role:          candidate.Role,
		})
	}
	return projections, unresolvedSourceTerms
}

func (input AIModelInput) mapExperience(response AIExperienceResponse) (AIExperienceProjection, error) {
	projection := AIExperienceProjection{Status: response.Status}
	evidence := input.resolveKnownSpanIDs(response.EvidenceSpanIDs)
	projection.Evidence = evidence
	projection.ReviewTier = response.ReviewRecommendation

	projection.BaseYear = response.BaseYear
	for _, item := range response.Items {
		itemEvidence := input.resolveKnownSpanIDs(item.EvidenceSpanIDs)
		subjectKind, subjectKey, err := mapExperienceScope(item.ScopeType, item.ScopeKey)
		if err != nil {
			return AIExperienceProjection{}, err
		}
		if subjectKind == "technology" && !input.responseContract.hasTechnologyKey(subjectKey) {
			// A formal technology-scoped item requires an existing governed
			// identity. It is not representable otherwise, so drop only this item.
			continue
		}
		if subjectKind == "domain" && utf8RuneCount(subjectKey) > 128 {
			// The normalized relational field has a bounded domain key. A model
			// value outside that shape is omitted locally rather than failing the
			// independently representable response.
			continue
		}
		projection.Items = append(projection.Items, AIExperienceItemProjection{
			Evidence:     itemEvidence,
			MinimumYears: item.MinYears,
			ReviewTier:   item.ReviewRecommendation,
			SubjectKey:   subjectKey,
			SubjectKind:  subjectKind,
		})
	}
	for _, conditional := range response.ConditionalQualifications {
		conditionalEvidence := input.resolveKnownSpanIDs(conditional.EvidenceSpanIDs)
		educationLevel, _, exists := normalizedEducationLevel(conditional.WhenEducation)
		if !exists {
			return AIExperienceProjection{}, fmt.Errorf("conditional education level is invalid")
		}
		subjectKind, subjectKey, err := mapExperienceScope(conditional.ExperienceScope, conditional.ExperienceScopeKey)
		if err != nil {
			return AIExperienceProjection{}, err
		}
		if subjectKind != "overall" {
			// The current branch table does not carry a technology/domain scope.
			// Leave this individually unrepresentable branch out without rejecting
			// the response or its independently persistable conclusions.
			continue
		}
		projection.ConditionalQualifications = append(projection.ConditionalQualifications, AIConditionalQualificationProjection{
			EducationLevel: educationLevel,
			Evidence:       conditionalEvidence,
			MinimumYears:   conditional.MinYears,
			ReviewTier:     conditional.ReviewRecommendation,
			SubjectKey:     subjectKey,
			SubjectKind:    subjectKind,
		})
	}

	switch response.Status {
	case "configured", "not_configured", "uncertain":
	default:
		return AIExperienceProjection{}, fmt.Errorf("experience status is invalid")
	}
	return projection, nil
}

func (input AIModelInput) mapMinimumEducation(response AIMinimumEducationResponse) (AIMinimumEducationProjection, error) {
	evidence := input.resolveKnownSpanIDs(response.EvidenceSpanIDs)
	projection := AIMinimumEducationProjection{
		Evidence:   evidence,
		ReviewTier: response.ReviewRecommendation,
		Status:     response.Status,
	}
	if response.Level != nil {
		level, _, exists := normalizedEducationLevel(*response.Level)
		if exists {
			projection.Level = level
		}
	}
	switch response.Status {
	case "configured", "not_configured", "uncertain":
	default:
		return AIMinimumEducationProjection{}, fmt.Errorf("education status is invalid")
	}
	return projection, nil
}

func (input AIModelInput) mapRequirementGroups(groups []AITechnologyRequirementGroupResponse) ([]AIRequirementGroupProjection, error) {
	projections := make([]AIRequirementGroupProjection, 0, len(groups))
	for _, group := range groups {
		evidence := input.resolveKnownSpanIDs(group.EvidenceSpanIDs)
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
		if group.Role == "unspecified" || group.Necessity == "unspecified" {
			projection.CandidateRequirement = false
		}
		seenTechnologyKeys := make(map[string]struct{}, len(group.Options))
		for _, option := range group.Options {
			if !input.responseContract.hasTechnologyKey(option.TechnologyKey) {
				// A formal option can only reference an identity already governed by
				// the deterministic input. Drop an unmappable option, not the group
				// or unrelated conclusions.
				continue
			}
			mentionEvidenceIDs := make([]string, 0, len(option.MentionEvidenceIDs))
			seenEvidenceIDs := make(map[string]struct{}, len(option.MentionEvidenceIDs))
			for _, mentionEvidenceID := range option.MentionEvidenceIDs {
				reference, exists := input.responseContract.knownMentionReferences[mentionEvidenceID]
				if !exists || reference.technologyKey != option.TechnologyKey {
					continue
				}
				if _, duplicate := seenEvidenceIDs[mentionEvidenceID]; duplicate {
					continue
				}
				seenEvidenceIDs[mentionEvidenceID] = struct{}{}
				mentionEvidenceIDs = append(mentionEvidenceIDs, mentionEvidenceID)
			}
			if len(mentionEvidenceIDs) == 0 {
				continue
			}
			if _, duplicate := seenTechnologyKeys[option.TechnologyKey]; duplicate {
				// A group can only store one option per governed identity. Keep the
				// first structurally valid option rather than rejecting its group.
				continue
			}
			seenTechnologyKeys[option.TechnologyKey] = struct{}{}
			proficiency := normalizedProficiency(option.Proficiency)
			projection.Options = append(projection.Options, AIRequirementOptionProjection{
				MentionEvidenceIDs: mentionEvidenceIDs,
				Proficiency:        proficiency,
				TechnologyKey:      option.TechnologyKey,
			})
		}
		projections = append(projections, projection)
	}
	return projections, nil
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

func semanticMappingError(err error) error {
	// Every response reaching this layer already passed the strict schema. This
	// is therefore an internal representability failure, not a semantic policy
	// judgement about a model conclusion.
	return NewSafeAIError(AISafeErrorInvalidResponseSchema, err)
}
