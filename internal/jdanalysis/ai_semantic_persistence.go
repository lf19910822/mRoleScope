package jdanalysis

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// persistAndCompleteAIWork atomically stores the validated, normalized base
// semantic conclusions and completes their ledger item. It accepts a
// projection rather than a provider response, so raw model output never
// reaches PostgreSQL.
func (repository *Repository) persistAndCompleteAIWork(ctx context.Context, configurationID, workID, bodyContentVersionID string, input AIModelInput, projection AISemanticProjection) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin AI semantic persistence: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := repository.persistAndCompleteAIWorkInTransaction(ctx, tx, configurationID, workID, bodyContentVersionID, input, projection); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return newAIPersistenceStageError(aiPersistenceStageFinalization, fmt.Errorf("commit AI semantic persistence: %w", err))
	}
	return nil
}

// persistAndCompleteAIWorkInTransaction is the one semantic persistence path
// shared by production execution and the CVM-only rollback diagnostic. The
// caller owns the transaction lifecycle; this function never commits.
func (repository *Repository) persistAndCompleteAIWorkInTransaction(ctx context.Context, tx pgx.Tx, configurationID, workID, bodyContentVersionID string, input AIModelInput, projection AISemanticProjection) error {
	var resultID string
	err := tx.QueryRow(ctx, `
SELECT id::text
FROM ai_semantic_results
WHERE ai_analysis_configuration_id = $1
  AND body_content_version_id = $2
	  AND execution_status = 'running'
	FOR UPDATE`, configurationID, bodyContentVersionID).Scan(&resultID)
	if err != nil {
		return newAIPersistenceStageError(aiPersistenceStageFinalization, NewSafeAIError(AISafeErrorConfigurationInvalid, err))
	}

	experienceArtifactID, err := insertAISemanticArtifact(ctx, tx, resultID, "experience_summary", projection.Experience.Status, projection.Experience.ReviewTier)
	if err != nil {
		return newAIPersistenceStageError(aiPersistenceStageExperience, err)
	}
	if err := insertAISemanticEvidence(ctx, tx, experienceArtifactID, projection.Experience.Evidence); err != nil {
		return newAIPersistenceStageError(aiPersistenceStageExperience, err)
	}
	if _, err := tx.Exec(ctx, `
	INSERT INTO ai_experience_summaries (id, status, base_year)
	VALUES ($1, $2::ai_semantic_value_status, $3)`, experienceArtifactID, projection.Experience.Status, projection.Experience.BaseYear); err != nil {
		return newAIPersistenceStageError(aiPersistenceStageExperience, fmt.Errorf("insert AI experience summary: %w", err))
	}
	if err := repository.persistAIExperienceItems(ctx, tx, resultID, experienceArtifactID, configurationID, bodyContentVersionID, input, projection.Experience.Items); err != nil {
		return newAIPersistenceStageError(aiPersistenceStageExperience, err)
	}
	if err := repository.persistAIConditionalQualifications(ctx, tx, resultID, experienceArtifactID, projection.Experience.ConditionalQualifications); err != nil {
		return newAIPersistenceStageError(aiPersistenceStageExperience, err)
	}

	educationArtifactID, err := insertAISemanticArtifact(ctx, tx, resultID, "education_summary", projection.MinimumEducation.Status, projection.MinimumEducation.ReviewTier)
	if err != nil {
		return newAIPersistenceStageError(aiPersistenceStageEducation, err)
	}
	if err := insertAISemanticEvidence(ctx, tx, educationArtifactID, projection.MinimumEducation.Evidence); err != nil {
		return newAIPersistenceStageError(aiPersistenceStageEducation, err)
	}
	var educationLevel *string
	if projection.MinimumEducation.Level != "" {
		educationLevel = &projection.MinimumEducation.Level
	}
	if _, err := tx.Exec(ctx, `
	INSERT INTO ai_minimum_education_requirements (id, status, minimum_level)
	VALUES ($1, $2::ai_semantic_value_status, $3::ai_education_level)`, educationArtifactID, projection.MinimumEducation.Status, educationLevel); err != nil {
		return newAIPersistenceStageError(aiPersistenceStageEducation, fmt.Errorf("insert AI minimum education summary: %w", err))
	}

	persistedCandidateCount, err := repository.persistAICandidateMentions(ctx, tx, configurationID, resultID, input, projection.CandidateMentions)
	if err != nil {
		return newAIPersistenceStageError(aiPersistenceStageCandidateMention, err)
	}
	if persistedCandidateCount == 0 {
		// The V2 mapper can intentionally skip a local candidate defect during
		// dictionary governance. The result status must describe durable facts,
		// not a transient claim that was safely omitted.
		projection.CandidateMentions = nil
	}
	if err := repository.persistAIRequirementGroups(ctx, tx, configurationID, bodyContentVersionID, resultID, input, projection.RequirementGroups); err != nil {
		return newAIPersistenceStageError(aiPersistenceStageRequirementGroup, err)
	}

	semanticStatus := semanticStatusForProjection(projection)
	command, err := tx.Exec(ctx, `
UPDATE ai_semantic_results
SET execution_status = 'complete', semantic_status = $3::ai_semantic_value_status,
    safe_error_code = NULL, completed_at = now()
WHERE ai_analysis_configuration_id = $1
	  AND body_content_version_id = $2
	  AND execution_status = 'running'`, configurationID, bodyContentVersionID, semanticStatus)
	if err != nil || command.RowsAffected() != 1 {
		return newAIPersistenceStageError(aiPersistenceStageFinalization, NewSafeAIError(AISafeErrorConfigurationInvalid, err))
	}
	command, err = tx.Exec(ctx, `
UPDATE ai_analysis_work_items
SET status = 'complete', completed_at = now(), last_safe_error_code = NULL, updated_at = now()
WHERE id = $1
	  AND ai_analysis_configuration_id = $2
	  AND status = 'running'`, workID, configurationID)
	if err != nil || command.RowsAffected() != 1 {
		return newAIPersistenceStageError(aiPersistenceStageFinalization, NewSafeAIError(AISafeErrorConfigurationInvalid, err))
	}
	return nil
}

