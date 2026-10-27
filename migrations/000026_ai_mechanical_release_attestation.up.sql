-- Mechanical publication records only terminal execution and bounded usage
-- facts. Existing review-derived attestation fields remain immutable history;
-- new fields make the Plus-only publication proof independent of review or
-- semantic-quality judgments.

ALTER TABLE ai_analysis_release_attestations
    ADD COLUMN total_work_count INTEGER NOT NULL DEFAULT 0
        CHECK (total_work_count >= 0),
    ADD COLUMN terminal_work_count INTEGER NOT NULL DEFAULT 0
        CHECK (terminal_work_count BETWEEN 0 AND total_work_count),
    ADD COLUMN structural_success_count INTEGER NOT NULL DEFAULT 0
        CHECK (structural_success_count BETWEEN 0 AND total_work_count),
    ADD COLUMN safe_failure_count INTEGER NOT NULL DEFAULT 0
        CHECK (safe_failure_count BETWEEN 0 AND total_work_count),
    ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0
        CHECK (attempt_count >= 0),
    ADD COLUMN input_token_count BIGINT NOT NULL DEFAULT 0
        CHECK (input_token_count >= 0),
    ADD COLUMN output_token_count BIGINT NOT NULL DEFAULT 0
        CHECK (output_token_count >= 0),
    ADD COLUMN charge_nano_rmb BIGINT NOT NULL DEFAULT 0
        CHECK (charge_nano_rmb >= 0),
    ADD CONSTRAINT ai_analysis_release_attestations_terminal_outcomes_match_work
        CHECK (structural_success_count + safe_failure_count <= terminal_work_count);
