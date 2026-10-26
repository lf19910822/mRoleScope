package jdanalysis

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

const AIControlledOutputAuditReviewer = "FAN"

type AIOutputAuditReviewInput struct {
	AIConfigurationID    string
	AttemptNumber        int
	BodyContentVersionID string
	OutputSHA256         string
	Verdict              string
	WorkItemID           string
}

// AIOutputAuditFailureCount is a fixed safe error category and its pending
// review count. It contains no filename, ID, digest, model text, or JD text.
type AIOutputAuditFailureCount struct {
	Count         int
	SafeErrorCode string
}

// AIOutputAuditPendingStatus is the safe, aggregate database half of the
// output-audit status view. Storage capacity is reported by AIOutputAuditStore.
type AIOutputAuditPendingStatus struct {
	FailureCounts      []AIOutputAuditFailureCount
	PendingReviewCount int
}

type controlledAIOutputAuditTarget struct {
	AIConfigurationID     string
	AttemptNumber         int
	BodyContentVersionID  string
	InputContractSHA256   string
	Model                 string
	PromptSHA256          string
	ResponseSchemaSHA256  string
	ResponseSchemaVersion string
	SafeErrorCode         string
	WorkItemID            string
}

// RunControlledAIOutputAuditSession is the only terminal seam that combines
// a captured model response with reconstructed input. It never accepts a path,
// body ID, prompt, model endpoint, SQL fragment, or free-form rationale.
func RunControlledAIOutputAuditSession(ctx context.Context, repository *Repository, store *aiOutputAuditStore, configurationIDs []string, model string, directTTY bool, input io.Reader, output io.Writer) (bool, error) {
	if repository == nil || store == nil || !validAIOutputAuditConfigurationIDs(configurationIDs) || !validControlledAIReviewModel(model) || !directTTY || input == nil || output == nil {
		return false, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	target, found, err := repository.nextControlledAIOutputAuditTarget(ctx, store, configurationIDs, model)
	if err != nil {
		return false, err
	}
	if !found {
		if _, err := fmt.Fprintln(output, "No captured contract-failure output is available."); err != nil {
			return false, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
		}
		return false, nil
	}
	if !validControlledAIOutputAuditContract(target) {
		return false, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	artifact, rawOutput, err := store.Load(AIOutputAuditIdentity{
		ConfigurationID:      target.AIConfigurationID,
		WorkItemID:           target.WorkItemID,
		AttemptNumber:        target.AttemptNumber,
		BodyContentVersionID: target.BodyContentVersionID,
	})
	if err != nil {
		return false, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	modelInput, err := repository.BuildAIModelInput(ctx, deterministicAIProviderRequestID(target.WorkItemID, target.AttemptNumber), target.AIConfigurationID, target.BodyContentVersionID)
	if err != nil {
		return false, err
	}
	inputJSON, err := json.Marshal(modelInput)
	if err != nil {
		return false, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	if err := writeControlledAIOutputAuditScreen(output, target, inputJSON, rawOutput); err != nil {
		return false, err
	}
	verdict, err := readControlledOutputAuditVerdict(bufio.NewReader(input))
	if err != nil {
		return false, err
	}
	if err := repository.RecordAIOutputAuditReview(ctx, AIOutputAuditReviewInput{
		AIConfigurationID:    target.AIConfigurationID,
		AttemptNumber:        target.AttemptNumber,
		BodyContentVersionID: target.BodyContentVersionID,
		OutputSHA256:         artifact.ResponseSHA256,
		Verdict:              verdict,
		WorkItemID:           target.WorkItemID,
	}); err != nil {
		return false, err
	}
	if _, err := fmt.Fprintf(output, "Recorded immutable contract-audit verdict: %s\n", verdict); err != nil {
		return false, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return true, nil
}

func (repository *Repository) nextControlledAIOutputAuditTarget(ctx context.Context, store *aiOutputAuditStore, configurationIDs []string, model string) (controlledAIOutputAuditTarget, bool, error) {
	if store == nil {
		return controlledAIOutputAuditTarget{}, false, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if len(configurationIDs) == 0 {
		return controlledAIOutputAuditTarget{}, false, nil
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
  AND work.last_safe_error_code IN ('invalid_response_schema', 'invalid_evidence_reference', 'policy_rejected')
  AND NOT EXISTS (
      SELECT 1
      FROM ai_output_audit_reviews AS review
      WHERE review.work_item_id = work.id
        AND review.attempt_number = attempt.attempt_number
        AND review.reviewer = $3
  )
ORDER BY array_position($1::uuid[], configuration.id), work.completed_at, work.id, attempt.attempt_number`, configurationIDs, model, AIControlledOutputAuditReviewer)
	if err != nil {
		return controlledAIOutputAuditTarget{}, false, fmt.Errorf("load controlled AI output audit target: %w", err)
	}
	defer rows.Close()
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
			return controlledAIOutputAuditTarget{}, false, fmt.Errorf("scan controlled AI output audit target: %w", err)
		}
		hasArtifact, err := store.Has(AIOutputAuditIdentity{
			ConfigurationID:      target.AIConfigurationID,
			WorkItemID:           target.WorkItemID,
			AttemptNumber:        target.AttemptNumber,
			BodyContentVersionID: target.BodyContentVersionID,
		})
		if err != nil {
			return controlledAIOutputAuditTarget{}, false, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
		}
		if hasArtifact {
			return target, true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return controlledAIOutputAuditTarget{}, false, fmt.Errorf("iterate controlled AI output audit targets: %w", err)
	}
	return controlledAIOutputAuditTarget{}, false, nil
}

// RecordAIOutputAuditReview persists only an immutable, non-text conclusion
// after the terminal session has independently verified artifact integrity.
func (repository *Repository) RecordAIOutputAuditReview(ctx context.Context, input AIOutputAuditReviewInput) error {
	if !validAIOutputAuditReviewInput(input) {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	command, err := repository.pool.Exec(ctx, `
INSERT INTO ai_output_audit_reviews (
    ai_analysis_configuration_id, work_item_id, body_content_version_id,
    attempt_number, reviewer, output_sha256, verdict
)
SELECT configuration.id, work.id, work.body_content_version_id,
       attempt.attempt_number, $5, $6, $7::ai_output_audit_verdict
FROM ai_analysis_configurations AS configuration
JOIN ai_analysis_work_items AS work
  ON work.ai_analysis_configuration_id = configuration.id
JOIN ai_analysis_attempts AS attempt
  ON attempt.work_item_id = work.id
 AND attempt.attempt_number = $3
 AND attempt.status = 'succeeded'
WHERE configuration.id = $1
  AND work.id = $2
  AND work.body_content_version_id = $4
  AND work.status = 'failed_safely'
  AND work.last_safe_error_code IN ('invalid_response_schema', 'invalid_evidence_reference', 'policy_rejected')`,
		input.AIConfigurationID, input.WorkItemID, input.AttemptNumber,
		input.BodyContentVersionID, AIControlledOutputAuditReviewer, input.OutputSHA256, input.Verdict,
	)
	if err != nil || command.RowsAffected() != 1 {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return nil
}

// ReadAIOutputAuditPendingStatus counts only failed work whose corresponding
// fixed identity is currently captured. It reads metadata through Has and
// never loads a model response byte.
func (repository *Repository) ReadAIOutputAuditPendingStatus(ctx context.Context, configurationIDs []string, store *aiOutputAuditStore) (AIOutputAuditPendingStatus, error) {
	if repository == nil || store == nil || !validAIOutputAuditConfigurationIDs(configurationIDs) {
		return AIOutputAuditPendingStatus{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if len(configurationIDs) == 0 {
		return AIOutputAuditPendingStatus{FailureCounts: []AIOutputAuditFailureCount{}}, nil
	}
	rows, err := repository.pool.Query(ctx, `
SELECT configuration.id::text,
       work.id::text,
       work.body_content_version_id::text,
       attempt.attempt_number,
       work.last_safe_error_code::text
FROM ai_analysis_work_items AS work
JOIN ai_analysis_configurations AS configuration
  ON configuration.id = work.ai_analysis_configuration_id
JOIN ai_analysis_attempts AS attempt
  ON attempt.work_item_id = work.id
 AND attempt.status = 'succeeded'
WHERE configuration.id = ANY($1::uuid[])
  AND ((configuration.model = 'qwen-plus' AND configuration.status = 'candidate')
       OR (configuration.model = 'qwen-flash' AND configuration.status = 'comparison'))
  AND work.status = 'failed_safely'
  AND work.last_safe_error_code IN ('invalid_response_schema', 'invalid_evidence_reference', 'policy_rejected')
  AND NOT EXISTS (
      SELECT 1
      FROM ai_output_audit_reviews AS review
      WHERE review.work_item_id = work.id
        AND review.attempt_number = attempt.attempt_number
        AND review.reviewer = $2
  )
ORDER BY array_position($1::uuid[], configuration.id), work.completed_at, work.id, attempt.attempt_number`, configurationIDs, AIControlledOutputAuditReviewer)
	if err != nil {
		return AIOutputAuditPendingStatus{}, fmt.Errorf("read pending AI output audit status: %w", err)
	}
	defer rows.Close()
	counts := make(map[string]int)
	for rows.Next() {
		var identity AIOutputAuditIdentity
		var safeErrorCode string
		if err := rows.Scan(&identity.ConfigurationID, &identity.WorkItemID, &identity.BodyContentVersionID, &identity.AttemptNumber, &safeErrorCode); err != nil {
			return AIOutputAuditPendingStatus{}, fmt.Errorf("scan pending AI output audit status: %w", err)
		}
		hasArtifact, err := store.Has(identity)
		if err != nil {
			return AIOutputAuditPendingStatus{}, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
		}
		if hasArtifact {
			counts[safeErrorCode]++
		}
	}
	if err := rows.Err(); err != nil {
		return AIOutputAuditPendingStatus{}, fmt.Errorf("iterate pending AI output audit status: %w", err)
	}
	status := AIOutputAuditPendingStatus{FailureCounts: make([]AIOutputAuditFailureCount, 0, len(counts))}
	for safeErrorCode, count := range counts {
		status.PendingReviewCount += count
		status.FailureCounts = append(status.FailureCounts, AIOutputAuditFailureCount{Count: count, SafeErrorCode: safeErrorCode})
	}
	sort.Slice(status.FailureCounts, func(left, right int) bool {
		return status.FailureCounts[left].SafeErrorCode < status.FailureCounts[right].SafeErrorCode
	})
	return status, nil
}

func validAIOutputAuditConfigurationIDs(configurationIDs []string) bool {
	seen := make(map[string]struct{}, len(configurationIDs))
	for _, configurationID := range configurationIDs {
		if !aiOutputAuditUUIDPattern.MatchString(configurationID) {
			return false
		}
		if _, duplicate := seen[configurationID]; duplicate {
			return false
		}
		seen[configurationID] = struct{}{}
	}
	return true
}

func writeControlledAIOutputAuditScreen(output io.Writer, target controlledAIOutputAuditTarget, inputJSON, rawOutput []byte) error {
	for _, section := range []struct {
		label string
		value string
	}{
		{"safe_error_code", target.SafeErrorCode},
		{"reconstructed_input", string(inputJSON)},
		{"raw_model_output", string(rawOutput)},
	} {
		if _, err := fmt.Fprintf(output, "--- %s ---\n%s\n", section.label, sanitizeControlledReviewTerminalText(section.value)); err != nil {
			return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
		}
	}
	if _, err := fmt.Fprint(output, "Contract verdict [prompt_noncompliance|schema_contract_mismatch|evidence_contract_mismatch|policy_contract_mismatch|unable_to_classify]: "); err != nil {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return nil
}

func readControlledOutputAuditVerdict(reader *bufio.Reader) (string, error) {
	verdict, err := readControlledReviewLine(reader)
	if err != nil || !validAIOutputAuditVerdict(verdict) {
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return verdict, nil
}

func validAIOutputAuditReviewInput(input AIOutputAuditReviewInput) bool {
	return aiOutputAuditUUIDPattern.MatchString(input.AIConfigurationID) &&
		aiOutputAuditUUIDPattern.MatchString(input.WorkItemID) &&
		aiOutputAuditUUIDPattern.MatchString(input.BodyContentVersionID) &&
		input.AttemptNumber >= 1 && input.AttemptNumber <= aiModelMaximumAttempts &&
		len(input.OutputSHA256) == 64 && validAIOutputAuditVerdict(input.Verdict)
}

func validAIOutputAuditVerdict(verdict string) bool {
	switch verdict {
	case "prompt_noncompliance", "schema_contract_mismatch", "evidence_contract_mismatch", "policy_contract_mismatch", "unable_to_classify":
		return true
	default:
		return false
	}
}

func validControlledAIOutputAuditInputContractSHA256(inputContractSHA256 string) bool {
	return inputContractSHA256 == controlledAIInputContractSHA256() ||
		inputContractSHA256 == controlledAIOutputAuditDiagnosticInputContractSHA256() ||
		inputContractSHA256 == legacyThirtyBodyControlledAIInputContractSHA256() ||
		inputContractSHA256 == legacyThirtyBodyControlledAIOutputAuditDiagnosticInputContractSHA256()
}

func validControlledAIOutputAuditContract(target controlledAIOutputAuditTarget) bool {
	if !validControlledAIOutputAuditInputContractSHA256(target.InputContractSHA256) {
		return false
	}
	return (target.PromptSHA256 == AISystemPromptV1SHA256() &&
		target.ResponseSchemaVersion == AIResponseSchemaVersionV1 &&
		target.ResponseSchemaSHA256 == AIResponseSchemaV1SHA256()) ||
		(target.PromptSHA256 == AISystemPromptV2SHA256() &&
			target.ResponseSchemaVersion == AIResponseSchemaVersionV2 &&
			target.ResponseSchemaSHA256 == AIResponseSchemaV2SHA256()) ||
		(target.PromptSHA256 == AISystemPromptV3SHA256() &&
			target.ResponseSchemaVersion == AIResponseSchemaVersionV3 &&
			target.ResponseSchemaSHA256 == AIResponseSchemaV3SHA256())
}
