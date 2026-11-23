ALTER TABLE ai_trend_request_audits
    DROP CONSTRAINT IF EXISTS ai_trend_request_audits_response_modes_valid,
    DROP CONSTRAINT IF EXISTS ai_trend_request_audits_response_modes_bounded,
    DROP COLUMN IF EXISTS reply_warning_count,
    DROP COLUMN IF EXISTS response_modes;