// persistAIExperienceItems stores overall, deterministically bound technology,
// and evidence-bound domain experience.
func (repository *Repository) persistAIExperienceItems(ctx context.Context, tx pgx.Tx, resultID, experienceSummaryID, configurationID, bodyContentVersionID string, input AIModelInput, items []AIExperienceItemProjection) error {
	for _, item := range items {
		if item.MinimumYears < 1 {
			return NewSafeAIError(AISafeErrorPolicyRejected, nil)
		}
		var technologyIdentityID *string
		var domainKey *string
		switch item.SubjectKind {
		case "overall":
			if item.SubjectKey != "" {
				return NewSafeAIError(AISafeErrorPolicyRejected, nil)
			}
		case "technology":
			identityID, err := loadVerifiedTechnologyIdentityForExperienceScope(ctx, tx, configurationID, bodyContentVersionID, input, item.SubjectKey)
			if err != nil {
				return err
			}
			technologyIdentityID = &identityID
		case "domain":
			if item.SubjectKey == "" {
				return NewSafeAIError(AISafeErrorPolicyRejected, nil)
			}
			domainKey = &item.SubjectKey
		default:
			return NewSafeAIError(AISafeErrorPolicyRejected, nil)
		}
		artifactID, err := insertAISemanticArtifact(ctx, tx, resultID, "experience_item", "configured", item.ReviewTier)
		if err != nil {
			return err
		}
		if err := insertAISemanticEvidence(ctx, tx, artifactID, item.Evidence); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO ai_experience_requirement_items (
    id, experience_summary_id, subject_kind, technology_identity_id, domain_key, minimum_years
) VALUES ($1, $2, $3, $4, $5, $6)`, artifactID, experienceSummaryID, item.SubjectKind, technologyIdentityID, domainKey, item.MinimumYears); err != nil {
			return fmt.Errorf("insert AI experience item: %w", err)
		}
	}
	return nil
}

// loadVerifiedTechnologyIdentityForExperienceScope resolves a technology scope
// through the exact deterministic evidence that was sent in this model input.
// A model-provided key alone is never sufficient to choose an identity.
func loadVerifiedTechnologyIdentityForExperienceScope(ctx context.Context, tx pgx.Tx, configurationID, bodyContentVersionID string, input AIModelInput, technologyKey string) (string, error) {
	var mentionEvidenceID string
	for _, mention := range input.VerifiedTechnologyMentions {
		if mention.TechnologyKey == technologyKey {
			mentionEvidenceID = mention.MentionEvidenceID
			break
		}
	}
	if mentionEvidenceID == "" {
		return "", NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	var technologyIdentityID string
	err := tx.QueryRow(ctx, `
SELECT identity.id::text
FROM ai_analysis_configurations AS ai_configuration
JOIN technology_analysis_results AS deterministic_result
  ON deterministic_result.configuration_id = ai_configuration.technology_analysis_configuration_id
 AND deterministic_result.body_content_version_id = $2
 AND deterministic_result.status = 'complete'
JOIN technology_mentions AS mention ON mention.analysis_result_id = deterministic_result.id
JOIN technology_mention_evidence AS evidence ON evidence.technology_mention_id = mention.id
JOIN technology_identities AS identity ON identity.id = mention.technology_identity_id
WHERE ai_configuration.id = $1
  AND evidence.id = $3
  AND identity.technology_key = $4`, configurationID, bodyContentVersionID, mentionEvidenceID, technologyKey).Scan(&technologyIdentityID)
	if err != nil {
		return "", NewSafeAIError(AISafeErrorInvalidEvidence, err)
	}
	return technologyIdentityID, nil
}

// persistAIConditionalQualifications records only explicit, base-experience
// education branches. The schema deliberately has no subject binding for
// technology/domain branches yet, so accepting either would lose provenance.
func (repository *Repository) persistAIConditionalQualifications(ctx context.Context, tx pgx.Tx, resultID, experienceSummaryID string, branches []AIConditionalQualificationProjection) error {
	for index, branch := range branches {
		if branch.SubjectKind != "overall" || branch.SubjectKey != "" || branch.EducationLevel == "" || branch.MinimumYears < 1 {
			return NewSafeAIError(AISafeErrorPolicyRejected, nil)
		}
		artifactID, err := insertAISemanticArtifact(ctx, tx, resultID, "condition_branch", "configured", branch.ReviewTier)
		if err != nil {
			return err
		}
		if err := insertAISemanticEvidence(ctx, tx, artifactID, branch.Evidence); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO ai_semantic_condition_branches (
    id, experience_summary_id, education_level, minimum_experience_years, branch_order
) VALUES ($1, $2, $3::ai_education_level, $4, $5)`,
			artifactID, experienceSummaryID, branch.EducationLevel, branch.MinimumYears, index+1); err != nil {
			return fmt.Errorf("insert AI education experience condition branch: %w", err)
		}
	}
	return nil
}

