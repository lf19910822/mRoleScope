package jdanalysis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
)

const AIControlledReviewer = "FAN"

type controlledAIReviewTarget struct {
	AIConfigurationID    string
	AISemanticResultID   string
	ArtifactID           string
	ArtifactKind         string
	SemanticStatus       string
	ReviewTier           string
	Description          string
	JobRequire           string
	Addition             string
	StructuredConclusion string
	EvidenceLocations    string
}

// RunControlledAIReviewSession is the only interactive review seam. It is
// intended for a direct CVM terminal: source text is written solely to the
// supplied terminal writer, and the caller can provide only a bounded verdict
// and, for errors, a bounded category. It never logs source text or accepts a
// body ID, model endpoint, prompt, SQL, credential, or free-form rationale.
func RunControlledAIReviewSession(ctx context.Context, repository *Repository, configurationID, model string, input io.Reader, output io.Writer) (bool, error) {
	if repository == nil || configurationID == "" || !validControlledAIReviewModel(model) || input == nil || output == nil {
		return false, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	target, found, err := repository.nextControlledAIReviewTarget(ctx, configurationID, model)
	if err != nil {
		return false, err
	}
	if !found {
		if _, err := fmt.Fprintln(output, "No unreviewed AI semantic artifact is available."); err != nil {
			return false, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
		}
		return false, nil
	}
	if err := writeControlledAIReviewScreen(output, target); err != nil {
		return false, err
	}
	reader := bufio.NewReader(input)
	verdict, err := readControlledReviewLine(reader)
	if err != nil {
		return false, err
	}
	errorCategory := ""
	if verdict == "minor_error" || verdict == "major_error" {
		if _, err := fmt.Fprint(output, "Error category [necessity_reversal|alternative_reversal|education_reversal|experience_reversal|other]: "); err != nil {
			return false, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
		}
		errorCategory, err = readControlledReviewLine(reader)
		if err != nil {
			return false, err
		}
	}
	review := AISemanticReviewInput{
		AIConfigurationID:  target.AIConfigurationID,
		AISemanticResultID: target.AISemanticResultID,
		ArtifactID:         target.ArtifactID,
		Reviewer:           AIControlledReviewer,
		Verdict:            verdict,
		ErrorCategory:      errorCategory,
	}
	if !validAISemanticReviewInput(review) {
		return false, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if err := repository.RecordAISemanticReview(ctx, review); err != nil {
		return false, err
	}
	if _, err := fmt.Fprintf(output, "Recorded immutable verdict: %s\n", verdict); err != nil {
		return false, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return true, nil
}

func validControlledAIReviewModel(model string) bool {
	return model == "qwen-plus"
}

func (repository *Repository) nextControlledAIReviewTarget(ctx context.Context, configurationID, model string) (controlledAIReviewTarget, bool, error) {
	var target controlledAIReviewTarget
	err := repository.pool.QueryRow(ctx, `
SELECT configuration.id::text,
       result.id::text,
       artifact.id::text,
       artifact.artifact_kind::text,
       artifact.semantic_status::text,
       artifact.review_tier::text,
       body.description_text,
       body.job_require_text,
       body.addition_text,
       jsonb_strip_nulls(jsonb_build_object(
           'artifact_kind', artifact.artifact_kind,
           'semantic_status', artifact.semantic_status,
           'experience_status', experience.status,
           'experience_base_year', experience.base_year,
           'experience_subject_kind', experience_item.subject_kind,
           'experience_minimum_years', experience_item.minimum_years,
           'minimum_education_status', education.status,
           'minimum_education_level', education.minimum_level,
           'condition_education_level', condition_branch.education_level,
           'condition_minimum_experience_years', condition_branch.minimum_experience_years,
           'requirement_role', requirement_group.role,
           'requirement_necessity', requirement_group.necessity,
           'requirement_operator', requirement_group.operator,
           'requirement_min_satisfied', requirement_group.min_satisfied,
           'option_technology_key', option_identity.technology_key,
           'option_canonical_name', option_identity.canonical_name,
           'option_proficiency', requirement_option.proficiency,
           'candidate_text', dictionary_candidate.candidate_text
       ))::text,
       COALESCE((
           SELECT jsonb_agg(jsonb_build_object(
               'source_field', evidence.source_field,
               'codepoint_start', evidence.codepoint_start,
               'codepoint_end', evidence.codepoint_end,
               'span_id', evidence.span_id
           ) ORDER BY evidence.source_field, evidence.codepoint_start, evidence.codepoint_end)
           FROM ai_semantic_evidence AS evidence
           WHERE evidence.semantic_artifact_id = artifact.id
       ), '[]'::jsonb)::text
FROM ai_analysis_configurations AS configuration
JOIN ai_semantic_results AS result
  ON result.ai_analysis_configuration_id = configuration.id
 AND result.execution_status = 'complete'
JOIN ai_semantic_artifacts AS artifact ON artifact.ai_semantic_result_id = result.id
JOIN job_body_content_versions AS body ON body.id = result.body_content_version_id
JOIN ai_analysis_work_items AS work
  ON work.ai_analysis_configuration_id = result.ai_analysis_configuration_id
 AND work.body_content_version_id = result.body_content_version_id
LEFT JOIN ai_experience_summaries AS experience ON experience.id = artifact.id
LEFT JOIN ai_experience_requirement_items AS experience_item ON experience_item.id = artifact.id
LEFT JOIN ai_minimum_education_requirements AS education ON education.id = artifact.id
LEFT JOIN ai_semantic_condition_branches AS condition_branch ON condition_branch.id = artifact.id
LEFT JOIN ai_technology_requirement_groups AS requirement_group ON requirement_group.id = artifact.id
LEFT JOIN ai_technology_requirement_options AS requirement_option ON requirement_option.id = artifact.id
LEFT JOIN technology_identities AS option_identity ON option_identity.id = requirement_option.technology_identity_id
LEFT JOIN ai_candidate_technology_mentions AS candidate_mention ON candidate_mention.id = artifact.id
LEFT JOIN technology_dictionary_candidates AS dictionary_candidate ON dictionary_candidate.id = candidate_mention.dictionary_candidate_id
WHERE configuration.id = $1
  AND configuration.model = $2
  AND configuration.model = 'qwen-plus'
  AND configuration.status = 'candidate'
  AND NOT EXISTS (
      SELECT 1
      FROM ai_semantic_reviews AS review
      WHERE review.semantic_artifact_id = artifact.id
        AND review.reviewer = $3
  )
ORDER BY CASE WHEN work.trigger = 'trial' THEN 0 ELSE 1 END,
         CASE artifact.review_tier
             WHEN 'required_review' THEN 0
             WHEN 'spot_check' THEN 1
             ELSE 2
         END,
         artifact.created_at,
         artifact.id
LIMIT 1`, configurationID, model, AIControlledReviewer).Scan(
		&target.AIConfigurationID,
		&target.AISemanticResultID,
		&target.ArtifactID,
		&target.ArtifactKind,
		&target.SemanticStatus,
		&target.ReviewTier,
		&target.Description,
		&target.JobRequire,
		&target.Addition,
		&target.StructuredConclusion,
		&target.EvidenceLocations,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return controlledAIReviewTarget{}, false, nil
	}
	if err != nil {
		return controlledAIReviewTarget{}, false, fmt.Errorf("load controlled AI review target: %w", err)
	}
	return target, true, nil
}

func writeControlledAIReviewScreen(output io.Writer, target controlledAIReviewTarget) error {
	for _, section := range []struct {
		label string
		value string
	}{
		{"artifact_id", target.ArtifactID},
		{"artifact_kind", target.ArtifactKind},
		{"semantic_status", target.SemanticStatus},
		{"review_tier", target.ReviewTier},
		{"structured_conclusion", target.StructuredConclusion},
		{"evidence_locations", target.EvidenceLocations},
		{"description", target.Description},
		{"jobRequire", target.JobRequire},
		{"addition", target.Addition},
	} {
		if _, err := fmt.Fprintf(output, "--- %s ---\n%s\n", section.label, sanitizeControlledReviewTerminalText(section.value)); err != nil {
			return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
		}
	}
	if _, err := fmt.Fprint(output, "Verdict [accepted|not_reviewable|minor_error|major_error]: "); err != nil {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return nil
}

func readControlledReviewLine(reader *bufio.Reader) (string, error) {
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	return value, nil
}

// sanitizeControlledReviewTerminalText blocks untrusted terminal-control
// characters while preserving ordinary Unicode and newlines for human review.
func sanitizeControlledReviewTerminalText(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if character == '\n' || character == '\t' || (!unicode.IsControl(character) && character != 0x1b) {
			builder.WriteRune(character)
			continue
		}
		fmt.Fprintf(&builder, "\\u%04X", character)
	}
	return builder.String()
}
