-- Do not discard an attestation that uses the mechanical proof fields.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ai_analysis_release_attestations
        WHERE total_work_count <> 0
           OR terminal_work_count <> 0
           OR structural_success_count <> 0
           OR safe_failure_count <> 0
           OR attempt_count <> 0
           OR input_token_count <> 0
           OR output_token_count <> 0
           OR charge_nano_rmb <> 0
    ) THEN
        RAISE EXCEPTION 'cannot roll back 000026 while mechanical release attestations exist';
    END IF;
END
$$;

ALTER TABLE ai_analysis_release_attestations
    DROP CONSTRAINT IF EXISTS ai_analysis_release_attestations_terminal_outcomes_match_work,
    DROP COLUMN charge_nano_rmb,
    DROP COLUMN output_token_count,
    DROP COLUMN input_token_count,
    DROP COLUMN attempt_count,
    DROP COLUMN safe_failure_count,
    DROP COLUMN structural_success_count,
    DROP COLUMN terminal_work_count,
    DROP COLUMN total_work_count;
