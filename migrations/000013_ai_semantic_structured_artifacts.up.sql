-- Final semantic facts are normalized so reviews and release gates can reason
-- about each conclusion without retaining the model's JSON response or any JD
-- snippet. Every conclusion is first an artifact, allowing its final CVM
-- evidence coordinates and review tier to have one stable owner.

CREATE TYPE ai_semantic_artifact_kind AS ENUM (
    'experience_summary', 'experience_item', 'education_summary', 'condition_branch',
    'requirement_group', 'requirement_option', 'candidate_technology'
);
CREATE TYPE ai_semantic_review_tier AS ENUM ('routine', 'spot_check', 'required_review');
CREATE TYPE ai_education_level AS ENUM ('associate_or_lower', 'bachelor', 'master', 'doctor');
CREATE TYPE ai_requirement_role AS ENUM ('qualification', 'responsibility', 'mention', 'unspecified');
CREATE TYPE ai_requirement_necessity AS ENUM ('required', 'preferred', 'unspecified');
CREATE TYPE ai_requirement_operator AS ENUM ('at_least');
CREATE TYPE ai_proficiency_level AS ENUM ('p1', 'p2', 'p3', 'p4');
CREATE TYPE ai_semantic_review_verdict AS ENUM ('accepted', 'minor_error', 'major_error', 'not_reviewable');
CREATE TYPE ai_semantic_review_error_category AS ENUM (
    'necessity_reversal', 'alternative_reversal', 'education_reversal',
    'experience_reversal', 'other'
);

CREATE TABLE ai_semantic_artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_semantic_result_id UUID NOT NULL REFERENCES ai_semantic_results(id),
    artifact_kind ai_semantic_artifact_kind NOT NULL,
    semantic_status ai_semantic_value_status NOT NULL CHECK (semantic_status <> 'not_analyzed'),
    review_tier ai_semantic_review_tier NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, ai_semantic_result_id)
);

CREATE INDEX ai_semantic_artifacts_result_tier_idx
    ON ai_semantic_artifacts (ai_semantic_result_id, review_tier, artifact_kind);

CREATE TABLE ai_semantic_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    semantic_artifact_id UUID NOT NULL REFERENCES ai_semantic_artifacts(id),
    span_id TEXT NOT NULL CHECK (char_length(span_id) BETWEEN 1 AND 128),
    source_field TEXT NOT NULL CHECK (source_field IN ('description', 'jobRequire', 'addition')),
    normalization_version TEXT NOT NULL CHECK (char_length(normalization_version) BETWEEN 1 AND 128),
    normalized_field_sha256 CHAR(64) NOT NULL CHECK (normalized_field_sha256 ~ '^[0-9a-f]{64}$'),
    codepoint_start INTEGER NOT NULL CHECK (codepoint_start >= 0),
    codepoint_end INTEGER NOT NULL CHECK (codepoint_end > codepoint_start),
    technology_mention_evidence_id UUID REFERENCES technology_mention_evidence(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (semantic_artifact_id, source_field, codepoint_start, codepoint_end, span_id)
);

CREATE INDEX ai_semantic_evidence_artifact_idx
    ON ai_semantic_evidence (semantic_artifact_id, source_field, codepoint_start);

CREATE TABLE ai_experience_summaries (
    id UUID PRIMARY KEY REFERENCES ai_semantic_artifacts(id),
    status ai_semantic_value_status NOT NULL CHECK (status <> 'not_analyzed'),
    base_year NUMERIC(5, 2),
    CONSTRAINT ai_experience_summaries_value_matches_status CHECK (
        (status = 'configured' AND base_year IS NOT NULL AND base_year > 0)
        OR (status IN ('not_configured', 'uncertain') AND base_year IS NULL)
    )
);

