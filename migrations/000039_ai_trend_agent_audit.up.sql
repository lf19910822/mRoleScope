ALTER TABLE ai_trend_request_audits
    ADD COLUMN response_modes TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN reply_warning_count INTEGER NOT NULL DEFAULT 0 CHECK (reply_warning_count BETWEEN 0 AND 3),
    ADD CONSTRAINT ai_trend_request_audits_response_modes_bounded
        CHECK (cardinality(response_modes) <= 2);

ALTER TABLE ai_trend_request_audits
    ADD CONSTRAINT ai_trend_request_audits_response_modes_valid
        CHECK (response_modes <@ ARRAY['trend', 'guidance']::TEXT[]);
