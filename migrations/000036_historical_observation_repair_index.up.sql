-- A target calendar date may have at most one controlled historical repair.
-- The source publication itself remains immutable and the current publication
-- pointer is deliberately not changed by a repair.
CREATE UNIQUE INDEX collection_runs_one_repair_per_scope_date_idx
    ON collection_runs (observation_scope_id, data_date)
    WHERE trigger_kind = 'repair';

COMMENT ON INDEX collection_runs_one_repair_per_scope_date_idx IS
    'At most one idempotent historical observation repair per scope and calendar date.';