CREATE TABLE ai_experience_requirement_items (
    id UUID PRIMARY KEY REFERENCES ai_semantic_artifacts(id),
    experience_summary_id UUID NOT NULL REFERENCES ai_experience_summaries(id),
    subject_kind TEXT NOT NULL CHECK (subject_kind IN ('overall', 'technology', 'domain')),
    technology_identity_id UUID REFERENCES technology_identities(id),
    domain_key TEXT CHECK (domain_key IS NULL OR char_length(domain_key) BETWEEN 1 AND 128),
    minimum_years NUMERIC(5, 2) NOT NULL CHECK (minimum_years > 0),
    CONSTRAINT ai_experience_requirement_items_subject_matches_kind CHECK (
        (subject_kind = 'overall' AND technology_identity_id IS NULL AND domain_key IS NULL)
        OR (subject_kind = 'technology' AND technology_identity_id IS NOT NULL AND domain_key IS NULL)
        OR (subject_kind = 'domain' AND technology_identity_id IS NULL AND domain_key IS NOT NULL)
    )
);

CREATE INDEX ai_experience_requirement_items_summary_idx
    ON ai_experience_requirement_items (experience_summary_id);

CREATE TABLE ai_minimum_education_requirements (
    id UUID PRIMARY KEY REFERENCES ai_semantic_artifacts(id),
    status ai_semantic_value_status NOT NULL CHECK (status <> 'not_analyzed'),
    minimum_level ai_education_level,
    CONSTRAINT ai_minimum_education_requirements_value_matches_status CHECK (
        (status = 'configured' AND minimum_level IS NOT NULL)
        OR (status IN ('not_configured', 'uncertain') AND minimum_level IS NULL)
    )
);

CREATE TABLE ai_semantic_condition_branches (
    id UUID PRIMARY KEY REFERENCES ai_semantic_artifacts(id),
    experience_summary_id UUID REFERENCES ai_experience_summaries(id),
    education_level ai_education_level,
    minimum_experience_years NUMERIC(5, 2) CHECK (
        minimum_experience_years IS NULL OR minimum_experience_years > 0
    ),
    branch_order INTEGER NOT NULL CHECK (branch_order >= 1),
    UNIQUE (experience_summary_id, branch_order),
    CONSTRAINT ai_semantic_condition_branches_has_condition CHECK (
        education_level IS NOT NULL OR minimum_experience_years IS NOT NULL
    )
);

CREATE TABLE ai_technology_requirement_groups (
    id UUID PRIMARY KEY REFERENCES ai_semantic_artifacts(id),
    role ai_requirement_role NOT NULL,
    necessity ai_requirement_necessity NOT NULL,
    operator ai_requirement_operator NOT NULL,
    min_satisfied INTEGER NOT NULL CHECK (min_satisfied >= 1)
);

CREATE TABLE ai_technology_requirement_options (
    id UUID PRIMARY KEY REFERENCES ai_semantic_artifacts(id),
    requirement_group_id UUID NOT NULL REFERENCES ai_technology_requirement_groups(id),
    technology_identity_id UUID NOT NULL REFERENCES technology_identities(id),
    proficiency ai_proficiency_level,
    option_order INTEGER NOT NULL DEFAULT 1 CHECK (option_order >= 1),
    UNIQUE (requirement_group_id, technology_identity_id),
    UNIQUE (requirement_group_id, option_order)
);

CREATE INDEX ai_technology_requirement_options_technology_idx
    ON ai_technology_requirement_options (technology_identity_id, requirement_group_id);

-- The candidate text remains in the existing remote dictionary-candidate
-- table. This link cannot create a technology identity, alias or option.
CREATE TABLE ai_candidate_technology_mentions (
    id UUID PRIMARY KEY REFERENCES ai_semantic_artifacts(id),
    dictionary_candidate_id UUID NOT NULL REFERENCES technology_dictionary_candidates(id),
    UNIQUE (dictionary_candidate_id, id)
);

