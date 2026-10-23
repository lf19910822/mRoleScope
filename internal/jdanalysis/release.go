package jdanalysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	// V02ReleaseGateVersion identifies the fixed, deterministic gate used for
	// the first baseline. It is attested with every release event rather than
	// accepting a caller-provided policy label.
	V02ReleaseGateVersion    = "technology-mentions-v0.2-release-gate-v1"
	FallbackReleasePublisher = "FAN"
)

type WithdrawalCategory string

const (
	WithdrawalMaterialResultQualityDefect WithdrawalCategory = "material_result_quality_defect"
	WithdrawalSecurityPrivacyIncident     WithdrawalCategory = "security_privacy_incident"
)

var (
	ErrReleaseGateFailed       = errors.New("candidate analysis configuration did not pass the release gate")
	ErrBaselineWithdrawalState = errors.New("published v0.2 baseline is unavailable for withdrawal")
	ErrWithdrawalCategory      = errors.New("withdrawal category is not allowed")
)

// ReleaseSummary deliberately contains only durable identifiers and counts.
// It can cross the root-owned operations boundary without exposing body text.
type ReleaseSummary struct {
	ConfigurationID     string
	DictionaryVersionID string
	RuleSetSHA256       string
	ReleaseGateVersion  string
	Publisher           string
	Queued              int
	Pending             int
	MissingResults      int
	EvidenceTotal       int
	InvalidEvidence     int
	Published           bool
	Reused              bool
}

type WithdrawalSummary struct {
	ConfigurationID     string
	DictionaryVersionID string
	RuleSetSHA256       string
	ReleaseGateVersion  string
	Publisher           string
	Category            WithdrawalCategory
	Withdrawn           bool
	Reused              bool
}

type releaseConfiguration struct {
	ID                  string
	DictionaryVersionID string
	RuleSetSHA256       string
	Status              string
}

