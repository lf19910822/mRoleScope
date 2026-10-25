package jdanalysis

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
)

// AIOutputAuditPolicyDiagnosticCategory is a closed, non-text classification
// for replaying a captured policy rejection inside the CVM audit boundary.
// It deliberately never includes a model value, source fragment, identifier,
// filesystem path, digest, or database error.
type AIOutputAuditPolicyDiagnosticCategory string

const (
	AIOutputAuditPolicyDiagnosticExperience       AIOutputAuditPolicyDiagnosticCategory = "experience"
	AIOutputAuditPolicyDiagnosticEducation        AIOutputAuditPolicyDiagnosticCategory = "education"
	AIOutputAuditPolicyDiagnosticCandidateMention AIOutputAuditPolicyDiagnosticCategory = "candidate_mention"
	AIOutputAuditPolicyDiagnosticRequirementGroup AIOutputAuditPolicyDiagnosticCategory = "requirement_group"
	AIOutputAuditPolicyDiagnosticFinalization     AIOutputAuditPolicyDiagnosticCategory = "finalization"
	AIOutputAuditPolicyDiagnosticReplaySucceeded  AIOutputAuditPolicyDiagnosticCategory = "replay_succeeded"
	AIOutputAuditPolicyDiagnosticUnknown          AIOutputAuditPolicyDiagnosticCategory = "unknown"
)

// AIOutputAuditPolicyDiagnosticCount is a safe aggregate. It intentionally
// has no raw response, input, identifier, path, digest, source value, or
// database error field.
type AIOutputAuditPolicyDiagnosticCount struct {
	Category AIOutputAuditPolicyDiagnosticCategory
	Count    int
}

// AIOutputAuditPolicyDiagnostic is the CVM-only, offline replay summary for
// captured model responses that previously stopped at policy_rejected.
type AIOutputAuditPolicyDiagnostic struct {
	Counts         []AIOutputAuditPolicyDiagnosticCount
	ProcessedCount int
}

// aiPersistenceStage is internal execution context. It is fixed by the
// program, rather than derived from an untrusted provider response.
type aiPersistenceStage string

const (
	aiPersistenceStageExperience       aiPersistenceStage = "experience"
	aiPersistenceStageEducation        aiPersistenceStage = "education"
	aiPersistenceStageCandidateMention aiPersistenceStage = "candidate_mention"
	aiPersistenceStageRequirementGroup aiPersistenceStage = "requirement_group"
	aiPersistenceStageFinalization     aiPersistenceStage = "finalization"
)

type aiPersistenceStageError struct {
	cause error
	stage aiPersistenceStage
}

func newAIPersistenceStageError(stage aiPersistenceStage, cause error) error {
	if cause == nil {
		return nil
	}
	return aiPersistenceStageError{cause: cause, stage: stage}
}

func (err aiPersistenceStageError) Error() string {
	return "AI semantic persistence failed at a fixed stage"
}

func (err aiPersistenceStageError) Unwrap() error {
	return err.cause
}

func classifyAIOutputAuditPolicyDiagnostic(err error) AIOutputAuditPolicyDiagnosticCategory {
	if err == nil {
		return AIOutputAuditPolicyDiagnosticReplaySucceeded
	}
	var staged aiPersistenceStageError
	if !errors.As(err, &staged) {
		return AIOutputAuditPolicyDiagnosticUnknown
	}
	switch staged.stage {
	case aiPersistenceStageExperience:
		return AIOutputAuditPolicyDiagnosticExperience
	case aiPersistenceStageEducation:
		return AIOutputAuditPolicyDiagnosticEducation
	case aiPersistenceStageCandidateMention:
		return AIOutputAuditPolicyDiagnosticCandidateMention
	case aiPersistenceStageRequirementGroup:
		return AIOutputAuditPolicyDiagnosticRequirementGroup
	case aiPersistenceStageFinalization:
		return AIOutputAuditPolicyDiagnosticFinalization
	default:
		return AIOutputAuditPolicyDiagnosticUnknown
	}
}

