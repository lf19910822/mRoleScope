package aitrend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	AuditRetention      = 30 * 24 * time.Hour
	AuditMaximumRecords = 10_000
)

type AuditRecord struct {
	ID                 string
	RequestHash        string
	IntentHash         *string
	DraftHash          *string
	ContractVersion    string
	ValidationStatus   string
	Status             string
	SafeErrorCode      *string
	AIAttemptCount     int
	ToolCallCount      int
	ToolFailureCount   int
	InputTokenCount    int
	OutputTokenCount   int
	Duration           time.Duration
	DraftCurveCount    int
	DraftSelectedCount int
	DraftWarningCount  int
	CreatedAt          time.Time
	CompletedAt        *time.Time
	ExpiresAt          time.Time
}

type AuditToolRecord struct {
	AuditID       string
	Sequence      int
	ToolName      string
	Status        string
	SafeErrorCode *string
	Duration      time.Duration
	StartedAt     time.Time
	CompletedAt   *time.Time
}

func HashRequest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func (record AuditRecord) Validate() error {
	if len(record.RequestHash) != 64 || record.ContractVersion == "" || record.ValidationStatus == "" || record.Status == "" || record.ExpiresAt.Before(record.CreatedAt) {
		return fmt.Errorf("invalid bounded audit record")
	}
	if record.AIAttemptCount < 0 || record.AIAttemptCount > MaxAIAttempts || record.ToolCallCount < 0 || record.ToolCallCount > MaxToolCalls || record.DraftCurveCount < 0 || record.DraftCurveCount > MaxCurves || record.DraftSelectedCount < 0 || record.DraftSelectedCount > MaxCurves {
		return fmt.Errorf("audit record exceeds contract limit")
	}
	return nil
}

// AuditStore persists aggregate facts only. Callers must pass a hash and
// contract-derived counters; raw requests, model output and tool payloads are
// deliberately absent from this API.
type AuditStore struct{ pool *pgxpool.Pool }

func NewAuditStore(pool *pgxpool.Pool) *AuditStore { return &AuditStore{pool: pool} }

func (store *AuditStore) Insert(ctx context.Context, record AuditRecord) (string, error) {
	if store == nil || store.pool == nil {
		return "", fmt.Errorf("audit store is unavailable")
	}
	if err := record.Validate(); err != nil {
		return "", err
	}
	var id string
	err := store.pool.QueryRow(ctx, `INSERT INTO ai_trend_request_audits
        (request_hash, intent_hash, draft_hash, contract_version, validation_status, status, safe_error_code,
         ai_attempt_count, tool_call_count, tool_failure_count, input_token_count, output_token_count, duration_ms,
         draft_curve_count, draft_selected_count, draft_warning_count, created_at, completed_at, expires_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
        RETURNING id::text`,
		record.RequestHash, record.IntentHash, record.DraftHash, record.ContractVersion, record.ValidationStatus, record.Status, record.SafeErrorCode,
		record.AIAttemptCount, record.ToolCallCount, record.ToolFailureCount, record.InputTokenCount, record.OutputTokenCount, record.Duration.Milliseconds(),
		record.DraftCurveCount, record.DraftSelectedCount, record.DraftWarningCount, record.CreatedAt, record.CompletedAt, record.ExpiresAt).Scan(&id)
	return id, err
}

func (store *AuditStore) InsertTool(ctx context.Context, record AuditToolRecord) error {
	if store == nil || store.pool == nil {
		return fmt.Errorf("audit store is unavailable")
	}
	if record.AuditID == "" || record.Sequence < 1 || record.Sequence > MaxToolCalls || record.ToolName == "" {
		return fmt.Errorf("invalid bounded tool audit")
	}
	_, err := store.pool.Exec(ctx, `INSERT INTO ai_trend_request_audit_tools
        (request_audit_id, sequence_number, tool_name, status, safe_error_code, duration_ms, started_at, completed_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, record.AuditID, record.Sequence, record.ToolName, record.Status, record.SafeErrorCode, record.Duration.Milliseconds(), record.StartedAt, record.CompletedAt)
	return err
}

// Cleanup is intentionally warning-only at the call site: a transient cleanup
// failure must not turn a successful user request into a failed request.
func (store *AuditStore) Cleanup(ctx context.Context) error {
	if store == nil || store.pool == nil {
		return fmt.Errorf("audit store is unavailable")
	}
	_, err := store.pool.Exec(ctx, `
WITH expired AS (
    SELECT id FROM ai_trend_request_audits WHERE expires_at < now()
), overflow AS (
    SELECT id FROM ai_trend_request_audits ORDER BY created_at ASC, id ASC OFFSET $1
), victims AS (
    SELECT id FROM expired UNION SELECT id FROM overflow
)
DELETE FROM ai_trend_request_audits WHERE id IN (SELECT id FROM victims)`, AuditMaximumRecords)
	return err
}
