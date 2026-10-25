package jdanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var (
	ErrAIConfigurationBusy  = errors.New("another AI semantic analysis process is active for this configuration")
	ErrAISemanticWorkFailed = errors.New("one or more AI semantic analysis work items failed safely")
)

// AISemanticProcessor consumes only the durable AI work ledger. Its model
// client receives a transient, CVM-resident input and never receives a source
// URL, COS object, metadata record, or complete dictionary.
type AISemanticProcessor struct {
	Repository                   *Repository
	Runner                       AIModelAttemptRunner
	RequirePersistentTrialBudget bool
	// OutputAudit is supplied only by the fixed candidate/comparison trial
	// runtime. It is intentionally absent from published analysis.
	OutputAudit        *aiOutputAuditStore
	RequireOutputAudit bool
}

// AIProcessSummary contains no source or model content. It is safe for a
// fixed operation's status output and audit accounting.
type AIProcessSummary struct {
	RunID     string
	Claimed   int
	Completed int
	Failed    int
}

type aiProcessingConfiguration struct {
	ID     string
	Model  string
	Status string
}

type aiAnalysisWork struct {
	AttemptCount         int
	BodyContentVersionID string
	ID                   string
}

// TryAcquireAIConfigurationLock serializes one-shot semantic workers per
// immutable AI configuration. It intentionally uses a separate advisory-lock
// namespace from deterministic technology analysis.
func (repository *Repository) TryAcquireAIConfigurationLock(ctx context.Context, configurationID string) (release func(), acquired bool, err error) {
	if configurationID == "" {
		return nil, false, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	connection, err := repository.pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire AI analysis lock connection: %w", err)
	}
	var locked bool
	err = connection.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1, 0))`, "ai-semantic-analysis:"+configurationID).Scan(&locked)
	if err != nil {
		connection.Release()
		return nil, false, fmt.Errorf("acquire AI analysis configuration lock: %w", err)
	}
	if !locked {
		connection.Release()
		return func() {}, false, nil
	}
	return func() {
		_, _ = connection.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, "ai-semantic-analysis:"+configurationID)
		connection.Release()
	}, true, nil
}

// ProcessPending executes durable work under one explicit AI configuration.
// It never reaches source synchronization, and safe model failures leave the
// corresponding result as not_analyzed instead of inventing a zero value.
func (processor AISemanticProcessor) ProcessPending(ctx context.Context, configurationID string) (AIProcessSummary, error) {
	if processor.Repository == nil || configurationID == "" || processor.Runner.Client == nil {
		return AIProcessSummary{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	release, acquired, err := processor.Repository.TryAcquireAIConfigurationLock(ctx, configurationID)
	if err != nil {
		return AIProcessSummary{}, err
	}
	if !acquired {
		return AIProcessSummary{}, ErrAIConfigurationBusy
	}
	defer release()

	configuration, err := processor.Repository.loadAIProcessingConfiguration(ctx, configurationID)
	if err != nil {
		return AIProcessSummary{}, err
	}
	recoveredFailedWorkCount, err := processor.Repository.requeueInterruptedAIWork(ctx, configuration.ID)
	if err != nil {
		return AIProcessSummary{}, err
	}
	runID, err := processor.Repository.createAIAnalysisRun(ctx, configuration)
	if err != nil {
		return AIProcessSummary{}, err
	}
	summary := AIProcessSummary{RunID: runID, Failed: recoveredFailedWorkCount}
	if err := processor.Repository.RecordAISafeAuditEvent(ctx, AISafeAuditEventInput{
		AIConfigurationID: configuration.ID,
		RunID:             runID,
		Code:              AIAuditWorkQueued,
	}); err != nil {
		return summary, err
	}

	var lastFailureCode AISafeErrorCode
	if recoveredFailedWorkCount > 0 {
		lastFailureCode = AISafeErrorProviderTimeout
	}
	for {
		work, found, err := processor.Repository.claimNextAIWork(ctx, configuration.ID, runID)
		if err != nil {
			_ = processor.Repository.finishAIAnalysisRun(context.Background(), runID, summary, AISafeErrorConfigurationInvalid)
			return summary, err
		}
		if !found {
			break
		}
		summary.Claimed++
		if err := processor.processWork(ctx, configuration, runID, work); err != nil {
			summary.Failed++
			code := safeAIErrorCode(err)
			lastFailureCode = code
			_ = processor.Repository.failAIWork(context.Background(), configuration.ID, work, code)
			_ = processor.Repository.RecordAISafeAuditEvent(context.Background(), AISafeAuditEventInput{
				AIConfigurationID: configuration.ID,
				RunID:             runID,
				WorkItemID:        work.ID,
				Severity:          AIAuditError,
				Code:              AIAuditSafeFailureRecorded,
				SafeErrorCode:     code,
				AttemptCount:      work.AttemptCount,
			})
			continue
		}
		summary.Completed++
	}
	if summary.Failed > 0 {
		if err := processor.Repository.finishAIAnalysisRun(ctx, runID, summary, lastFailureCode); err != nil {
			return summary, err
		}
		return summary, ErrAISemanticWorkFailed
	}
	if err := processor.Repository.finishAIAnalysisRun(ctx, runID, summary, ""); err != nil {
		return summary, err
	}
	return summary, nil
}

func (processor AISemanticProcessor) processWork(ctx context.Context, configuration aiProcessingConfiguration, runID string, work aiAnalysisWork) error {
	requestID := deterministicAIProviderRequestID(work.ID, work.AttemptCount+1)
	input, err := processor.Repository.BuildAIModelInput(ctx, requestID, configuration.ID, work.BodyContentVersionID)
	if err != nil {
		return err
	}
	runner := processor.Runner
	runner.InitialAttemptCount = work.AttemptCount
	if processor.RequirePersistentTrialBudget {
		budget, err := processor.Repository.NewPersistentAITrialBudget(ctx, configuration.ID)
		if err != nil {
			return err
		}
		runner.PersistentTrialBudget = budget
		runner.PersistentTrialWorkItem = work.ID
	}
	var auditReservation *AIOutputAuditReservation
	if processor.requiresOutputAudit(configuration) {
		if processor.OutputAudit == nil {
			return NewSafeAIError(AISafeErrorOutputAuditCapture, nil)
		}
		var err error
		auditReservation, err = processor.OutputAudit.Reserve()
		if err != nil {
			return NewSafeAIError(AISafeErrorOutputAuditCapture, err)
		}
		defer func() { _ = auditReservation.Release() }()
	}
	response, attempts, executeErr := runner.Execute(ctx, work.ID, input)
	if executeErr != nil {
		if err := processor.Repository.recordAIAttempts(ctx, configuration.ID, runID, work, attempts, response, false); err != nil {
			return err
		}
		return executeErr
	}
	if auditReservation != nil {
		if len(attempts) == 0 {
			return NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		if _, err := auditReservation.Capture(AIOutputAuditIdentity{
			ConfigurationID:      configuration.ID,
			WorkItemID:           work.ID,
			AttemptNumber:        attempts[len(attempts)-1].Number,
			BodyContentVersionID: work.BodyContentVersionID,
		}, response.Content); err != nil {
			if recordErr := processor.Repository.recordAIAttempts(ctx, configuration.ID, runID, work, attempts, response, true); recordErr != nil {
				return recordErr
			}
			return NewSafeAIError(AISafeErrorOutputAuditCapture, err)
		}
	}
	if err := processor.Repository.recordAIAttempts(ctx, configuration.ID, runID, work, attempts, response, true); err != nil {
		return err
	}
	projection, err := input.ValidateAndMapResponse(response.Content)
	if err != nil {
		return err
	}
	if err := processor.Repository.persistAndCompleteAIWork(ctx, configuration.ID, work.ID, work.BodyContentVersionID, input, projection); err != nil {
		return err
	}
	return processor.Repository.RecordAISafeAuditEvent(ctx, AISafeAuditEventInput{
		AIConfigurationID: configuration.ID,
		RunID:             runID,
		WorkItemID:        work.ID,
		Code:              AIAuditAttemptCompleted,
		AttemptCount:      len(attempts),
		InputTokenCount:   response.Usage.InputTokens,
		OutputTokenCount:  response.Usage.OutputTokens,
	})
}

func (processor AISemanticProcessor) requiresOutputAudit(configuration aiProcessingConfiguration) bool {
	return processor.RequireOutputAudit && (configuration.Status == "candidate" || configuration.Status == "comparison")
}

func (repository *Repository) loadAIProcessingConfiguration(ctx context.Context, configurationID string) (aiProcessingConfiguration, error) {
	var configuration aiProcessingConfiguration
	err := repository.pool.QueryRow(ctx, `
