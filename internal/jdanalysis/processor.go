package jdanalysis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrConfigurationBusy        = errors.New("another analysis process is active for this configuration")
	ErrAnalysisWorkFailed       = errors.New("one or more analysis work items failed safely")
	ErrNoPublishedConfiguration = errors.New("no published analysis configuration is available")
)

type AnalysisTrigger string

const (
	TriggerNewBody     AnalysisTrigger = "new_body"
	TriggerReanalysis  AnalysisTrigger = "reanalysis"
	TriggerManualRetry AnalysisTrigger = "manual_retry"
)

func (trigger AnalysisTrigger) Valid() bool {
	return trigger == TriggerNewBody || trigger == TriggerReanalysis || trigger == TriggerManualRetry
}

// EnqueueBody makes a durable, idempotent promise to analyze a body under one
// configuration. It does not read source APIs or start parsing work.
func (repository *Repository) EnqueueBody(ctx context.Context, configurationID, bodyContentVersionID string, trigger AnalysisTrigger) (bool, error) {
	if configurationID == "" || bodyContentVersionID == "" || !trigger.Valid() {
		return false, fmt.Errorf("analysis work item is incomplete")
	}
	command, err := repository.pool.Exec(ctx, `
INSERT INTO technology_analysis_work_items (configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, $3)
ON CONFLICT (configuration_id, body_content_version_id) DO NOTHING`, configurationID, bodyContentVersionID, trigger)
	if err != nil {
		return false, fmt.Errorf("enqueue analysis body: %w", err)
	}
	return command.RowsAffected() == 1, nil
}

// TryAcquireConfigurationLock holds an advisory lock on a dedicated session.
// It prevents two one-shot analyzers from consuming the same configuration at
// once without requiring a generic distributed-workflow service.
func (repository *Repository) TryAcquireConfigurationLock(ctx context.Context, configurationID string) (release func(), acquired bool, err error) {
	connection, err := repository.pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire analysis lock connection: %w", err)
	}
	var locked bool
	err = connection.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1, 0))`, "technology-analysis:"+configurationID).Scan(&locked)
	if err != nil {
		connection.Release()
		return nil, false, fmt.Errorf("acquire analysis configuration lock: %w", err)
	}
	if !locked {
		connection.Release()
		return func() {}, false, nil
	}
	return func() {
		_, _ = connection.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, "technology-analysis:"+configurationID)
		connection.Release()
	}, true, nil
}

type ResultPublisher func(context.Context, ResultIdentity, []MentionInput) (PublishResult, error)

// Processor is deliberately a PostgreSQL-only consumer. Its optional Publish
// hook exists for synthetic fault tests; production uses Repository directly.
type Processor struct {
	Repository *Repository
	Publish    ResultPublisher
}

type ProcessSummary struct {
	RunID     string
	Claimed   int
	Completed int
	Failed    int
}

// PublishedConfigurationID resolves the sole released configuration. Worker
// analysis has no free-form configuration argument, so normal operations can
// never accidentally process an unreviewed candidate configuration.
func (repository *Repository) PublishedConfigurationID(ctx context.Context) (string, error) {
	var configurationID string
	err := repository.pool.QueryRow(ctx, `
SELECT configuration.id::text
FROM technology_analysis_publication_state AS publication
JOIN technology_analysis_configurations AS configuration
  ON configuration.id = publication.configuration_id
