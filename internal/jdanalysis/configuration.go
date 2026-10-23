package jdanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var (
	ErrCandidateNotReady = errors.New("candidate analysis configuration is not ready to publish")
	ErrCandidateFailed   = errors.New("candidate analysis configuration has failed")
)

// CandidateConfigurationInput is intentionally limited to the immutable
// parser/dictionary identity and a non-secret operator label. It never holds
// source text, source URLs, or collector credentials.
type CandidateConfigurationInput struct {
	ParserVersion       string
	DictionaryVersionID string
	CreatedBy           string
}

type CandidatePreparation struct {
	ConfigurationID string
	Queued          int
	Reused          bool
}

type CandidatePromotion struct {
	ConfigurationID string
	Queued          int
	Pending         int
	MissingResults  int
	Published       bool
	Reused          bool
}

type ruleSetSnapshotEntry struct {
	TechnologyKey   string `json:"technology_key"`
	RuleKey         string `json:"rule_key"`
	Alias           string `json:"alias"`
	CaseMode        string `json:"case_mode"`
	BoundaryMode    string `json:"boundary_mode"`
	OverlapPriority int    `json:"overlap_priority"`
}

type ruleSetSnapshot struct {
	SHA256  string
	RuleIDs []string
}

type ruleSetQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

// loadEnabledRuleSet serializes the exact executable matching semantics in a
// stable order.  This is a configuration input, not a mutable dictionary
// lookup: later approvals must result in a different configuration.
func (repository *Repository) loadEnabledRuleSet(ctx context.Context, querier ruleSetQuerier, dictionaryVersionID string) (ruleSetSnapshot, error) {
	rows, err := querier.Query(ctx, `
SELECT rule.id::text, identity.technology_key, rule.rule_key, rule.alias,
       rule.case_mode, rule.boundary_mode, rule.overlap_priority
FROM technology_dictionary_rules AS rule
JOIN technology_identities AS identity ON identity.id = rule.technology_identity_id
WHERE rule.dictionary_version_id = $1 AND rule.state = 'enabled'
ORDER BY rule.rule_key`, dictionaryVersionID)
	if err != nil {
		return ruleSetSnapshot{}, fmt.Errorf("load enabled rule-set snapshot: %w", err)
	}
	defer rows.Close()

	entries := make([]ruleSetSnapshotEntry, 0)
	ruleIDs := make([]string, 0)
	for rows.Next() {
		var entry ruleSetSnapshotEntry
		var ruleID string
		if err := rows.Scan(&ruleID, &entry.TechnologyKey, &entry.RuleKey, &entry.Alias,
			&entry.CaseMode, &entry.BoundaryMode, &entry.OverlapPriority); err != nil {
			return ruleSetSnapshot{}, fmt.Errorf("scan enabled rule-set snapshot: %w", err)
		}
		entries = append(entries, entry)
		ruleIDs = append(ruleIDs, ruleID)
	}
	if err := rows.Err(); err != nil {
		return ruleSetSnapshot{}, fmt.Errorf("iterate enabled rule-set snapshot: %w", err)
	}
	if len(entries) == 0 {
		return ruleSetSnapshot{}, fmt.Errorf("analysis configuration requires at least one enabled dictionary rule")
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		return ruleSetSnapshot{}, fmt.Errorf("encode enabled rule-set snapshot: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return ruleSetSnapshot{SHA256: hex.EncodeToString(sum[:]), RuleIDs: ruleIDs}, nil
}

func persistConfigurationRuleSnapshot(ctx context.Context, tx pgx.Tx, configurationID string, ruleIDs []string) error {
	for _, ruleID := range ruleIDs {
		if _, err := tx.Exec(ctx, `
INSERT INTO technology_analysis_configuration_rules (configuration_id, dictionary_rule_id)
VALUES ($1, $2)
ON CONFLICT (configuration_id, dictionary_rule_id) DO NOTHING`, configurationID, ruleID); err != nil {
			return fmt.Errorf("persist configuration rule-set snapshot: %w", err)
		}
	}
	return nil
}

// PrepareCandidateConfiguration creates or resumes a candidate configuration
// and queues every historical body version. The unique queue identity makes a
// repeated request idempotent, while a candidate remains invisible to normal
// source publication until PromoteCandidateConfiguration succeeds.
func (repository *Repository) PrepareCandidateConfiguration(ctx context.Context, input CandidateConfigurationInput) (CandidatePreparation, error) {
	if input.ParserVersion == "" || input.DictionaryVersionID == "" || input.CreatedBy == "" {
		return CandidatePreparation{}, fmt.Errorf("candidate analysis configuration is incomplete")
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CandidatePreparation{}, fmt.Errorf("begin candidate configuration preparation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := ensurePublicationState(ctx, tx); err != nil {
		return CandidatePreparation{}, err
	}
	if err := lockCandidateDictionaryVersion(ctx, tx, input.DictionaryVersionID); err != nil {
		return CandidatePreparation{}, err
	}
	snapshot, err := repository.loadEnabledRuleSet(ctx, tx, input.DictionaryVersionID)
	if err != nil {
		return CandidatePreparation{}, err
	}
	var configurationID, status string
	err = tx.QueryRow(ctx, `
INSERT INTO technology_analysis_configurations (
    parser_version, dictionary_version_id, rule_set_sha256, status, created_by
) VALUES ($1, $2, $3, 'candidate', $4)
ON CONFLICT (parser_version, dictionary_version_id, rule_set_sha256)
DO UPDATE SET parser_version = EXCLUDED.parser_version
RETURNING id::text, status::text`, input.ParserVersion, input.DictionaryVersionID, snapshot.SHA256, input.CreatedBy).Scan(&configurationID, &status)
	if err != nil {
		return CandidatePreparation{}, fmt.Errorf("create or find candidate analysis configuration: %w", err)
	}
	if err := persistConfigurationRuleSnapshot(ctx, tx, configurationID, snapshot.RuleIDs); err != nil {
		return CandidatePreparation{}, err
	}
	preparation := CandidatePreparation{ConfigurationID: configurationID, Reused: status == "published"}
	if status == "failed" || status == "retired" {
		return CandidatePreparation{}, fmt.Errorf("candidate analysis configuration cannot resume from status %s", status)
	}
	if status == "candidate" {
		queued, err := enqueueAllBodyVersions(ctx, tx, configurationID)
		if err != nil {
			return CandidatePreparation{}, err
		}
		preparation.Queued = queued
		if err := appendConfigurationAuditTx(ctx, tx, configurationID, "candidate_prepared", map[string]int{"queued": queued}); err != nil {
			return CandidatePreparation{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return CandidatePreparation{}, fmt.Errorf("commit candidate configuration preparation: %w", err)
	}
	return preparation, nil
}

// lockCandidateDictionaryVersion serializes approvals, preparation, and the
// first release against one dictionary version. A frozen version never gains
// a new executable rule or a rewritten configuration snapshot.
func lockCandidateDictionaryVersion(ctx context.Context, tx pgx.Tx, dictionaryVersionID string) error {
	var status string
	err := tx.QueryRow(ctx, `
SELECT status::text
FROM technology_dictionary_versions
WHERE id = $1
FOR UPDATE`, dictionaryVersionID).Scan(&status)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("candidate dictionary version is unavailable")
	}
	if err != nil {
		return fmt.Errorf("lock candidate dictionary version: %w", err)
	}
	if status != "candidate" {
		return fmt.Errorf("candidate dictionary version is immutable")
	}
	return nil
}

// PromoteCandidateConfiguration first catches up every body version that has
// committed since preparation, then advances the singleton publication pointer
// in the same transaction only when each body has a complete result. The
// shared source-publication lock keeps a body commit from racing that check.
func (repository *Repository) PromoteCandidateConfiguration(ctx context.Context, configurationID string) (CandidatePromotion, error) {
	if configurationID == "" {
		return CandidatePromotion{}, fmt.Errorf("candidate analysis configuration id is required")
	}
	release, acquired, err := repository.TryAcquireConfigurationLock(ctx, configurationID)
	if err != nil {
		return CandidatePromotion{}, err
	}
	if !acquired {
		return CandidatePromotion{}, ErrConfigurationBusy
	}
	defer release()

	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CandidatePromotion{}, fmt.Errorf("begin candidate promotion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := ensurePublicationState(ctx, tx); err != nil {
		return CandidatePromotion{}, err
	}
	var previousConfigurationID *string
	if err := tx.QueryRow(ctx, `
SELECT configuration_id::text
FROM technology_analysis_publication_state
WHERE singleton = true
FOR UPDATE`).Scan(&previousConfigurationID); err != nil {
		return CandidatePromotion{}, fmt.Errorf("lock analysis publication state: %w", err)
	}
	var configuration analysisConfiguration
	var status string
	if err := tx.QueryRow(ctx, `
SELECT id::text, parser_version, dictionary_version_id::text, status::text
FROM technology_analysis_configurations
WHERE id = $1
FOR UPDATE`, configurationID).Scan(&configuration.ID, &configuration.ParserVersion, &configuration.DictionaryVersionID, &status); err != nil {
		return CandidatePromotion{}, fmt.Errorf("lock candidate analysis configuration: %w", err)
	}
	promotion := CandidatePromotion{ConfigurationID: configurationID}
	if status == "published" {
		if err := tx.Commit(ctx); err != nil {
			return CandidatePromotion{}, fmt.Errorf("commit reused candidate promotion: %w", err)
		}
		promotion.Published = true
		promotion.Reused = true
		return promotion, nil
	}
	if status == "failed" || status == "retired" {
		return CandidatePromotion{}, ErrCandidateFailed
	}
	if status != "candidate" {
		return CandidatePromotion{}, fmt.Errorf("candidate analysis configuration has invalid status %s", status)
	}
	queued, err := enqueueAllBodyVersions(ctx, tx, configurationID)
	if err != nil {
		return CandidatePromotion{}, err
	}
	promotion.Queued = queued
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM technology_analysis_work_items
WHERE configuration_id = $1 AND status <> 'complete'`, configurationID).Scan(&promotion.Pending); err != nil {
		return CandidatePromotion{}, fmt.Errorf("count incomplete candidate work: %w", err)
	}
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM job_body_content_versions AS body
LEFT JOIN technology_analysis_results AS result
  ON result.body_content_version_id = body.id
 AND result.configuration_id = $1
 AND result.status = 'complete'
WHERE result.id IS NULL`, configuration.ID).Scan(&promotion.MissingResults); err != nil {
		return CandidatePromotion{}, fmt.Errorf("count missing candidate analysis results: %w", err)
	}
	if promotion.Pending != 0 || promotion.MissingResults != 0 {
		if err := appendConfigurationAuditTx(ctx, tx, configurationID, "candidate_not_ready", map[string]int{
			"queued": queued, "pending": promotion.Pending, "missing_results": promotion.MissingResults,
		}); err != nil {
			return CandidatePromotion{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return CandidatePromotion{}, fmt.Errorf("commit candidate catch-up work: %w", err)
		}
		return promotion, ErrCandidateNotReady
	}
	if previousConfigurationID != nil && *previousConfigurationID != configurationID {
		if _, err := tx.Exec(ctx, `
UPDATE technology_analysis_configurations
SET status = 'superseded'
WHERE id = $1 AND status = 'published'`, *previousConfigurationID); err != nil {
			return CandidatePromotion{}, fmt.Errorf("supersede previous analysis configuration: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
UPDATE technology_analysis_configurations
SET status = 'published', published_at = now(), failed_at = NULL, safe_failure_code = NULL
WHERE id = $1`, configurationID); err != nil {
		return CandidatePromotion{}, fmt.Errorf("publish candidate analysis configuration: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE technology_analysis_publication_state
SET configuration_id = $1, updated_at = now()
WHERE singleton = true`, configurationID); err != nil {
		return CandidatePromotion{}, fmt.Errorf("advance analysis publication pointer: %w", err)
	}
	if err := appendConfigurationAuditTx(ctx, tx, configurationID, "candidate_published", map[string]int{"queued": queued}); err != nil {
		return CandidatePromotion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CandidatePromotion{}, fmt.Errorf("commit candidate promotion: %w", err)
	}
	promotion.Published = true
	return promotion, nil
}

// FailCandidateConfiguration is an explicit controlled decision. Ordinary
// work failures stay pending for retry and do not discard a potentially
// recoverable candidate. The fixed code prevents untrusted exception text
// from being persisted as an operational state.
func (repository *Repository) FailCandidateConfiguration(ctx context.Context, configurationID, safeFailureCode string) error {
	if configurationID == "" || !validCandidateFailureCode(safeFailureCode) {
		return fmt.Errorf("candidate failure input is invalid")
	}
	release, acquired, err := repository.TryAcquireConfigurationLock(ctx, configurationID)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrConfigurationBusy
	}
	defer release()
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin candidate failure update: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := ensurePublicationState(ctx, tx); err != nil {
		return err
	}
	var currentConfigurationID *string
	if err := tx.QueryRow(ctx, `
SELECT configuration_id::text
FROM technology_analysis_publication_state
WHERE singleton = true
FOR UPDATE`).Scan(&currentConfigurationID); err != nil {
		return fmt.Errorf("lock analysis publication state for candidate failure: %w", err)
	}
	if currentConfigurationID != nil && *currentConfigurationID == configurationID {
		return fmt.Errorf("published analysis configuration cannot be failed as a candidate")
	}
	command, err := tx.Exec(ctx, `
UPDATE technology_analysis_configurations
SET status = 'failed', failed_at = now(), safe_failure_code = $2
WHERE id = $1 AND status = 'candidate'`, configurationID, safeFailureCode)
	if err != nil {
		return fmt.Errorf("mark candidate analysis configuration failed: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrCandidateFailed
	}
	if err := appendConfigurationAuditTx(ctx, tx, configurationID, "candidate_failed", map[string]int{}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit candidate failure update: %w", err)
	}
	return nil
}

func ensurePublicationState(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `
INSERT INTO technology_analysis_publication_state (singleton)
VALUES (true)
ON CONFLICT (singleton) DO NOTHING`); err != nil {
		return fmt.Errorf("ensure analysis publication state: %w", err)
	}
	return nil
}

func enqueueAllBodyVersions(ctx context.Context, tx pgx.Tx, configurationID string) (int, error) {
	command, err := tx.Exec(ctx, `
INSERT INTO technology_analysis_work_items (
    configuration_id, body_content_version_id, trigger
)
SELECT $1, body.id, 'reanalysis'
FROM job_body_content_versions AS body
ON CONFLICT (configuration_id, body_content_version_id) DO NOTHING`, configurationID)
	if err != nil {
		return 0, fmt.Errorf("enqueue historical body versions: %w", err)
	}
	return int(command.RowsAffected()), nil
}

func appendConfigurationAuditTx(ctx context.Context, tx pgx.Tx, configurationID, code string, diagnostics map[string]int) error {
	severity, message, encoded, err := safeAnalysisAudit(code, diagnostics)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO technology_analysis_audit_events (
    configuration_id, severity, code, message, diagnostics
) VALUES ($1, $2, $3, $4, $5)`, configurationID, severity, code, message, encoded); err != nil {
		return fmt.Errorf("append candidate configuration audit: %w", err)
	}
	return nil
}

func validCandidateFailureCode(code string) bool {
	return code == "analysis_work_failed" || code == "release_validation_failed" || code == "operator_cancelled"
}
