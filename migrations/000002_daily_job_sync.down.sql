DROP TABLE IF EXISTS collection_run_audit_events;
DROP TABLE IF EXISTS job_content_periods;
DROP TABLE IF EXISTS job_presence_periods;
DROP TABLE IF EXISTS observation_scope_publication_state;

DROP INDEX IF EXISTS collection_runs_scope_data_date_idx;
DROP INDEX IF EXISTS collection_runs_scope_active_idx;
ALTER TABLE collection_runs
    DROP CONSTRAINT IF EXISTS collection_runs_archive_manifest_sha256_valid,
    DROP CONSTRAINT IF EXISTS collection_runs_attempt_number_positive,
    DROP COLUMN IF EXISTS archive_manifest_sha256,
    DROP COLUMN IF EXISTS attempt_number,
    DROP COLUMN IF EXISTS trigger_kind,
    DROP COLUMN IF EXISTS data_date,
    DROP COLUMN IF EXISTS observation_scope_id;
CREATE UNIQUE INDEX collection_runs_schedule_key_active_idx
    ON collection_runs (schedule_key)
    WHERE status NOT IN ('published', 'failed', 'incomplete');

DROP TABLE IF EXISTS observation_scopes;
DROP TYPE IF EXISTS collection_trigger_kind;