SELECT id::text, model, status::text
FROM ai_analysis_configurations
WHERE id = $1
  AND status IN ('candidate', 'comparison', 'published')`, configurationID).Scan(&configuration.ID, &configuration.Model, &configuration.Status)
	if err != nil {
		return aiProcessingConfiguration{}, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return configuration, nil
}

func (repository *Repository) requeueInterruptedAIWork(ctx context.Context, configurationID string) (int, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin interrupted AI work recovery: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := repository.recordUnfinishedReservedAITrialAttempts(ctx, tx, configurationID); err != nil {
		return 0, err
	}
	var exhaustedWorkCount int
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1
  AND status = 'running'
  AND attempt_count >= $2`, configurationID, aiModelMaximumAttempts).Scan(&exhaustedWorkCount); err != nil {
		return 0, fmt.Errorf("count exhausted interrupted AI work: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE ai_analysis_work_items
SET status = CASE WHEN attempt_count >= $2 THEN 'failed_safely'::ai_analysis_work_status ELSE 'pending'::ai_analysis_work_status END,
    completed_at = CASE WHEN attempt_count >= $2 THEN now() ELSE NULL END,
    last_run_id = NULL,
    last_safe_error_code = CASE WHEN attempt_count >= $2 THEN $3::ai_safe_error_code ELSE NULL END,
    updated_at = now()
WHERE ai_analysis_configuration_id = $1
	  AND status = 'running'`, configurationID, aiModelMaximumAttempts, AISafeErrorProviderTimeout); err != nil {
		return 0, fmt.Errorf("requeue interrupted AI work: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE ai_semantic_results AS result
SET execution_status = CASE WHEN work.status = 'failed_safely' THEN 'failed_safely'::ai_semantic_execution_status ELSE 'pending'::ai_semantic_execution_status END,
    semantic_status = 'not_analyzed',
    safe_error_code = CASE WHEN work.status = 'failed_safely' THEN $2::ai_safe_error_code ELSE NULL END,
    completed_at = CASE WHEN work.status = 'failed_safely' THEN now() ELSE NULL END
FROM ai_analysis_work_items AS work
WHERE work.ai_analysis_configuration_id = $1
  AND work.status IN ('pending', 'failed_safely')
  AND result.ai_analysis_configuration_id = work.ai_analysis_configuration_id
  AND result.body_content_version_id = work.body_content_version_id
  AND result.execution_status = 'running'`, configurationID, AISafeErrorProviderTimeout); err != nil {
		return 0, fmt.Errorf("requeue interrupted AI semantic result: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE ai_analysis_runs
SET status = 'failed_safely', completed_at = now(), safe_error_code = $2::ai_safe_error_code
WHERE ai_analysis_configuration_id = $1
  AND status = 'running'`, configurationID, AISafeErrorProviderTimeout); err != nil {
		return 0, fmt.Errorf("close interrupted AI analysis runs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit interrupted AI work recovery: %w", err)
	}
	return exhaustedWorkCount, nil
}

// recordUnfinishedReservedAITrialAttempts closes the narrow crash window
// between committing the pre-send cost reservation and recording the attempt.
// The provider may already have received that request, so recovery records the
// deterministic attempt number as an unknown-outcome timeout and consumes it
// before a later process may issue another request.
func (repository *Repository) recordUnfinishedReservedAITrialAttempts(ctx context.Context, tx pgx.Tx, configurationID string) error {
	rows, err := tx.Query(ctx, `
SELECT reservation.work_item_id::text, reservation.attempt_number, work.last_run_id::text
FROM ai_analysis_trial_budget_reservations AS reservation
JOIN ai_analysis_work_items AS work
  ON work.id = reservation.work_item_id
 AND work.ai_analysis_configuration_id = reservation.ai_analysis_configuration_id
LEFT JOIN ai_analysis_attempts AS attempt
  ON attempt.work_item_id = reservation.work_item_id
 AND attempt.attempt_number = reservation.attempt_number
WHERE reservation.ai_analysis_configuration_id = $1
  AND reservation.status = 'reserved'
  AND work.status = 'running'
  AND work.last_run_id IS NOT NULL
  AND attempt.id IS NULL
ORDER BY reservation.work_item_id, reservation.attempt_number`, configurationID)
	if err != nil {
		return fmt.Errorf("find unfinished reserved AI trial attempts: %w", err)
	}
	type unfinishedReservation struct {
		workItemID    string
		attemptNumber int
		runID         string
	}
	unfinished := make([]unfinishedReservation, 0)
	for rows.Next() {
		var reservation unfinishedReservation
		if err := rows.Scan(&reservation.workItemID, &reservation.attemptNumber, &reservation.runID); err != nil {
			rows.Close()
			return fmt.Errorf("scan unfinished reserved AI trial attempt: %w", err)
		}
		unfinished = append(unfinished, reservation)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate unfinished reserved AI trial attempts: %w", err)
	}
	rows.Close()

	for _, reservation := range unfinished {
		if _, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_attempts (
    ai_analysis_configuration_id, work_item_id, run_id, attempt_number, status,
    input_token_limit, output_token_limit, safe_error_code, provider_request_id, completed_at
) VALUES ($1, $2, $3, $4, 'failed', 10000, 2500, $5::ai_safe_error_code, $6::uuid, now())`,
			configurationID, reservation.workItemID, reservation.runID, reservation.attemptNumber,
			AISafeErrorProviderTimeout, deterministicAIProviderRequestID(reservation.workItemID, reservation.attemptNumber)); err != nil {
			return fmt.Errorf("record unfinished reserved AI trial attempt: %w", err)
		}
	}
	if len(unfinished) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
UPDATE ai_analysis_work_items AS work
SET attempt_count = GREATEST(work.attempt_count, reserved.maximum_attempt_number),
    updated_at = now()
FROM (
    SELECT work_item_id, max(attempt_number) AS maximum_attempt_number
    FROM ai_analysis_trial_budget_reservations
    WHERE ai_analysis_configuration_id = $1
    GROUP BY work_item_id
) AS reserved
WHERE work.id = reserved.work_item_id
  AND work.ai_analysis_configuration_id = $1
  AND work.status = 'running'`, configurationID); err != nil {
		return fmt.Errorf("advance recovered AI trial attempt counts: %w", err)
	}
	return nil
}

// ResumeFailedAIWork is the explicit recovery seam for work whose prior
// terminal cause has been repaired (for example, a corrected credential or
// provider outage). Ordinary processing never requeues failed_safely work;
// the three physical-attempt ceiling remains in force across a resume.
func (repository *Repository) ResumeFailedAIWork(ctx context.Context, configurationID string) (int, error) {
	if configurationID == "" {
		return 0, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin AI work resume: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `
UPDATE ai_analysis_work_items
SET status = 'pending', completed_at = NULL, last_run_id = NULL,
    last_safe_error_code = NULL, updated_at = now()
WHERE ai_analysis_configuration_id = $1
  AND status = 'failed_safely'
  AND attempt_count < 3
RETURNING body_content_version_id::text`, configurationID)
	if err != nil {
		return 0, fmt.Errorf("resume failed AI work: %w", err)
	}
	bodyContentVersionIDs := make([]string, 0)
	for rows.Next() {
		var bodyContentVersionID string
		if err := rows.Scan(&bodyContentVersionID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan resumed AI work: %w", err)
		}
		bodyContentVersionIDs = append(bodyContentVersionIDs, bodyContentVersionID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate resumed AI work: %w", err)
	}
	rows.Close()
	for _, bodyContentVersionID := range bodyContentVersionIDs {
		command, err := tx.Exec(ctx, `
UPDATE ai_semantic_results
SET execution_status = 'pending', semantic_status = 'not_analyzed',
    safe_error_code = NULL, completed_at = NULL
WHERE ai_analysis_configuration_id = $1
  AND body_content_version_id = $2
  AND execution_status = 'failed_safely'`, configurationID, bodyContentVersionID)
		if err != nil || command.RowsAffected() != 1 {
			return 0, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit AI work resume: %w", err)
	}
	return len(bodyContentVersionIDs), nil
}

func (repository *Repository) createAIAnalysisRun(ctx context.Context, configuration aiProcessingConfiguration) (string, error) {
	trigger := "trial"
	if configuration.Status == "published" {
		trigger = "new_body"
	}
	var runID string
	err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_analysis_runs (ai_analysis_configuration_id, trigger)
VALUES ($1, $2::ai_analysis_trigger)
RETURNING id::text`, configuration.ID, trigger).Scan(&runID)
	if err != nil {
		return "", fmt.Errorf("create AI analysis run: %w", err)
	}
	return runID, nil
}