func (repository *Repository) persistAICandidateMentions(ctx context.Context, tx pgx.Tx, configurationID, resultID string, input AIModelInput, candidates []AICandidateMentionProjection) (int, error) {
	if len(candidates) == 0 {
		return 0, nil
	}
	var dictionaryVersionID string
	if err := tx.QueryRow(ctx, `
SELECT technology_configuration.dictionary_version_id::text
FROM ai_analysis_configurations AS ai_configuration
JOIN technology_analysis_configurations AS technology_configuration
  ON technology_configuration.id = ai_configuration.technology_analysis_configuration_id
	WHERE ai_configuration.id = $1`, configurationID).Scan(&dictionaryVersionID); err != nil {
		return 0, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	persistedCount := 0
	for _, candidate := range candidates {
		var activeAlias bool
		if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM technology_dictionary_rules
    WHERE dictionary_version_id = $1
      AND state = 'enabled'
      AND lower(alias) = lower($2)
)`, dictionaryVersionID, candidate.CandidateText).Scan(&activeAlias); err != nil {
			return 0, fmt.Errorf("check AI candidate against active dictionary aliases: %w", err)
		}
		if activeAlias {
			// A current dictionary alias is not a dictionary candidate. Under the
			// tolerant validator it is a local model mistake that can be safely
			// omitted; it must not roll back unrelated, verified JD semantics.
			if input.validatorVersion == AIValidatorVersionV2 {
				continue
			}
			return 0, NewSafeAIError(AISafeErrorPolicyRejected, nil)
		}
		var dictionaryCandidateID string
		err := tx.QueryRow(ctx, `
INSERT INTO technology_dictionary_candidates (
    dictionary_version_id, candidate_text, state, candidate_summary
) VALUES ($1, $2, 'candidate', 'AI semantic candidate requires human approval')
ON CONFLICT (dictionary_version_id, candidate_text)
DO UPDATE SET candidate_text = EXCLUDED.candidate_text
RETURNING id::text`, dictionaryVersionID, candidate.CandidateText).Scan(&dictionaryCandidateID)
		if err != nil {
			return 0, NewSafeAIError(AISafeErrorPolicyRejected, err)
		}
		artifactID, err := insertAISemanticArtifact(ctx, tx, resultID, "candidate_technology", "configured", "required_review")
		if err != nil {
			return 0, err
		}
		if err := insertAISemanticEvidence(ctx, tx, artifactID, []AISemanticEvidence{candidate.Evidence}); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO ai_candidate_technology_mentions (id, dictionary_candidate_id)
VALUES ($1, $2)`, artifactID, dictionaryCandidateID); err != nil {
			return 0, fmt.Errorf("link AI dictionary candidate artifact: %w", err)
		}
		persistedCount++
	}
	return persistedCount, nil
}

