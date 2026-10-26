-- Retain only bounded, provider-neutral completion metadata with each
-- physical attempt. Raw provider output remains exclusively in the CVM's
-- root-only audit store and is never represented in PostgreSQL.

ALTER TABLE ai_analysis_attempts
    ADD COLUMN provider_completion_signal TEXT,
    ADD COLUMN provider_response_length_bytes INTEGER,
    ADD CONSTRAINT ai_analysis_attempts_provider_completion_metadata_valid CHECK (
        (provider_completion_signal IS NULL AND provider_response_length_bytes IS NULL)
        OR (
            provider_completion_signal IN ('unknown', 'stop', 'length', 'other')
            AND provider_response_length_bytes BETWEEN 1 AND 1048576
        )
    );
