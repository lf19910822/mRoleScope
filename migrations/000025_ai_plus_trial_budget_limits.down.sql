-- Keep the permissive checks because immutable Plus-only groups may already
-- exist. Reverting the creation default preserves historical readability and
-- avoids mutating or deleting either generation of budget records.

ALTER TABLE ai_analysis_trial_budget_groups
    ALTER COLUMN maximum_logical_task_count SET DEFAULT 60,
    ALTER COLUMN maximum_physical_attempt_count SET DEFAULT 180,
    ALTER COLUMN maximum_charge_nano_rmb SET DEFAULT 2000000000;
