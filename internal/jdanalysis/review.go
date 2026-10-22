package jdanalysis

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ReviewVerdict string

const (
	ReviewCorrect   ReviewVerdict = "correct"
	ReviewIncorrect ReviewVerdict = "incorrect"
	ReviewUncertain ReviewVerdict = "uncertain"
)

func (verdict ReviewVerdict) Valid() bool {
	return verdict == ReviewCorrect || verdict == ReviewIncorrect || verdict == ReviewUncertain
}

type ReviewInput struct {
	AnalysisResultID            string
	TechnologyMentionEvidenceID string
	Verdict                     ReviewVerdict
	ErrorCategory               string
	Reviewer                    string
}

// RemoteReviewEvidence returns the exact normalized remote field text only to
// a caller already connected to the remote analysis database. It must never be
// serialized to local artifacts, operational output, or logs. All other review
// APIs use the identifier and coordinate fields below.
type RemoteReviewEvidence struct {
	AnalysisResultID            string
	TechnologyMentionEvidenceID string
	BodyContentVersionID        string
	ParserVersion               string
	DictionaryVersionID         string
	SourceField                 string
	NormalizationVersion        string
	NormalizedFieldSHA256       string
	DictionaryRuleID            string
	MatchedAlias                string
	CodePointStart              int
	CodePointEnd                int
	NormalizedFieldText         string
}

// LoadRemoteReviewEvidence is the narrow, controlled remote query used by an
// operator before recording a review. It verifies that the evidence belongs
// to the result and returns one normalized body field, never a raw response or
// an arbitrary job/body lookup.
func (repository *Repository) LoadRemoteReviewEvidence(ctx context.Context, analysisResultID, evidenceID string) (RemoteReviewEvidence, error) {
	if analysisResultID == "" || evidenceID == "" {
		return RemoteReviewEvidence{}, fmt.Errorf("review evidence identity is incomplete")
	}
	var evidence RemoteReviewEvidence
	err := repository.pool.QueryRow(ctx, `
SELECT result.id::text, evidence.id::text, result.body_content_version_id::text,
       result.parser_version, result.dictionary_version_id::text,
       evidence.source_field, evidence.normalization_version,
       evidence.normalized_field_sha256, evidence.dictionary_rule_id::text,
       evidence.matched_alias, evidence.codepoint_start, evidence.codepoint_end,
       CASE evidence.source_field
           WHEN 'description' THEN body.description_text
           WHEN 'jobRequire' THEN body.job_require_text
           WHEN 'addition' THEN body.addition_text
       END
FROM technology_mention_evidence AS evidence
JOIN technology_mentions AS mention ON mention.id = evidence.technology_mention_id
JOIN technology_analysis_results AS result ON result.id = mention.analysis_result_id
JOIN job_body_content_versions AS body ON body.id = result.body_content_version_id
WHERE result.id = $1 AND evidence.id = $2`, analysisResultID, evidenceID).Scan(
		&evidence.AnalysisResultID, &evidence.TechnologyMentionEvidenceID, &evidence.BodyContentVersionID,
		&evidence.ParserVersion, &evidence.DictionaryVersionID, &evidence.SourceField,
		&evidence.NormalizationVersion, &evidence.NormalizedFieldSHA256,
		&evidence.DictionaryRuleID, &evidence.MatchedAlias, &evidence.CodePointStart,
		&evidence.CodePointEnd, &evidence.NormalizedFieldText)
	if err != nil {
		return RemoteReviewEvidence{}, fmt.Errorf("load controlled remote review evidence: %w", err)
	}
	return evidence, nil
}

// RecordReview appends an immutable human decision. It deliberately does not
// update result, mention, evidence, dictionary, or configuration rows.
func (repository *Repository) RecordReview(ctx context.Context, input ReviewInput) (string, error) {
	if err := input.Validate(); err != nil {
		return "", err
	}
	var reviewID string
	err := repository.pool.QueryRow(ctx, `
INSERT INTO technology_mention_reviews (
    analysis_result_id, technology_mention_evidence_id, verdict, error_category, reviewer
)
SELECT $1, evidence.id, $3, NULLIF($4, ''), $5
FROM technology_mention_evidence AS evidence
JOIN technology_mentions AS mention ON mention.id = evidence.technology_mention_id
WHERE evidence.id = $2 AND mention.analysis_result_id = $1
RETURNING id::text`, input.AnalysisResultID, input.TechnologyMentionEvidenceID,
		input.Verdict, input.ErrorCategory, input.Reviewer).Scan(&reviewID)
	if err != nil {
		return "", fmt.Errorf("record immutable technology mention review: %w", err)
	}
	return reviewID, nil
}

func (input ReviewInput) Validate() error {
	if input.AnalysisResultID == "" || input.TechnologyMentionEvidenceID == "" || !input.Verdict.Valid() || strings.TrimSpace(input.Reviewer) == "" {
		return fmt.Errorf("technology mention review is incomplete")
	}
	if len(input.Reviewer) > 256 || len(input.ErrorCategory) > 128 {
		return fmt.Errorf("technology mention review exceeds permitted field length")
	}
	if input.Verdict == ReviewIncorrect && strings.TrimSpace(input.ErrorCategory) == "" {
		return fmt.Errorf("incorrect technology mention review requires an error category")
	}
	if input.Verdict != ReviewIncorrect && input.ErrorCategory != "" {
		return fmt.Errorf("only incorrect technology mention reviews may have an error category")
	}
	return nil
}

type ReviewRecord struct {
	ID                          string
	AnalysisResultID            string
	TechnologyMentionEvidenceID string
	Verdict                     ReviewVerdict
	ErrorCategory               string
	Reviewer                    string
	ReviewedAt                  time.Time
}

func (repository *Repository) ListReviewsForResult(ctx context.Context, analysisResultID string) ([]ReviewRecord, error) {
	if analysisResultID == "" {
		return nil, fmt.Errorf("analysis result id is required")
	}
	rows, err := repository.pool.Query(ctx, `
SELECT id::text, analysis_result_id::text, technology_mention_evidence_id::text,
       verdict::text, COALESCE(error_category, ''), reviewer, reviewed_at
FROM technology_mention_reviews
WHERE analysis_result_id = $1
ORDER BY reviewed_at, id`, analysisResultID)
	if err != nil {
		return nil, fmt.Errorf("list technology mention reviews: %w", err)
	}
	defer rows.Close()
	reviews := make([]ReviewRecord, 0)
	for rows.Next() {
		var review ReviewRecord
		if err := rows.Scan(&review.ID, &review.AnalysisResultID, &review.TechnologyMentionEvidenceID,
			&review.Verdict, &review.ErrorCategory, &review.Reviewer, &review.ReviewedAt); err != nil {
			return nil, fmt.Errorf("scan technology mention review: %w", err)
		}
		reviews = append(reviews, review)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate technology mention reviews: %w", err)
	}
	return reviews, nil
}