// DiagnoseCapturedAIOutputPolicyFailures replays only captured, unreviewed
// policy failures. It uses the same parsing, mapping, identity lookups, and
// persistence statements as production, but rolls every transaction back.
// No provider request is made and no semantic result, review, ledger row, or
// audit artifact is durably changed.
func (repository *Repository) DiagnoseCapturedAIOutputPolicyFailures(ctx context.Context, store *aiOutputAuditStore, configurationIDs []string, model string) (AIOutputAuditPolicyDiagnostic, error) {
	if repository == nil || store == nil || !validAIOutputAuditConfigurationIDs(configurationIDs) || !validControlledAIReviewModel(model) {
		return AIOutputAuditPolicyDiagnostic{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	targets, err := repository.loadControlledAIOutputAuditPolicyTargets(ctx, configurationIDs, model)
	if err != nil {
		return AIOutputAuditPolicyDiagnostic{}, err
	}
	counts := make(map[AIOutputAuditPolicyDiagnosticCategory]int)
	for _, target := range targets {
		identity := AIOutputAuditIdentity{
			ConfigurationID:      target.AIConfigurationID,
			WorkItemID:           target.WorkItemID,
			AttemptNumber:        target.AttemptNumber,
			BodyContentVersionID: target.BodyContentVersionID,
		}
		_, rawOutput, err := store.Load(identity)
		if err != nil {
			return AIOutputAuditPolicyDiagnostic{}, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
		}
		input, err := repository.BuildAIModelInput(ctx, deterministicAIProviderRequestID(target.WorkItemID, target.AttemptNumber), target.AIConfigurationID, target.BodyContentVersionID)
		if err != nil {
			return AIOutputAuditPolicyDiagnostic{}, err
		}
		projection, err := input.ValidateAndMapResponse(rawOutput)
		if err == nil {
			err = repository.replayAIPersistenceForOutputAuditDiagnostic(ctx, target, input, projection)
		}
		counts[classifyAIOutputAuditPolicyDiagnostic(err)]++
	}
	return newAIOutputAuditPolicyDiagnostic(counts, len(targets)), nil
}

func (repository *Repository) loadControlledAIOutputAuditPolicyTargets(ctx context.Context, configurationIDs []string, model string) ([]controlledAIOutputAuditTarget, error) {
	if len(configurationIDs) == 0 {
		return []controlledAIOutputAuditTarget{}, nil
	}
	rows, err := repository.pool.Query(ctx, `
SELECT configuration.id::text,
       work.id::text,
       work.body_content_version_id::text,
       attempt.attempt_number,
       configuration.model,
       configuration.prompt_sha256,
       configuration.response_schema_version,
       configuration.response_schema_sha256,
       configuration.input_contract_sha256,
       work.last_safe_error_code::text
FROM ai_analysis_configurations AS configuration
JOIN ai_analysis_work_items AS work
  ON work.ai_analysis_configuration_id = configuration.id
JOIN ai_analysis_attempts AS attempt
  ON attempt.work_item_id = work.id
 AND attempt.status = 'succeeded'
WHERE configuration.id = ANY($1::uuid[])
  AND configuration.model = $2
  AND ((configuration.model = 'qwen-plus' AND configuration.status = 'candidate')
       OR (configuration.model = 'qwen-flash' AND configuration.status = 'comparison'))
  AND work.status = 'failed_safely'
  AND work.last_safe_error_code = 'policy_rejected'
  AND NOT EXISTS (
      SELECT 1
      FROM ai_output_audit_reviews AS review
      WHERE review.work_item_id = work.id
        AND review.attempt_number = attempt.attempt_number
        AND review.reviewer = $3
  )
ORDER BY array_position($1::uuid[], configuration.id), work.completed_at, work.id, attempt.attempt_number`, configurationIDs, model, AIControlledOutputAuditReviewer)
	if err != nil {
		return nil, fmt.Errorf("load controlled AI output-audit policy targets: %w", err)
	}
	defer rows.Close()
	targets := make([]controlledAIOutputAuditTarget, 0)
	for rows.Next() {
		var target controlledAIOutputAuditTarget
		if err := rows.Scan(
			&target.AIConfigurationID,
			&target.WorkItemID,
			&target.BodyContentVersionID,
			&target.AttemptNumber,
			&target.Model,
			&target.PromptSHA256,
			&target.ResponseSchemaVersion,
			&target.ResponseSchemaSHA256,
			&target.InputContractSHA256,
			&target.SafeErrorCode,
		); err != nil {
			return nil, fmt.Errorf("scan controlled AI output-audit policy target: %w", err)
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate controlled AI output-audit policy targets: %w", err)
	}
	return targets, nil
}

func (repository *Repository) replayAIPersistenceForOutputAuditDiagnostic(ctx context.Context, target controlledAIOutputAuditTarget, input AIModelInput, projection AISemanticProjection) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	result, err := tx.Exec(ctx, `
UPDATE ai_semantic_results
SET execution_status = 'running', semantic_status = 'not_analyzed',
    safe_error_code = NULL, completed_at = NULL
WHERE ai_analysis_configuration_id = $1
  AND body_content_version_id = $2
  AND execution_status = 'failed_safely'
  AND safe_error_code = 'policy_rejected'`, target.AIConfigurationID, target.BodyContentVersionID)
	if err != nil || result.RowsAffected() != 1 {
		return newAIPersistenceStageError(aiPersistenceStageFinalization, NewSafeAIError(AISafeErrorConfigurationInvalid, err))
	}
	work, err := tx.Exec(ctx, `
UPDATE ai_analysis_work_items
SET status = 'running', completed_at = NULL, last_run_id = NULL,
    last_safe_error_code = NULL, updated_at = now()
WHERE id = $1
  AND ai_analysis_configuration_id = $2
  AND body_content_version_id = $3
  AND status = 'failed_safely'
  AND last_safe_error_code = 'policy_rejected'`, target.WorkItemID, target.AIConfigurationID, target.BodyContentVersionID)
	if err != nil || work.RowsAffected() != 1 {
		return newAIPersistenceStageError(aiPersistenceStageFinalization, NewSafeAIError(AISafeErrorConfigurationInvalid, err))
	}
	return repository.persistAndCompleteAIWorkInTransaction(ctx, tx, target.AIConfigurationID, target.WorkItemID, target.BodyContentVersionID, input, projection)
}

func newAIOutputAuditPolicyDiagnostic(counts map[AIOutputAuditPolicyDiagnosticCategory]int, processedCount int) AIOutputAuditPolicyDiagnostic {
	result := AIOutputAuditPolicyDiagnostic{
		Counts:         make([]AIOutputAuditPolicyDiagnosticCount, 0, len(counts)),
		ProcessedCount: processedCount,
	}
	for category, count := range counts {
		result.Counts = append(result.Counts, AIOutputAuditPolicyDiagnosticCount{Category: category, Count: count})
	}
	sort.Slice(result.Counts, func(left, right int) bool {
		return result.Counts[left].Category < result.Counts[right].Category
	})
	return result
}