WHERE publication.singleton = true AND configuration.status = 'published'`).Scan(&configurationID)
	if err == pgx.ErrNoRows {
		return "", ErrNoPublishedConfiguration
	}
	if err != nil {
		return "", fmt.Errorf("find published analysis configuration: %w", err)
	}
	return configurationID, nil
}

func (processor Processor) ProcessPending(ctx context.Context, configurationID string) (ProcessSummary, error) {
	if processor.Repository == nil || configurationID == "" {
		return ProcessSummary{}, fmt.Errorf("analysis processor is incomplete")
	}
	release, acquired, err := processor.Repository.TryAcquireConfigurationLock(ctx, configurationID)
	if err != nil {
		return ProcessSummary{}, err
	}
	if !acquired {
		return ProcessSummary{}, ErrConfigurationBusy
	}
	defer release()

	configuration, err := processor.Repository.loadConfiguration(ctx, configurationID)
	if err != nil {
		return ProcessSummary{}, err
	}
	if err := processor.Repository.requeueInterruptedWork(ctx, configuration.ID); err != nil {
		return ProcessSummary{}, err
	}
	runID, err := processor.Repository.createAnalysisRun(ctx, configuration.ID, TriggerNewBody)
	if err != nil {
		return ProcessSummary{}, err
	}
	summary := ProcessSummary{RunID: runID}
	if err := processor.Repository.appendAudit(ctx, configuration.ID, runID, "", "analysis_run_started", map[string]int{"claimed": 0}); err != nil {
		return summary, err
	}

	rules, err := processor.Repository.loadExecutableRules(ctx, configuration)
	if err != nil {
		_ = processor.Repository.finishAnalysisRun(context.Background(), runID, summary, "analysis_rule_loading_failed")
		return summary, err
	}
	matcherRules := make([]DictionaryRule, 0, len(rules))
	rulesByKey := make(map[string]databaseRule, len(rules))
	for _, rule := range rules {
		matcherRules = append(matcherRules, rule.DictionaryRule)
		rulesByKey[rule.RuleKey] = rule
	}
	matcher, err := NewMatcher(matcherRules)
	if err != nil {
		_ = processor.Repository.finishAnalysisRun(context.Background(), runID, summary, "analysis_rule_validation_failed")
		return summary, err
	}

	for {
		work, found, err := processor.Repository.claimNextWork(ctx, configuration.ID, runID)
		if err != nil {
			_ = processor.Repository.finishAnalysisRun(context.Background(), runID, summary, "analysis_queue_claim_failed")
			return summary, err
		}
		if !found {
			break
		}
		summary.Claimed++
		if err := processor.processWork(ctx, configuration, matcher, rulesByKey, work); err != nil {
			summary.Failed++
			_ = processor.Repository.returnWorkToPending(context.Background(), work.ID, "analysis_result_failed")
			_ = processor.Repository.appendAudit(context.Background(), configuration.ID, runID, work.ID, "analysis_work_failed", map[string]int{"attempt": work.AttemptCount})
			continue
		}
		summary.Completed++
		if err := processor.Repository.completeWork(ctx, work.ID); err != nil {
			summary.Failed++
			_ = processor.Repository.returnWorkToPending(context.Background(), work.ID, "analysis_work_completion_failed")
			_ = processor.Repository.appendAudit(context.Background(), configuration.ID, runID, work.ID, "analysis_work_completion_failed", map[string]int{"attempt": work.AttemptCount})
		}
	}
	if summary.Failed > 0 {
		if err := processor.Repository.finishAnalysisRun(ctx, runID, summary, "analysis_work_failed"); err != nil {
			return summary, err
		}
		return summary, ErrAnalysisWorkFailed
	}
	if err := processor.Repository.finishAnalysisRun(ctx, runID, summary, ""); err != nil {
		return summary, err
	}
	return summary, nil
}

func (processor Processor) processWork(ctx context.Context, configuration analysisConfiguration, matcher Matcher, rulesByKey map[string]databaseRule, work analysisWork) error {
	body, err := processor.Repository.loadBodyContentFromPool(ctx, work.BodyContentVersionID)
	if err != nil {
		return err
	}
	matches, err := matcher.MatchBodyFields(map[string]string{
		SourceFieldDescription: body.description,
		SourceFieldJobRequire:  body.jobRequire,
		SourceFieldAddition:    body.addition,
	})
	if err != nil {
		return err
	}
	mentions := make(map[string]*MentionInput)
	for _, match := range matches {
		rule, exists := rulesByKey[match.RuleKey]
		if !exists {
			return fmt.Errorf("matched dictionary rule is unavailable")
		}
		text, _, ok := body.field(match.SourceField)
		if !ok {
			return fmt.Errorf("matched source field is unavailable")
		}
		interval, err := match.CodePointRange(text)
		if err != nil {
			return err
		}
		mention := mentions[rule.TechnologyIdentityID]
		if mention == nil {
			mention = &MentionInput{TechnologyIdentityID: rule.TechnologyIdentityID}
			mentions[rule.TechnologyIdentityID] = mention
		}
		mention.Evidence = append(mention.Evidence, EvidenceInput{
			SourceField: match.SourceField, RuleID: rule.ID, Start: interval.Start, End: interval.End,
		})
	}
	resultMentions := make([]MentionInput, 0, len(mentions))
	for _, mention := range mentions {
		resultMentions = append(resultMentions, *mention)
	}
	identity := ResultIdentity{
		BodyContentVersionID: work.BodyContentVersionID,
		ConfigurationID:      configuration.ID,
	}
	publisher := processor.Publish
	if publisher == nil {
		publisher = processor.Repository.PublishCompleteResult
	}
	_, err = publisher(ctx, identity, resultMentions)
	return err
}

type analysisConfiguration struct {
	ID                  string
	ParserVersion       string
	DictionaryVersionID string
}

type databaseRule struct {
	DictionaryRule
	ID                   string
	TechnologyIdentityID string
}

type analysisWork struct {
	ID                   string
	BodyContentVersionID string
	AttemptCount         int
}

func (repository *Repository) loadConfiguration(ctx context.Context, configurationID string) (analysisConfiguration, error) {
	var configuration analysisConfiguration
	err := repository.pool.QueryRow(ctx, `
