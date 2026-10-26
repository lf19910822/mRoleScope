ALTER TABLE ai_analysis_attempts
    DROP CONSTRAINT IF EXISTS ai_analysis_attempts_provider_completion_metadata_valid,
    DROP COLUMN IF EXISTS provider_response_length_bytes,
    DROP COLUMN IF EXISTS provider_completion_signal;
