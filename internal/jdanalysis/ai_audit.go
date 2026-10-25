package jdanalysis

import (
	"context"
)

// AIAuditCode is a closed, text-free event vocabulary. Its values are not
// operator supplied and do not describe source content or provider details.
type AIAuditCode string

const (
	AIAuditCandidatePrepared    AIAuditCode = "candidate_prepared"
	AIAuditWorkQueued           AIAuditCode = "work_queued"
	AIAuditAttemptStarted       AIAuditCode = "attempt_started"
	AIAuditAttemptCompleted     AIAuditCode = "attempt_completed"
	AIAuditSafeFailureRecorded  AIAuditCode = "safe_failure_recorded"
	AIAuditConfigurationPublish AIAuditCode = "configuration_published"
)

type AIAuditSeverity string

const (
	AIAuditInfo    AIAuditSeverity = "info"
	AIAuditWarning AIAuditSeverity = "warning"
	AIAuditError   AIAuditSeverity = "error"
)

// AISafeAuditEventInput deliberately has no message, diagnostics map, raw
// error, request, response, span, candidate string or credential field.
type AISafeAuditEventInput struct {
	AIConfigurationID string
	RunID             string
	WorkItemID        string
	Severity          AIAuditSeverity
	Code              AIAuditCode
	SafeErrorCode     AISafeErrorCode
	LogicalTaskCount  int
	AttemptCount      int
	InputTokenCount   int
	OutputTokenCount  int
	ChargeRMB         float64
}

func (repository *Repository) RecordAISafeAuditEvent(ctx context.Context, input AISafeAuditEventInput) error {
	if input.Severity == "" {
		input.Severity = AIAuditInfo
	}
	if !validAISafeAuditEvent(input) {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_audit_events (
    ai_analysis_configuration_id, run_id, work_item_id, severity, code, safe_error_code,
    logical_task_count, attempt_count, input_token_count, output_token_count, charge_rmb
) VALUES (
    $1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, $4::ai_audit_severity,
    $5::ai_audit_code, NULLIF($6, '')::ai_safe_error_code,
    $7, $8, $9, $10, $11
)`, input.AIConfigurationID, input.RunID, input.WorkItemID, input.Severity,
		input.Code, input.SafeErrorCode, input.LogicalTaskCount, input.AttemptCount,
		input.InputTokenCount, input.OutputTokenCount, input.ChargeRMB); err != nil {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return nil
}

func validAISafeAuditEvent(input AISafeAuditEventInput) bool {
	if input.AIConfigurationID == "" || !input.Code.valid() ||
		input.LogicalTaskCount < 0 || input.AttemptCount < 0 ||
		input.InputTokenCount < 0 || input.OutputTokenCount < 0 || input.ChargeRMB < 0 {
		return false
	}
	if !input.Severity.valid() {
		return false
	}
	return (input.Code == AIAuditSafeFailureRecorded) == input.SafeErrorCode.valid()
}

func (code AIAuditCode) valid() bool {
	switch code {
	case AIAuditCandidatePrepared, AIAuditWorkQueued, AIAuditAttemptStarted,
		AIAuditAttemptCompleted, AIAuditSafeFailureRecorded, AIAuditConfigurationPublish:
		return true
	default:
		return false
	}
}

func (severity AIAuditSeverity) valid() bool {
	return severity == AIAuditInfo || severity == AIAuditWarning || severity == AIAuditError
}