// ReleaseV02Baseline is the only repository release entry point for the
// first baseline. It resolves the fixed v0.2 dictionary and parser itself;
// callers cannot select a configuration, rule set, SQL statement, or gate.
func (repository *Repository) ReleaseV02Baseline(ctx context.Context, authenticatedPublisher string) (ReleaseSummary, error) {
	publisher := resolvedReleasePublisher(authenticatedPublisher)
	configurationID, err := repository.v02ConfigurationID(ctx)
	if err != nil {
		return ReleaseSummary{}, err
	}
	release, acquired, err := repository.TryAcquireConfigurationLock(ctx, configurationID)
	if err != nil {
		return ReleaseSummary{}, err
	}
	if !acquired {
		return ReleaseSummary{}, ErrConfigurationBusy
	}
	defer release()

	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ReleaseSummary{}, fmt.Errorf("begin v0.2 baseline release: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := ensurePublicationState(ctx, tx); err != nil {
		return ReleaseSummary{}, err
	}

	var currentConfigurationID *string
	if err := tx.QueryRow(ctx, `
SELECT configuration_id::text
FROM technology_analysis_publication_state
WHERE singleton = true
FOR UPDATE`).Scan(&currentConfigurationID); err != nil {
		return ReleaseSummary{}, fmt.Errorf("lock analysis publication state for v0.2 release: %w", err)
	}

	dictionaryID, dictionaryStatus, err := lockV02Dictionary(ctx, tx)
	if err != nil {
		return ReleaseSummary{}, err
	}
	configuration, err := lockReleaseConfiguration(ctx, tx, configurationID, dictionaryID)
	if err != nil {
		return ReleaseSummary{}, err
	}
	summary := ReleaseSummary{
		ConfigurationID: configuration.ID, DictionaryVersionID: dictionaryID,
		RuleSetSHA256: configuration.RuleSetSHA256, ReleaseGateVersion: V02ReleaseGateVersion,
	}

	if dictionaryStatus == "published" && configuration.Status == "published" &&
		currentConfigurationID != nil && *currentConfigurationID == configuration.ID {
		if err := tx.QueryRow(ctx, `
SELECT publisher
FROM technology_analysis_release_events
WHERE configuration_id = $1 AND event_type = 'published'`, configuration.ID).Scan(&summary.Publisher); err != nil {
			return ReleaseSummary{}, fmt.Errorf("load v0.2 release attestation: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return ReleaseSummary{}, fmt.Errorf("commit reused v0.2 release: %w", err)
		}
		summary.Published = true
		summary.Reused = true
		return summary, nil
	}
	if dictionaryStatus != "candidate" || configuration.Status != "candidate" || currentConfigurationID != nil {
		return ReleaseSummary{}, ErrReleaseGateFailed
	}

	if err := validateFrozenV02RuleSet(ctx, tx, dictionaryID, configuration); err != nil {
		return ReleaseSummary{}, err
	}
	queued, err := enqueueAllBodyVersions(ctx, tx, configuration.ID)
	if err != nil {
		return ReleaseSummary{}, err
	}
	summary.Queued = queued
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM technology_analysis_work_items
WHERE configuration_id = $1 AND status <> 'complete'`, configuration.ID).Scan(&summary.Pending); err != nil {
		return ReleaseSummary{}, fmt.Errorf("count incomplete v0.2 analysis work: %w", err)
	}
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM job_body_content_versions AS body
LEFT JOIN technology_analysis_results AS result
  ON result.body_content_version_id = body.id
 AND result.configuration_id = $1
 AND result.status = 'complete'
WHERE result.id IS NULL`, configuration.ID).Scan(&summary.MissingResults); err != nil {
		return ReleaseSummary{}, fmt.Errorf("count missing v0.2 analysis results: %w", err)
	}
	if summary.Pending != 0 || summary.MissingResults != 0 {
		if err := appendConfigurationAuditTx(ctx, tx, configuration.ID, "candidate_not_ready", map[string]int{
			"queued": queued, "pending": summary.Pending, "missing_results": summary.MissingResults,
		}); err != nil {
			return ReleaseSummary{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ReleaseSummary{}, fmt.Errorf("commit v0.2 release catch-up: %w", err)
		}
		return summary, ErrCandidateNotReady
	}

	summary.EvidenceTotal, summary.InvalidEvidence, err = validateReleaseEvidence(ctx, tx, configuration.ID)
	if err != nil {
		return ReleaseSummary{}, err
	}
	if summary.InvalidEvidence != 0 {
		if err := appendConfigurationAuditTx(ctx, tx, configuration.ID, "release_validation_failed", map[string]int{
			"invalid_evidence": summary.InvalidEvidence,
		}); err != nil {
			return ReleaseSummary{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ReleaseSummary{}, fmt.Errorf("commit v0.2 evidence validation result: %w", err)
		}
		return summary, ErrReleaseGateFailed
	}

	if _, err := tx.Exec(ctx, `
UPDATE technology_dictionary_versions
SET status = 'published', published_at = now(), retired_at = NULL
WHERE id = $1 AND status = 'candidate'`, dictionaryID); err != nil {
		return ReleaseSummary{}, fmt.Errorf("publish v0.2 dictionary: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE technology_analysis_configurations
SET status = 'published', published_at = now(), failed_at = NULL, safe_failure_code = NULL
WHERE id = $1 AND status = 'candidate'`, configuration.ID); err != nil {
		return ReleaseSummary{}, fmt.Errorf("publish v0.2 analysis configuration: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE technology_analysis_publication_state
SET configuration_id = $1, updated_at = now()
WHERE singleton = true`, configuration.ID); err != nil {
		return ReleaseSummary{}, fmt.Errorf("advance v0.2 analysis publication pointer: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO technology_analysis_release_events (
    configuration_id, dictionary_version_id, event_type, rule_set_sha256,
    release_gate_version, publisher
) VALUES ($1, $2, 'published', $3, $4, $5)`,
		configuration.ID, dictionaryID, configuration.RuleSetSHA256, V02ReleaseGateVersion, publisher); err != nil {
		return ReleaseSummary{}, fmt.Errorf("record v0.2 release attestation: %w", err)
	}
	if err := appendConfigurationAuditTx(ctx, tx, configuration.ID, "candidate_published", map[string]int{"queued": queued}); err != nil {
		return ReleaseSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ReleaseSummary{}, fmt.Errorf("commit v0.2 baseline release: %w", err)
	}
	summary.Publisher = publisher
	summary.Published = true
	return summary, nil
}

// WithdrawV02Baseline retires the only first baseline instead of deleting
// evidence. It has no arbitrary target or free-form reason: callers choose
// exactly one of the two policy categories defined above.
func (repository *Repository) WithdrawV02Baseline(ctx context.Context, authenticatedPublisher string, category WithdrawalCategory) (WithdrawalSummary, error) {
	if !category.Valid() {
		return WithdrawalSummary{}, ErrWithdrawalCategory
	}
	publisher := resolvedReleasePublisher(authenticatedPublisher)
	configurationID, err := repository.PublishedConfigurationID(ctx)
	if err != nil {
		return WithdrawalSummary{}, ErrBaselineWithdrawalState
	}
	release, acquired, err := repository.TryAcquireConfigurationLock(ctx, configurationID)
	if err != nil {
		return WithdrawalSummary{}, err
	}
	if !acquired {
		return WithdrawalSummary{}, ErrConfigurationBusy
	}
	defer release()

	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return WithdrawalSummary{}, fmt.Errorf("begin v0.2 baseline withdrawal: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := ensurePublicationState(ctx, tx); err != nil {
		return WithdrawalSummary{}, err
	}
	var currentConfigurationID *string
	if err := tx.QueryRow(ctx, `
SELECT configuration_id::text
FROM technology_analysis_publication_state
WHERE singleton = true
FOR UPDATE`).Scan(&currentConfigurationID); err != nil {
		return WithdrawalSummary{}, fmt.Errorf("lock analysis publication state for withdrawal: %w", err)
	}
	if currentConfigurationID == nil || *currentConfigurationID != configurationID {
		return WithdrawalSummary{}, ErrBaselineWithdrawalState
	}
	dictionaryID, dictionaryStatus, err := lockV02Dictionary(ctx, tx)
	if err != nil {
		return WithdrawalSummary{}, err
	}
	configuration, err := lockReleaseConfiguration(ctx, tx, configurationID, dictionaryID)
	if err != nil {
		return WithdrawalSummary{}, err
	}
	if dictionaryStatus != "published" || configuration.Status != "published" {
		return WithdrawalSummary{}, ErrBaselineWithdrawalState
	}
	var publishedAttestations int
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM technology_analysis_release_events
WHERE configuration_id = $1
  AND dictionary_version_id = $2
  AND event_type = 'published'`, configuration.ID, dictionaryID).Scan(&publishedAttestations); err != nil {
		return WithdrawalSummary{}, fmt.Errorf("verify v0.2 release attestation: %w", err)
	}
	if publishedAttestations != 1 {
		return WithdrawalSummary{}, ErrBaselineWithdrawalState
	}
	if _, err := tx.Exec(ctx, `
UPDATE technology_analysis_configurations
SET status = 'retired'
WHERE id = $1 AND status = 'published'`, configuration.ID); err != nil {
		return WithdrawalSummary{}, fmt.Errorf("retire v0.2 analysis configuration: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE technology_dictionary_versions
SET status = 'retired', retired_at = now()
WHERE id = $1 AND status = 'published'`, dictionaryID); err != nil {
		return WithdrawalSummary{}, fmt.Errorf("retire v0.2 dictionary: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE technology_analysis_publication_state
SET configuration_id = NULL, updated_at = now()
WHERE singleton = true`); err != nil {
		return WithdrawalSummary{}, fmt.Errorf("clear v0.2 analysis publication pointer: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO technology_analysis_release_events (
    configuration_id, dictionary_version_id, event_type, rule_set_sha256,
    release_gate_version, publisher, withdrawal_category
) VALUES ($1, $2, 'withdrawn', $3, $4, $5, $6)`,
		configuration.ID, dictionaryID, configuration.RuleSetSHA256, V02ReleaseGateVersion, publisher, category); err != nil {
		return WithdrawalSummary{}, fmt.Errorf("record v0.2 withdrawal attestation: %w", err)
	}
	if err := appendConfigurationAuditTx(ctx, tx, configuration.ID, "baseline_withdrawn", map[string]int{}); err != nil {
		return WithdrawalSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return WithdrawalSummary{}, fmt.Errorf("commit v0.2 baseline withdrawal: %w", err)
	}
	return WithdrawalSummary{
		ConfigurationID: configuration.ID, DictionaryVersionID: dictionaryID,
		RuleSetSHA256: configuration.RuleSetSHA256, ReleaseGateVersion: V02ReleaseGateVersion,
		Publisher: publisher, Category: category, Withdrawn: true,
	}, nil
}

func (category WithdrawalCategory) Valid() bool {
	return category == WithdrawalMaterialResultQualityDefect || category == WithdrawalSecurityPrivacyIncident
}

func resolvedReleasePublisher(value string) string {
	if validOperator(value) {
		return value
	}
	return FallbackReleasePublisher
}

func (repository *Repository) v02ConfigurationID(ctx context.Context) (string, error) {
	var configurationID string
	err := repository.pool.QueryRow(ctx, `
SELECT configuration.id::text
FROM technology_analysis_configurations AS configuration
JOIN technology_dictionary_versions AS version ON version.id = configuration.dictionary_version_id
WHERE version.version_label = $1
  AND configuration.parser_version = $2
  AND configuration.status IN ('candidate', 'published')
ORDER BY configuration.created_at DESC
LIMIT 1`, CandidateV02Seed().VersionLabel, V01ParserVersion).Scan(&configurationID)
	if err == pgx.ErrNoRows {
		return "", ErrCandidateNotReady
	}
	if err != nil {
		return "", fmt.Errorf("find v0.2 release configuration: %w", err)
	}
	return configurationID, nil
}

func lockV02Dictionary(ctx context.Context, tx pgx.Tx) (string, string, error) {
	var dictionaryID, status string
	err := tx.QueryRow(ctx, `
SELECT id::text, status::text
FROM technology_dictionary_versions
WHERE version_label = $1
FOR UPDATE`, CandidateV02Seed().VersionLabel).Scan(&dictionaryID, &status)
	if err == pgx.ErrNoRows {
		return "", "", ErrCandidateNotReady
	}
	if err != nil {
		return "", "", fmt.Errorf("lock v0.2 dictionary: %w", err)
	}
	return dictionaryID, status, nil
}

func lockReleaseConfiguration(ctx context.Context, tx pgx.Tx, configurationID, dictionaryID string) (releaseConfiguration, error) {
	var configuration releaseConfiguration
	err := tx.QueryRow(ctx, `
SELECT id::text, dictionary_version_id::text, rule_set_sha256, status::text
FROM technology_analysis_configurations
WHERE id = $1 AND dictionary_version_id = $2
FOR UPDATE`, configurationID, dictionaryID).Scan(
		&configuration.ID, &configuration.DictionaryVersionID, &configuration.RuleSetSHA256, &configuration.Status)
	if err == pgx.ErrNoRows {
		return releaseConfiguration{}, ErrCandidateNotReady
	}
	if err != nil {
		return releaseConfiguration{}, fmt.Errorf("lock v0.2 release configuration: %w", err)
	}
	return configuration, nil
}

func validateFrozenV02RuleSet(ctx context.Context, tx pgx.Tx, dictionaryID string, configuration releaseConfiguration) error {
	snapshot, err := (&Repository{}).loadEnabledRuleSet(ctx, tx, dictionaryID)
	if err != nil {
		return fmt.Errorf("load v0.2 release rule set: %w", err)
	}
	if snapshot.SHA256 != configuration.RuleSetSHA256 {
		return ErrReleaseGateFailed
	}
	rows, err := tx.Query(ctx, `
SELECT rule.rule_key, rule.state::text, rule.approved_by IS NOT NULL AND rule.approved_at IS NOT NULL
FROM technology_analysis_configuration_rules AS snapshot
JOIN technology_dictionary_rules AS rule ON rule.id = snapshot.dictionary_rule_id
WHERE snapshot.configuration_id = $1
ORDER BY rule.rule_key`, configuration.ID)
	if err != nil {
		return fmt.Errorf("load frozen v0.2 release rule-set members: %w", err)
	}
	defer rows.Close()
	expected := make(map[string]struct{}, len(V02ApprovedRuleKeys()))
	for _, ruleKey := range V02ApprovedRuleKeys() {
		expected[ruleKey] = struct{}{}
	}
	actual := make(map[string]struct{}, len(expected))
	for rows.Next() {
		var ruleKey, state string
		var approved bool
		if err := rows.Scan(&ruleKey, &state, &approved); err != nil {
			return fmt.Errorf("scan frozen v0.2 release rule-set member: %w", err)
		}
		if state != string(RuleEnabled) || !approved {
			return ErrReleaseGateFailed
		}
		if _, allowed := expected[ruleKey]; !allowed {
			return ErrReleaseGateFailed
		}
		actual[ruleKey] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate frozen v0.2 release rule-set members: %w", err)
	}
	if len(actual) != len(expected) || len(actual) != len(snapshot.RuleIDs) {
		return ErrReleaseGateFailed
	}
	return nil
}

// validateReleaseEvidence replays every stored coordinate inside PostgreSQL
// and returns counts only. The query never transfers normalized field text,
// aliases, snippets, or raw source data out of the remote database.
func validateReleaseEvidence(ctx context.Context, tx pgx.Tx, configurationID string) (total, invalid int, err error) {
	err = tx.QueryRow(ctx, `
WITH evidence_rows AS (
    SELECT evidence.codepoint_start, evidence.codepoint_end, evidence.matched_alias,
           evidence.normalization_version, evidence.normalized_field_sha256,
           rule.case_mode, body.normalization_version AS body_normalization_version,
           CASE evidence.source_field
             WHEN 'description' THEN body.description_text
             WHEN 'jobRequire' THEN body.job_require_text
             WHEN 'addition' THEN body.addition_text
           END AS field_text,
           CASE evidence.source_field
             WHEN 'description' THEN body.description_sha256
             WHEN 'jobRequire' THEN body.job_require_sha256
             WHEN 'addition' THEN body.addition_sha256
           END AS field_sha256,
           EXISTS (
             SELECT 1
             FROM technology_analysis_configuration_rules AS frozen
             WHERE frozen.configuration_id = result.configuration_id
               AND frozen.dictionary_rule_id = evidence.dictionary_rule_id
           ) AS in_frozen_rule_set
    FROM technology_mention_evidence AS evidence
    JOIN technology_mentions AS mention ON mention.id = evidence.technology_mention_id
    JOIN technology_analysis_results AS result ON result.id = mention.analysis_result_id
    JOIN job_body_content_versions AS body ON body.id = result.body_content_version_id
    JOIN technology_dictionary_rules AS rule ON rule.id = evidence.dictionary_rule_id
    WHERE result.configuration_id = $1
)
SELECT count(*), count(*) FILTER (WHERE
    normalization_version <> body_normalization_version
    OR normalized_field_sha256 <> field_sha256
    OR codepoint_start < 0
    OR codepoint_end <= codepoint_start
    OR codepoint_end > char_length(field_text)
    OR CASE case_mode
        WHEN 'sensitive' THEN substring(field_text FROM codepoint_start + 1 FOR codepoint_end - codepoint_start) <> matched_alias
        ELSE lower(substring(field_text FROM codepoint_start + 1 FOR codepoint_end - codepoint_start)) <> lower(matched_alias)
      END
    OR (codepoint_start > 0 AND substring(field_text FROM codepoint_start FOR 1) ~ '[A-Za-z0-9_-]')
    OR (codepoint_end < char_length(field_text) AND substring(field_text FROM codepoint_end + 1 FOR 1) ~ '[A-Za-z0-9_-]')
    OR NOT in_frozen_rule_set
)
FROM evidence_rows`, configurationID).Scan(&total, &invalid)
	if err != nil {
		return 0, 0, fmt.Errorf("replay v0.2 release evidence safely: %w", err)
	}
	return total, invalid, nil
}
