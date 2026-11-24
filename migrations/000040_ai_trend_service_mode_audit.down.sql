ALTER TABLE ai_trend_request_audits
    DROP CONSTRAINT IF EXISTS ai_trend_request_audits_classification_status_valid,
    DROP CONSTRAINT IF EXISTS ai_trend_request_audits_service_tags_valid,
    DROP CONSTRAINT IF EXISTS ai_trend_request_audits_service_tags_bounded,
    DROP COLUMN IF EXISTS classification_duration_ms,
    DROP COLUMN IF EXISTS classification_output_token_count,
    DROP COLUMN IF EXISTS classification_input_token_count,
    DROP COLUMN IF EXISTS classification_status,
    DROP COLUMN IF EXISTS service_tags;
