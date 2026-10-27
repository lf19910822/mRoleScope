-- The former shared Plus/Flash trial used 60 logical tasks, 180 attempts and
-- a 2.00 RMB budget. Those immutable historical rows must remain readable.
-- New Plus-only trials get their own one-member group with the five-body,
-- fifteen-attempt, 2.50 RMB contract.

ALTER TABLE ai_analysis_trial_budget_groups
    -- PostgreSQL truncated the original generated names to these identifiers.
    DROP CONSTRAINT IF EXISTS ai_analysis_trial_budget_group_maximum_logical_task_count_check,
    DROP CONSTRAINT IF EXISTS ai_analysis_trial_budget_gro_maximum_physical_attempt_cou_check,
    DROP CONSTRAINT IF EXISTS ai_analysis_trial_budget_groups_maximum_charge_nano_rmb_check,
    ALTER COLUMN maximum_logical_task_count SET DEFAULT 5,
    ALTER COLUMN maximum_physical_attempt_count SET DEFAULT 15,
    ALTER COLUMN maximum_charge_nano_rmb SET DEFAULT 2500000000,
    ADD CONSTRAINT ai_trial_budget_groups_logical_task_limit
        CHECK (maximum_logical_task_count IN (5, 60)),
    ADD CONSTRAINT ai_trial_budget_groups_physical_attempt_limit
        CHECK (maximum_physical_attempt_count IN (15, 180)),
    ADD CONSTRAINT ai_trial_budget_groups_charge_limit
        CHECK (maximum_charge_nano_rmb IN (2000000000, 2500000000));