SELECT id::text, parser_version, dictionary_version_id::text
FROM technology_analysis_configurations
WHERE id = $1`, configurationID).Scan(&configuration.ID, &configuration.ParserVersion, &configuration.DictionaryVersionID)
	if err != nil {
		return analysisConfiguration{}, fmt.Errorf("load analysis configuration: %w", err)
	}
	return configuration, nil
}

func (repository *Repository) loadExecutableRules(ctx context.Context, configuration analysisConfiguration) ([]databaseRule, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT rule.id::text, identity.id::text, identity.technology_key, identity.canonical_name,
       identity.technology_kind, rule.rule_key, rule.alias, rule.case_mode,
       rule.boundary_mode, rule.overlap_priority, rule.rule_summary,
       rule.approved_by, rule.approved_at
FROM technology_analysis_configuration_rules AS snapshot
JOIN technology_dictionary_rules AS rule ON rule.id = snapshot.dictionary_rule_id
JOIN technology_identities AS identity ON identity.id = rule.technology_identity_id
WHERE snapshot.configuration_id = $1
ORDER BY rule.rule_key`, configuration.ID)
	if err != nil {
		return nil, fmt.Errorf("load enabled dictionary rules: %w", err)
	}
	defer rows.Close()
	rules := make([]databaseRule, 0)
	for rows.Next() {
		var rule databaseRule
		var reviewer string
		var decidedAt time.Time
		if err := rows.Scan(
			&rule.ID, &rule.TechnologyIdentityID, &rule.TechnologyKey, &rule.CanonicalName,
			&rule.TechnologyKind, &rule.RuleKey, &rule.Alias, &rule.CaseMode,
			&rule.BoundaryMode, &rule.OverlapPriority, &rule.Summary, &reviewer, &decidedAt); err != nil {
			return nil, fmt.Errorf("scan enabled dictionary rule: %w", err)
		}
		rule.State = RuleEnabled
		rule.Approval = &Approval{Reviewer: reviewer, Decided: decidedAt, Rationale: "recorded dictionary approval"}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled dictionary rules: %w", err)
	}
	return rules, nil
}