CREATE TABLE ai_semantic_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_semantic_result_id UUID NOT NULL,
    semantic_artifact_id UUID NOT NULL,
    reviewer TEXT NOT NULL CHECK (char_length(reviewer) BETWEEN 1 AND 256),
    verdict ai_semantic_review_verdict NOT NULL,
    error_category ai_semantic_review_error_category,
    reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (semantic_artifact_id, ai_semantic_result_id)
        REFERENCES ai_semantic_artifacts (id, ai_semantic_result_id),
    UNIQUE (semantic_artifact_id, reviewer),
    CONSTRAINT ai_semantic_reviews_error_matches_verdict CHECK (
        (verdict = 'accepted' AND error_category IS NULL)
        OR (verdict IN ('minor_error', 'major_error') AND error_category IS NOT NULL)
        OR (verdict = 'not_reviewable' AND error_category IS NULL)
    )
);

CREATE INDEX ai_semantic_reviews_result_time_idx
    ON ai_semantic_reviews (ai_semantic_result_id, reviewed_at DESC);

CREATE TABLE ai_analysis_publication_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    ai_analysis_configuration_id UUID REFERENCES ai_analysis_configurations(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO ai_analysis_publication_state (singleton) VALUES (true);

-- The comparison model is intentionally never eligible for default routing.
-- Enforcing the invariant at the pointer closes the gap between a controlled
-- release operation and any future direct SQL maintenance path.
CREATE FUNCTION require_published_qwen_plus_ai_publication()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    selected_model TEXT;
    selected_status ai_analysis_configuration_status;
BEGIN
    IF NEW.ai_analysis_configuration_id IS NULL THEN
        RETURN NEW;
    END IF;
    SELECT model, status INTO selected_model, selected_status
    FROM ai_analysis_configurations
    WHERE id = NEW.ai_analysis_configuration_id;
    IF selected_model IS DISTINCT FROM 'qwen-plus'
       OR selected_status IS DISTINCT FROM 'published' THEN
        RAISE EXCEPTION 'AI publication default requires a published qwen-plus configuration';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER ai_analysis_publication_state_requires_published_qwen_plus
BEFORE INSERT OR UPDATE OF ai_analysis_configuration_id ON ai_analysis_publication_state
FOR EACH ROW EXECUTE FUNCTION require_published_qwen_plus_ai_publication();

CREATE TABLE ai_analysis_release_attestations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_analysis_configuration_id UUID NOT NULL UNIQUE REFERENCES ai_analysis_configurations(id),
    gate_version TEXT NOT NULL CHECK (char_length(gate_version) BETWEEN 1 AND 128),
    publisher TEXT NOT NULL CHECK (char_length(publisher) BETWEEN 1 AND 256),
    attestation_sha256 CHAR(64) NOT NULL CHECK (attestation_sha256 ~ '^[0-9a-f]{64}$'),
    trial_body_count INTEGER NOT NULL CHECK (trial_body_count >= 0),
    fully_reviewed_trial_body_count INTEGER NOT NULL CHECK (
        fully_reviewed_trial_body_count BETWEEN 0 AND trial_body_count
    ),
    major_error_count INTEGER NOT NULL CHECK (major_error_count >= 0),
    released_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Review verdicts and release attestations are append-only proof. They are
-- deliberately separate from mutable execution bookkeeping.
CREATE FUNCTION reject_ai_semantic_review_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'AI semantic reviews are immutable';
END;
$$;

CREATE TRIGGER ai_semantic_reviews_immutable
BEFORE UPDATE OR DELETE ON ai_semantic_reviews
FOR EACH ROW EXECUTE FUNCTION reject_ai_semantic_review_mutation();

CREATE FUNCTION reject_ai_analysis_release_attestation_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'AI analysis release attestations are immutable';
END;
$$;

CREATE TRIGGER ai_analysis_release_attestations_immutable
BEFORE UPDATE OR DELETE ON ai_analysis_release_attestations
FOR EACH ROW EXECUTE FUNCTION reject_ai_analysis_release_attestation_mutation();
