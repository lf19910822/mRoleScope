-- The Plus baseline and Flash comparison are one bounded external trial, not
-- two independent per-worker allowances. This ledger stores only immutable
-- configuration identities and fixed accounting limits; it never stores text,
-- prompts, responses, provider diagnostics, or credentials.

CREATE TABLE ai_analysis_trial_budget_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    maximum_logical_task_count INTEGER NOT NULL DEFAULT 60
        CHECK (maximum_logical_task_count = 60),
    maximum_physical_attempt_count INTEGER NOT NULL DEFAULT 180
        CHECK (maximum_physical_attempt_count = 180),
    maximum_charge_nano_rmb BIGINT NOT NULL DEFAULT 2000000000
        CHECK (maximum_charge_nano_rmb = 2000000000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ai_analysis_trial_budget_group_members (
    ai_analysis_trial_budget_group_id UUID NOT NULL
        REFERENCES ai_analysis_trial_budget_groups(id),
    ai_analysis_configuration_id UUID NOT NULL UNIQUE
        REFERENCES ai_analysis_configurations(id),
    PRIMARY KEY (ai_analysis_trial_budget_group_id, ai_analysis_configuration_id)
);

CREATE FUNCTION reject_ai_analysis_trial_budget_group_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'AI trial budget groups are immutable';
END;
$$;

CREATE TRIGGER ai_analysis_trial_budget_groups_immutable
BEFORE UPDATE OR DELETE ON ai_analysis_trial_budget_groups
FOR EACH ROW EXECUTE FUNCTION reject_ai_analysis_trial_budget_group_mutation();

CREATE TRIGGER ai_analysis_trial_budget_group_members_immutable
BEFORE UPDATE OR DELETE ON ai_analysis_trial_budget_group_members
FOR EACH ROW EXECUTE FUNCTION reject_ai_analysis_trial_budget_group_mutation();