func (repository *Repository) claimNextAIWork(ctx context.Context, configurationID, runID string) (aiAnalysisWork, bool, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return aiAnalysisWork{}, false, fmt.Errorf("begin AI work claim: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var work aiAnalysisWork
	err = tx.QueryRow(ctx, `
WITH next_item AS (
    SELECT work.id
    FROM ai_analysis_work_items AS work
    JOIN ai_analysis_configurations AS ai_configuration
      ON ai_configuration.id = work.ai_analysis_configuration_id
    JOIN technology_analysis_results AS deterministic_result
      ON deterministic_result.body_content_version_id = work.body_content_version_id
     AND deterministic_result.configuration_id = ai_configuration.technology_analysis_configuration_id
     AND deterministic_result.status = 'complete'
    WHERE work.ai_analysis_configuration_id = $1
      AND work.status = 'pending'
      AND work.attempt_count < 3
    ORDER BY work.queued_at, work.id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE ai_analysis_work_items AS item
SET status = 'running', last_run_id = $2, last_safe_error_code = NULL, updated_at = now()
FROM next_item
WHERE item.id = next_item.id
RETURNING item.id::text, item.body_content_version_id::text, item.attempt_count`, configurationID, runID).Scan(&work.ID, &work.BodyContentVersionID, &work.AttemptCount)
	if err == pgx.ErrNoRows {
		if err := tx.Commit(ctx); err != nil {
			return aiAnalysisWork{}, false, fmt.Errorf("commit empty AI work claim: %w", err)
		}
		return aiAnalysisWork{}, false, nil
	}
	if err != nil {
		return aiAnalysisWork{}, false, fmt.Errorf("claim AI work: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE ai_semantic_results
SET execution_status = 'running', semantic_status = 'not_analyzed', safe_error_code = NULL, completed_at = NULL
WHERE ai_analysis_configuration_id = $1
  AND body_content_version_id = $2
  AND execution_status = 'pending'`, configurationID, work.BodyContentVersionID); err != nil {
		return aiAnalysisWork{}, false, fmt.Errorf("mark AI semantic result running: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return aiAnalysisWork{}, false, fmt.Errorf("commit AI work claim: %w", err)
	}
	return work, true, nil
}

func (repository *Repository) recordAIAttempts(ctx context.Context, configurationID, runID string, work aiAnalysisWork, attempts []AIModelAttempt, response AIModelResponse, succeeded bool) error {
	if len(attempts) == 0 {
		return nil
	}
	if work.AttemptCount+len(attempts) > aiModelMaximumAttempts {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin AI attempt recording: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	for index, attempt := range attempts {
		number := attempt.Number
		if number != work.AttemptCount+index+1 || number < 1 || number > aiModelMaximumAttempts {
			return NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		status := "failed"
		code := attempt.SafeErrorCode
		if succeeded && index == len(attempts)-1 && code == "" {
			status = "succeeded"
		}
		if code == "" && status == "failed" {
			code = AISafeErrorConfigurationInvalid
		}
		var attemptID string
		if err := tx.QueryRow(ctx, `
INSERT INTO ai_analysis_attempts (
    ai_analysis_configuration_id, work_item_id, run_id, attempt_number, status,
    input_token_limit, output_token_limit, safe_error_code, provider_request_id, completed_at
) VALUES ($1, $2, $3, $4, $5::ai_analysis_attempt_status,
          10000, 2500, NULLIF($6, '')::ai_safe_error_code, $7::uuid, now())
RETURNING id::text`, configurationID, work.ID, runID, number, status, code,
			deterministicAIProviderRequestID(work.ID, number)).Scan(&attemptID); err != nil {
			return fmt.Errorf("record AI attempt: %w", err)
		}
		if status == "succeeded" {
			if err := repository.recordPersistentAIUsageIfPresent(ctx, tx, configurationID, work.ID, number, attemptID, response.Usage); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(ctx, `
UPDATE ai_analysis_work_items
SET attempt_count = attempt_count + $2, updated_at = now()
WHERE id = $1
  AND status = 'running'`, work.ID, len(attempts)); err != nil {
		return fmt.Errorf("update AI work attempt count: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit AI attempt recording: %w", err)
	}
	return nil
}

// recordPersistentAIUsageIfPresent writes returned usage only when the
// pre-send durable reservation has already been settled. Legacy synthetic
// processor tests without the controlled trial gate retain their zero-charge
// behavior; a partially durable successful attempt is rejected rather than
// silently recording made-up accounting.
func (repository *Repository) recordPersistentAIUsageIfPresent(ctx context.Context, tx pgx.Tx, configurationID, workItemID string, attemptNumber int, attemptID string, responseUsage AIModelUsage) error {
	var reservationExists bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM ai_analysis_trial_budget_reservations
    WHERE ai_analysis_configuration_id = $1
      AND work_item_id = $2
      AND attempt_number = $3
)`, configurationID, workItemID, attemptNumber).Scan(&reservationExists); err != nil {
		return fmt.Errorf("inspect persistent AI trial reservation: %w", err)
	}
	if !reservationExists {
		if _, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_usage_records (
    attempt_id, input_tokens, output_tokens, estimated_charge_rmb, actual_charge_rmb
) VALUES ($1, $2, $3, 0, 0)`, attemptID, responseUsage.InputTokens, responseUsage.OutputTokens); err != nil {
			return fmt.Errorf("record legacy AI attempt usage: %w", err)
		}
		return nil
	}
	var inputTokens, outputTokens int
	var estimatedChargeNanoRMB, actualChargeNanoRMB int64
	err := tx.QueryRow(ctx, `
SELECT actual_input_tokens, actual_output_tokens,
       reserved_charge_nano_rmb, actual_charge_nano_rmb
FROM ai_analysis_trial_budget_reservations
WHERE ai_analysis_configuration_id = $1
  AND work_item_id = $2
  AND attempt_number = $3
  AND status = 'settled'`, configurationID, workItemID, attemptNumber).Scan(
		&inputTokens, &outputTokens, &estimatedChargeNanoRMB, &actualChargeNanoRMB,
	)
	if err != nil || inputTokens != responseUsage.InputTokens || outputTokens != responseUsage.OutputTokens {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_usage_records (
    attempt_id, input_tokens, output_tokens, estimated_charge_rmb, actual_charge_rmb
) VALUES ($1, $2, $3, $4::numeric / 1000000000, $5::numeric / 1000000000)`,
		attemptID, inputTokens, outputTokens, estimatedChargeNanoRMB, actualChargeNanoRMB); err != nil {
		return fmt.Errorf("record persistent AI attempt usage: %w", err)
	}
	return nil
}

func (repository *Repository) completeAIWork(ctx context.Context, configurationID, workID, bodyContentVersionID, semanticStatus string) error {
	if !validCompletedAISemanticStatus(semanticStatus) {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin AI work completion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	command, err := tx.Exec(ctx, `
UPDATE ai_semantic_results
SET execution_status = 'complete', semantic_status = $3::ai_semantic_value_status,
    safe_error_code = NULL, completed_at = now()
WHERE ai_analysis_configuration_id = $1
  AND body_content_version_id = $2
  AND execution_status = 'running'`, configurationID, bodyContentVersionID, semanticStatus)
	if err != nil || command.RowsAffected() != 1 {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	command, err = tx.Exec(ctx, `
UPDATE ai_analysis_work_items
SET status = 'complete', completed_at = now(), last_safe_error_code = NULL, updated_at = now()
WHERE id = $1
  AND ai_analysis_configuration_id = $2
  AND status = 'running'`, workID, configurationID)
	if err != nil || command.RowsAffected() != 1 {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit AI work completion: %w", err)
	}
	return nil
}

func (repository *Repository) failAIWork(ctx context.Context, configurationID string, work aiAnalysisWork, code AISafeErrorCode) error {
	if !code.valid() {
		code = AISafeErrorConfigurationInvalid
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin AI work failure: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `
UPDATE ai_semantic_results
SET execution_status = 'failed_safely', semantic_status = 'not_analyzed',
    safe_error_code = $3::ai_safe_error_code, completed_at = now()
WHERE ai_analysis_configuration_id = $1
  AND body_content_version_id = $2
  AND execution_status = 'running'`, configurationID, work.BodyContentVersionID, code); err != nil {
		return fmt.Errorf("fail AI semantic result: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE ai_analysis_work_items
SET status = 'failed_safely', completed_at = now(), last_safe_error_code = $2::ai_safe_error_code, updated_at = now()
WHERE id = $1
  AND status = 'running'`, work.ID, code); err != nil {
		return fmt.Errorf("fail AI work: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit AI work failure: %w", err)
	}
	return nil
}

func (repository *Repository) finishAIAnalysisRun(ctx context.Context, runID string, summary AIProcessSummary, code AISafeErrorCode) error {
	status := "complete"
	if summary.Failed > 0 || code != "" {
		status = "failed_safely"
		if !code.valid() {
			code = AISafeErrorConfigurationInvalid
		}
	}
	command, err := repository.pool.Exec(ctx, `
UPDATE ai_analysis_runs
SET status = $2::ai_analysis_run_status, completed_at = now(), claimed_count = $3,
    completed_count = $4, failed_count = $5, safe_error_code = NULLIF($6, '')::ai_safe_error_code
WHERE id = $1`, runID, status, summary.Claimed, summary.Completed, summary.Failed, code)
	if err != nil || command.RowsAffected() != 1 {
		return fmt.Errorf("finish AI analysis run: %w", err)
	}
	return nil
}

func semanticStatusForProjection(projection AISemanticProjection) string {
	if projection.Experience.Status == "uncertain" || projection.MinimumEducation.Status == "uncertain" {
		return "uncertain"
	}
	if projection.Experience.Status == "configured" || projection.MinimumEducation.Status == "configured" ||
		len(projection.RequirementGroups) > 0 || len(projection.CandidateMentions) > 0 {
		return "configured"
	}
	return "not_configured"
}

func safeAIErrorCode(err error) AISafeErrorCode {
	var safe safeAIError
	if errors.As(err, &safe) {
		return safe.code
	}
	var provider AIProviderFailure
	if errors.As(err, &provider) && provider.Code.valid() {
		return provider.Code
	}
	return AISafeErrorConfigurationInvalid
}

func deterministicAIProviderRequestID(workID string, number int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("ai-semantic-attempt-v1|%s|%d", workID, number)))
	encoded := hex.EncodeToString(sum[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}
