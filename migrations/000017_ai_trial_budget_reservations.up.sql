-- A reservation commits before a provider request is sent. Therefore a
-- process crash cannot make a physical request disappear from the shared
-- trial limits. Only opaque IDs, bounded token counts, and nano-RMB amounts
-- are retained here.

CREATE TYPE ai_analysis_trial_budget_reservation_status AS ENUM ('reserved', 'settled');

CREATE TABLE ai_analysis_trial_budget_reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_analysis_trial_budget_group_id UUID NOT NULL,
    ai_analysis_configuration_id UUID NOT NULL,
    work_item_id UUID NOT NULL,
    attempt_number INTEGER NOT NULL CHECK (attempt_number BETWEEN 1 AND 3),
    status ai_analysis_trial_budget_reservation_status NOT NULL DEFAULT 'reserved',
    estimated_input_tokens INTEGER NOT NULL CHECK (estimated_input_tokens BETWEEN 0 AND 10000),
    reserved_charge_nano_rmb BIGINT NOT NULL CHECK (reserved_charge_nano_rmb >= 0),
    actual_input_tokens INTEGER,
    actual_output_tokens INTEGER,
    actual_charge_nano_rmb BIGINT,
    reserved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    settled_at TIMESTAMPTZ,
    FOREIGN KEY (ai_analysis_trial_budget_group_id, ai_analysis_configuration_id)
        REFERENCES ai_analysis_trial_budget_group_members (
            ai_analysis_trial_budget_group_id, ai_analysis_configuration_id
        ),
    FOREIGN KEY (work_item_id, ai_analysis_configuration_id)
        REFERENCES ai_analysis_work_items (id, ai_analysis_configuration_id),
    UNIQUE (work_item_id, attempt_number),
    CONSTRAINT ai_analysis_trial_budget_reservations_status_matches_usage CHECK (
        (status = 'reserved'
         AND actual_input_tokens IS NULL
         AND actual_output_tokens IS NULL
         AND actual_charge_nano_rmb IS NULL
         AND settled_at IS NULL)
        OR
        (status = 'settled'
         AND actual_input_tokens IS NOT NULL
         AND actual_output_tokens IS NOT NULL
         AND actual_charge_nano_rmb IS NOT NULL
         AND settled_at IS NOT NULL)
    )
);

CREATE INDEX ai_analysis_trial_budget_reservations_group_idx
    ON ai_analysis_trial_budget_reservations (
        ai_analysis_trial_budget_group_id, reserved_at, id
    );