func (repository *Repository) persistAIRequirementGroups(ctx context.Context, tx pgx.Tx, configurationID, bodyContentVersionID, resultID string, input AIModelInput, groups []AIRequirementGroupProjection) error {
	for _, group := range groups {
		var operator *string
		var minSatisfied *int
		switch group.Role {
		case "qualification":
			if group.Operator != "at_least" || group.MinSatisfied < 1 || group.MinSatisfied > len(group.Options) {
				return NewSafeAIError(AISafeErrorPolicyRejected, nil)
			}
			operator = &group.Operator
			minSatisfied = &group.MinSatisfied
		case "responsibility", "mention", "unspecified":
			if group.Operator != "" || group.MinSatisfied != 0 {
				return NewSafeAIError(AISafeErrorPolicyRejected, nil)
			}
		default:
			return NewSafeAIError(AISafeErrorPolicyRejected, nil)
		}
		groupArtifactID, err := insertAISemanticArtifact(ctx, tx, resultID, "requirement_group", "configured", group.ReviewTier)
		if err != nil {
			return err
		}
		if err := insertAISemanticEvidence(ctx, tx, groupArtifactID, group.Evidence); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO ai_technology_requirement_groups (
    id, role, necessity, operator, min_satisfied
) VALUES ($1, $2::ai_requirement_role, $3::ai_requirement_necessity,
          $4::ai_requirement_operator, $5)`,
			groupArtifactID, group.Role, group.Necessity, operator, minSatisfied); err != nil {
			return fmt.Errorf("insert AI technology requirement group: %w", err)
		}
		for optionOrder, option := range group.Options {
			technologyIdentityID, err := loadVerifiedTechnologyIdentityForOption(ctx, tx, configurationID, bodyContentVersionID, option)
			if err != nil {
				return err
			}
			optionArtifactID, err := insertAISemanticArtifact(ctx, tx, resultID, "requirement_option", "configured", group.ReviewTier)
			if err != nil {
				return err
			}
			if err := insertAIRequirementOptionEvidence(ctx, tx, optionArtifactID, input, option); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
INSERT INTO ai_technology_requirement_options (
    id, requirement_group_id, technology_identity_id, proficiency, option_order
) VALUES ($1, $2, $3, $4::ai_proficiency_level, $5)`,
				optionArtifactID, groupArtifactID, technologyIdentityID, option.Proficiency, optionOrder+1); err != nil {
				return fmt.Errorf("insert AI technology requirement option: %w", err)
			}
		}
	}
	return nil
}