func (repository *Repository) loadBodyContentFromPool(ctx context.Context, bodyContentVersionID string) (bodyContent, error) {
	var body bodyContent
	err := repository.pool.QueryRow(ctx, `
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

func (repository *Repository) requeueInterruptedWork(ctx context.Context, configurationID string) error {
	_, err := repository.pool.Exec(ctx, `
UPDATE technology_analysis_work_items
SET status = 'pending', current_run_id = NULL, last_safe_error_code = 'interrupted_analysis_requeued', updated_at = now()
WHERE configuration_id = $1 AND status = 'running'`, configurationID)
	if err != nil {
		return fmt.Errorf("requeue interrupted analysis work: %w", err)
	}
	return nil
}

func (repository *Repository) createAnalysisRun(ctx context.Context, configurationID string, trigger AnalysisTrigger) (string, error) {
	var runID string
	err := repository.pool.QueryRow(ctx, `
INSERT INTO technology_analysis_runs (configuration_id, trigger)
VALUES ($1, $2)
RETURNING id::text`, configurationID, trigger).Scan(&runID)
	if err != nil {
		return "", fmt.Errorf("create analysis run: %w", err)
	}
	return runID, nil
}

func (repository *Repository) claimNextWork(ctx context.Context, configurationID, runID string) (analysisWork, bool, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return analysisWork{}, false, fmt.Errorf("begin analysis work claim: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var work analysisWork
	err = tx.QueryRow(ctx, `
WITH next_item AS (
    SELECT id
    FROM technology_analysis_work_items
    WHERE configuration_id = $1 AND status = 'pending'
      AND (last_attempt_run_id IS NULL OR last_attempt_run_id <> $2)
    ORDER BY queued_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE technology_analysis_work_items AS item
SET status = 'running', attempt_count = item.attempt_count + 1,
    current_run_id = $2, last_attempt_run_id = $2,
    last_safe_error_code = NULL, updated_at = now()
FROM next_item
WHERE item.id = next_item.id
RETURNING item.id::text, item.body_content_version_id::text, item.attempt_count`, configurationID, runID).Scan(&work.ID, &work.BodyContentVersionID, &work.AttemptCount)
	if err == pgx.ErrNoRows {
		if err := tx.Commit(ctx); err != nil {
			return analysisWork{}, false, fmt.Errorf("commit empty analysis work claim: %w", err)
		}
		return analysisWork{}, false, nil
	}
	if err != nil {
		return analysisWork{}, false, fmt.Errorf("claim analysis work: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return analysisWork{}, false, fmt.Errorf("commit analysis work claim: %w", err)
	}
	return work, true, nil
}

func (repository *Repository) completeWork(ctx context.Context, workID string) error {
	command, err := repository.pool.Exec(ctx, `
UPDATE technology_analysis_work_items
SET status = 'complete', completed_at = now(), updated_at = now(), last_safe_error_code = NULL
WHERE id = $1 AND status = 'running'`, workID)
	if err != nil {
		return fmt.Errorf("complete analysis work: %w", err)
	}
	if command.RowsAffected() != 1 {
		return fmt.Errorf("analysis work was not running at completion")
	}
	return nil
}

func (repository *Repository) returnWorkToPending(ctx context.Context, workID, safeCode string) error {
	_, err := repository.pool.Exec(ctx, `
UPDATE technology_analysis_work_items
SET status = 'pending', current_run_id = NULL, last_safe_error_code = $2, updated_at = now()
WHERE id = $1 AND status = 'running'`, workID, safeCode)
	if err != nil {
		return fmt.Errorf("return analysis work to pending: %w", err)
	}
	return nil
}

func (repository *Repository) finishAnalysisRun(ctx context.Context, runID string, summary ProcessSummary, safeFailureCode string) error {
	status := "complete"
	if safeFailureCode != "" || summary.Failed > 0 {
		status = "failed"
	}
	_, err := repository.pool.Exec(ctx, `
UPDATE technology_analysis_runs
SET status = $2, completed_at = now(), claimed_count = $3,
    completed_count = $4, failed_count = $5, safe_failure_code = NULLIF($6, '')
WHERE id = $1`, runID, status, summary.Claimed, summary.Completed, summary.Failed, safeFailureCode)
	if err != nil {
		return fmt.Errorf("finish analysis run: %w", err)
	}
	return nil
}

func (repository *Repository) appendAudit(ctx context.Context, configurationID, runID, workID, code string, diagnostics map[string]int) error {
	severity, message, encoded, err := safeAnalysisAudit(code, diagnostics)
	if err != nil {
		return err
	}
	_, err = repository.pool.Exec(ctx, `
INSERT INTO technology_analysis_audit_events (
    configuration_id, run_id, work_item_id, severity, code, message, diagnostics
) VALUES ($1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, $4, $5, $6, $7)`,
		configurationID, runID, workID, severity, code, message, encoded)
	if err != nil {
		return fmt.Errorf("append analysis audit: %w", err)
	}
	return nil
}
