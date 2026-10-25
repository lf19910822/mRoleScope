DROP TRIGGER IF EXISTS ai_analysis_trial_budget_group_members_immutable
    ON ai_analysis_trial_budget_group_members;
DROP TRIGGER IF EXISTS ai_analysis_trial_budget_groups_immutable
    ON ai_analysis_trial_budget_groups;
DROP FUNCTION IF EXISTS reject_ai_analysis_trial_budget_group_mutation();

DROP TABLE IF EXISTS ai_analysis_trial_budget_group_members;
DROP TABLE IF EXISTS ai_analysis_trial_budget_groups;
