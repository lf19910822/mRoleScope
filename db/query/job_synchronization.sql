-- name: EnsureObservationScope :one
INSERT INTO observation_scopes (source_company, filter_config, filter_sha256)
VALUES ($1, $2, $3)
ON CONFLICT (source_company, filter_sha256)
DO UPDATE SET filter_config = EXCLUDED.filter_config
RETURNING *;

-- name: CreateCollectionRun :one
INSERT INTO collection_runs (
    id, schedule_key, source_company, filter_config, filter_sha256,
    observation_scope_id, data_date, trigger_kind, attempt_number, status, started_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'collecting', $10)
RETURNING *;

-- name: AppendCollectionRunAuditEvent :one
INSERT INTO collection_run_audit_events (
    run_id, occurred_at, stage, severity, code, attempt_number, message, diagnostics
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetOpenPresencePeriods :many
SELECT p.*, j.source_job_id
FROM job_presence_periods AS p
JOIN jobs AS j ON j.id = p.job_id
WHERE p.observation_scope_id = $1 AND p.closed_at IS NULL
ORDER BY j.source_job_id;

-- name: GetRecentAuditEvents :many
SELECT *
FROM collection_run_audit_events
WHERE run_id = $1
ORDER BY occurred_at, id;
