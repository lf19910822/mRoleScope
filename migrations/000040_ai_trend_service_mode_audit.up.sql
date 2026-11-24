ALTER TABLE ai_trend_request_audits
    ADD COLUMN service_tags TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN classification_status TEXT,
    ADD COLUMN classification_input_token_count INTEGER NOT NULL DEFAULT 0 CHECK (classification_input_token_count >= 0),
    ADD COLUMN classification_output_token_count INTEGER NOT NULL DEFAULT 0 CHECK (classification_output_token_count >= 0),
    ADD COLUMN classification_duration_ms BIGINT NOT NULL DEFAULT 0 CHECK (classification_duration_ms >= 0),
    ADD CONSTRAINT ai_trend_request_audits_service_tags_bounded
        CHECK (cardinality(service_tags) <= 2),
    ADD CONSTRAINT ai_trend_request_audits_service_tags_valid
        CHECK (service_tags <@ ARRAY['trend', 'guidance']::TEXT[]),
    ADD CONSTRAINT ai_trend_request_audits_classification_status_valid
        CHECK (classification_status IS NULL OR classification_status IN ('running', 'completed', 'failed'));