func loadVerifiedTechnologyIdentityForOption(ctx context.Context, tx pgx.Tx, configurationID, bodyContentVersionID string, option AIRequirementOptionProjection) (string, error) {
	if len(option.MentionEvidenceIDs) == 0 {
		return "", NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	var technologyIdentityID string
	err := tx.QueryRow(ctx, `
SELECT identity.id::text
FROM ai_analysis_configurations AS ai_configuration
JOIN technology_analysis_results AS deterministic_result
  ON deterministic_result.configuration_id = ai_configuration.technology_analysis_configuration_id
 AND deterministic_result.body_content_version_id = $2
 AND deterministic_result.status = 'complete'
JOIN technology_mentions AS mention ON mention.analysis_result_id = deterministic_result.id
JOIN technology_mention_evidence AS evidence ON evidence.technology_mention_id = mention.id
JOIN technology_identities AS identity ON identity.id = mention.technology_identity_id
WHERE ai_configuration.id = $1
  AND evidence.id = $3
  AND identity.technology_key = $4`, configurationID, bodyContentVersionID, option.MentionEvidenceIDs[0], option.TechnologyKey).Scan(&technologyIdentityID)
	if err != nil {
		return "", NewSafeAIError(AISafeErrorInvalidEvidence, err)
	}
	return technologyIdentityID, nil
}

func insertAIRequirementOptionEvidence(ctx context.Context, tx pgx.Tx, artifactID string, input AIModelInput, option AIRequirementOptionProjection) error {
	for _, mentionEvidenceID := range option.MentionEvidenceIDs {
		mention, found := findVerifiedTechnologyMention(input, mentionEvidenceID)
		if !found || mention.TechnologyKey != option.TechnologyKey {
			return NewSafeAIError(AISafeErrorInvalidEvidence, nil)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO ai_semantic_evidence (
    semantic_artifact_id, span_id, source_field, normalization_version,
    normalized_field_sha256, codepoint_start, codepoint_end, technology_mention_evidence_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			artifactID, mention.SpanID, mention.SourceField, mention.NormalizationVersion,
			mention.NormalizedFieldSHA256, mention.CodePointRange.Start, mention.CodePointRange.End,
			mention.MentionEvidenceID); err != nil {
			return fmt.Errorf("insert AI technology requirement evidence: %w", err)
		}
	}
	return nil
}

func findVerifiedTechnologyMention(input AIModelInput, mentionEvidenceID string) (VerifiedTechnologyMention, bool) {
	for _, mention := range input.VerifiedTechnologyMentions {
		if mention.MentionEvidenceID == mentionEvidenceID {
			return mention, true
		}
	}
	return VerifiedTechnologyMention{}, false
}

func insertAISemanticArtifact(ctx context.Context, tx pgx.Tx, resultID, kind, status, reviewTier string) (string, error) {
	var artifactID string
	err := tx.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (
    ai_semantic_result_id, artifact_kind, semantic_status, review_tier
) VALUES ($1, $2::ai_semantic_artifact_kind, $3::ai_semantic_value_status, $4::ai_semantic_review_tier)
RETURNING id::text`, resultID, kind, status, reviewTier).Scan(&artifactID)
	if err != nil {
		return "", fmt.Errorf("insert AI semantic artifact: %w", err)
	}
	return artifactID, nil
}

func insertAISemanticEvidence(ctx context.Context, tx pgx.Tx, artifactID string, evidence []AISemanticEvidence) error {
	for _, item := range evidence {
		if _, err := tx.Exec(ctx, `
INSERT INTO ai_semantic_evidence (
    semantic_artifact_id, span_id, source_field, normalization_version,
    normalized_field_sha256, codepoint_start, codepoint_end
) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			artifactID, item.SpanID, item.SourceField, item.NormalizationVersion,
			item.NormalizedFieldSHA256, item.CodePointRange.Start, item.CodePointRange.End); err != nil {
			return fmt.Errorf("insert AI semantic evidence: %w", err)
		}
	}
	return nil
}
