-- A source-exact term is retained only when a v3 model response offers a
-- different normalization. It is a review artifact, never a dictionary
-- candidate, technology identity, alias, matching input, or requirement option.
ALTER TYPE ai_semantic_artifact_kind
    ADD VALUE IF NOT EXISTS 'unresolved_source_term';

CREATE TABLE ai_pending_unresolved_source_terms (
    id UUID PRIMARY KEY REFERENCES ai_semantic_artifacts(id),
    source_term TEXT NOT NULL CHECK (char_length(source_term) BETWEEN 1 AND 128)
);
