package jdanalysis

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists only remote PostgreSQL analysis facts. Callers must not
// pass source snippets or unredacted upstream errors through this boundary.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, databaseURL string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		// pgx parse/connect errors can include portions of a DSN. The caller is
		// an operational boundary, so retain no untrusted database error text.
		return nil, fmt.Errorf("create analysis database pool failed")
	}
	return &Repository{pool: pool}, nil
}

func (repository *Repository) Close() {
	if repository != nil && repository.pool != nil {
		repository.pool.Close()
	}
}

type ResultIdentity struct {
	BodyContentVersionID string
	ParserVersion        string
	DictionaryVersionID  string
}

type MentionInput struct {
	TechnologyIdentityID string
	Evidence             []EvidenceInput
}

// EvidenceInput has no source-text or snippet field. Coordinates are verified
// inside the remote transaction against the stored normalized field text.
type EvidenceInput struct {
	SourceField string
	RuleID      string
	Start       int
	End         int
}

type PublishResult struct {
	ResultID string
	Reused   bool
}

func (repository *Repository) PublishCompleteResult(ctx context.Context, identity ResultIdentity, mentions []MentionInput) (PublishResult, error) {
	if identity.BodyContentVersionID == "" || identity.ParserVersion == "" || identity.DictionaryVersionID == "" {
		return PublishResult{}, fmt.Errorf("analysis result identity is incomplete")
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PublishResult{}, fmt.Errorf("begin analysis result publication: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	result, err := publishCompleteResult(ctx, tx, identity, mentions)
	if err != nil {
		return PublishResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PublishResult{}, fmt.Errorf("commit analysis result publication: %w", err)
	}
	return result, nil
}

type bodyContent struct {
	normalizationVersion string
	description          string
	jobRequire           string
	addition             string
	descriptionSHA       string
	jobRequireSHA        string
	additionSHA          string
}

func publishCompleteResult(ctx context.Context, tx pgx.Tx, identity ResultIdentity, mentions []MentionInput) (PublishResult, error) {
	body, err := loadBodyContent(ctx, tx, identity.BodyContentVersionID)
	if err != nil {
		return PublishResult{}, err
	}
	resultID, status, err := lockOrCreateResult(ctx, tx, identity)
	if err != nil {
		return PublishResult{}, err
	}
	if status == "complete" {
		return PublishResult{ResultID: resultID, Reused: true}, nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM technology_mentions WHERE analysis_result_id = $1`, resultID); err != nil {
		return PublishResult{}, fmt.Errorf("clear incomplete analysis result: %w", err)
	}

	seenTechnologies := make(map[string]struct{}, len(mentions))
	for _, mention := range mentions {
		if mention.TechnologyIdentityID == "" || len(mention.Evidence) == 0 {
			return PublishResult{}, fmt.Errorf("technology mention is incomplete")
		}
		if _, duplicate := seenTechnologies[mention.TechnologyIdentityID]; duplicate {
			return PublishResult{}, fmt.Errorf("analysis result repeats a technology identity")
		}
		seenTechnologies[mention.TechnologyIdentityID] = struct{}{}
		mentionID, err := insertMention(ctx, tx, resultID, mention.TechnologyIdentityID)
		if err != nil {
			return PublishResult{}, err
		}
		for _, evidence := range mention.Evidence {
			if err := insertValidatedEvidence(ctx, tx, identity.DictionaryVersionID, body, mentionID, mention.TechnologyIdentityID, evidence); err != nil {
				return PublishResult{}, err
			}
		}
	}
	if _, err := tx.Exec(ctx, `
UPDATE technology_analysis_results
SET status = 'complete', completed_at = now(), safe_failure_code = NULL
WHERE id = $1`, resultID); err != nil {
		return PublishResult{}, fmt.Errorf("complete analysis result: %w", err)
	}
	return PublishResult{ResultID: resultID}, nil
}

func loadBodyContent(ctx context.Context, tx pgx.Tx, bodyContentVersionID string) (bodyContent, error) {
	var body bodyContent
	err := tx.QueryRow(ctx, `
SELECT normalization_version, description_text, job_require_text, addition_text,
       description_sha256, job_require_sha256, addition_sha256
FROM job_body_content_versions
WHERE id = $1`, bodyContentVersionID).Scan(
		&body.normalizationVersion, &body.description, &body.jobRequire, &body.addition,
		&body.descriptionSHA, &body.jobRequireSHA, &body.additionSHA)
	if err != nil {
		return bodyContent{}, fmt.Errorf("load body content version: %w", err)
	}
	return body, nil
}

func lockOrCreateResult(ctx context.Context, tx pgx.Tx, identity ResultIdentity) (string, string, error) {
	var resultID, status string
	err := tx.QueryRow(ctx, `
SELECT id::text, status::text
FROM technology_analysis_results
WHERE body_content_version_id = $1 AND parser_version = $2 AND dictionary_version_id = $3
FOR UPDATE`, identity.BodyContentVersionID, identity.ParserVersion, identity.DictionaryVersionID).Scan(&resultID, &status)
	if err == nil {
		return resultID, status, nil
	}
	if err != pgx.ErrNoRows {
		return "", "", fmt.Errorf("lock analysis result: %w", err)
	}
	err = tx.QueryRow(ctx, `
INSERT INTO technology_analysis_results (body_content_version_id, parser_version, dictionary_version_id, status)
VALUES ($1, $2, $3, 'running')
RETURNING id::text, status::text`, identity.BodyContentVersionID, identity.ParserVersion, identity.DictionaryVersionID).Scan(&resultID, &status)
	if err != nil {
		return "", "", fmt.Errorf("create analysis result: %w", err)
	}
	return resultID, status, nil
}

func insertMention(ctx context.Context, tx pgx.Tx, resultID, technologyIdentityID string) (string, error) {
	var mentionID string
	err := tx.QueryRow(ctx, `
INSERT INTO technology_mentions (analysis_result_id, technology_identity_id)
VALUES ($1, $2)
RETURNING id::text`, resultID, technologyIdentityID).Scan(&mentionID)
	if err != nil {
		return "", fmt.Errorf("insert technology mention: %w", err)
	}
	return mentionID, nil
}

func insertValidatedEvidence(ctx context.Context, tx pgx.Tx, dictionaryVersionID string, body bodyContent, mentionID, technologyIdentityID string, evidence EvidenceInput) error {
	text, fieldSHA, ok := body.field(evidence.SourceField)
	if !ok {
		return fmt.Errorf("evidence source field is outside the approved JD body scope")
	}
	rule, err := loadExecutableRule(ctx, tx, dictionaryVersionID, technologyIdentityID, evidence.RuleID)
	if err != nil {
		return err
	}
	interval := CodePointRange{Start: evidence.Start, End: evidence.End}
	if err := ValidateEvidenceInterval(evidence.SourceField, text, interval, rule); err != nil {
		return fmt.Errorf("validate evidence interval: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO technology_mention_evidence (
    technology_mention_id, source_field, normalization_version, normalized_field_sha256,
    dictionary_rule_id, matched_alias, codepoint_start, codepoint_end
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		mentionID, evidence.SourceField, body.normalizationVersion, fieldSHA,
		evidence.RuleID, rule.Alias, evidence.Start, evidence.End); err != nil {
		return fmt.Errorf("insert technology mention evidence: %w", err)
	}
	return nil
}

func (body bodyContent) field(sourceField string) (string, string, bool) {
	switch sourceField {
	case SourceFieldDescription:
		return body.description, body.descriptionSHA, true
	case SourceFieldJobRequire:
		return body.jobRequire, body.jobRequireSHA, true
	case SourceFieldAddition:
		return body.addition, body.additionSHA, true
	default:
		return "", "", false
	}
}

func loadExecutableRule(ctx context.Context, tx pgx.Tx, dictionaryVersionID, technologyIdentityID, ruleID string) (DictionaryRule, error) {
	var rule DictionaryRule
	var reviewer string
	var decidedAt time.Time
	var rationale string
	err := tx.QueryRow(ctx, `
SELECT identity.technology_key, identity.canonical_name, identity.technology_kind,
       rule.rule_key, rule.alias, rule.case_mode, rule.boundary_mode, rule.overlap_priority,
       rule.rule_summary, rule.approved_by, rule.approved_at
FROM technology_dictionary_rules AS rule
JOIN technology_identities AS identity ON identity.id = rule.technology_identity_id
WHERE rule.id = $1
  AND rule.dictionary_version_id = $2
  AND rule.technology_identity_id = $3
  AND rule.state = 'enabled'`, ruleID, dictionaryVersionID, technologyIdentityID).Scan(
		&rule.TechnologyKey, &rule.CanonicalName, &rule.TechnologyKind,
		&rule.RuleKey, &rule.Alias, &rule.CaseMode, &rule.BoundaryMode, &rule.OverlapPriority,
		&rule.Summary, &reviewer, &decidedAt)
	if err != nil {
		return DictionaryRule{}, fmt.Errorf("load approved dictionary rule: %w", err)
	}
	// The table-level enabled constraint guarantees approval fields exist. The
	// rationale is intentionally supplied as a safe fixed provenance marker;
	// the detailed decision remains in the separate audit table.
	rationale = "recorded dictionary approval"
	rule.State = RuleEnabled
	rule.Approval = &Approval{Reviewer: reviewer, Decided: decidedAt, Rationale: rationale}
	return rule, nil
}
