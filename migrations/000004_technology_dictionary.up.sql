-- An immutable dictionary version separates source terms from their later
-- human approval.  All v0.1 seeds start non-enabled; no rule is executable
-- without an explicit reviewer, time, and decision record.

CREATE TYPE technology_dictionary_status AS ENUM ('draft', 'candidate', 'published', 'retired');
CREATE TYPE technology_rule_state AS ENUM ('enabled', 'conditional', 'candidate');
CREATE TYPE technology_rule_decision AS ENUM ('approved_enabled', 'set_conditional', 'set_candidate', 'disabled');

CREATE TABLE technology_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    technology_key TEXT NOT NULL UNIQUE CHECK (char_length(technology_key) BETWEEN 1 AND 128),
    canonical_name TEXT NOT NULL CHECK (char_length(canonical_name) BETWEEN 1 AND 256),
    technology_kind TEXT NOT NULL CHECK (char_length(technology_kind) BETWEEN 1 AND 64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE technology_dictionary_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_label TEXT NOT NULL UNIQUE CHECK (char_length(version_label) BETWEEN 1 AND 128),
    content_sha256 CHAR(64) NOT NULL CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),
    status technology_dictionary_status NOT NULL DEFAULT 'draft',
    source_document TEXT NOT NULL CHECK (char_length(source_document) <= 512),
    created_by TEXT NOT NULL CHECK (char_length(created_by) BETWEEN 1 AND 256),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    retired_at TIMESTAMPTZ,
    CONSTRAINT technology_dictionary_versions_published_together CHECK (
        (status = 'published' AND published_at IS NOT NULL)
        OR (status <> 'published')
    )
);

CREATE TABLE technology_dictionary_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES technology_dictionary_versions(id),
    technology_identity_id UUID NOT NULL REFERENCES technology_identities(id),
    rule_key TEXT NOT NULL CHECK (char_length(rule_key) BETWEEN 1 AND 128),
    alias TEXT NOT NULL CHECK (char_length(alias) BETWEEN 1 AND 256),
    state technology_rule_state NOT NULL DEFAULT 'candidate',
    case_mode TEXT NOT NULL CHECK (case_mode IN ('sensitive', 'insensitive')),
    boundary_mode TEXT NOT NULL CHECK (boundary_mode IN ('exact', 'ascii_identifier', 'custom')),
    overlap_priority INTEGER NOT NULL DEFAULT 0,
    rule_summary TEXT NOT NULL CHECK (char_length(rule_summary) <= 1024),
    approved_by TEXT CHECK (char_length(approved_by) BETWEEN 1 AND 256),
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dictionary_version_id, rule_key),
    UNIQUE (dictionary_version_id, alias, technology_identity_id),
    CONSTRAINT technology_dictionary_rules_enabled_requires_approval CHECK (
        state <> 'enabled' OR (approved_by IS NOT NULL AND approved_at IS NOT NULL)
    )
);

CREATE INDEX technology_dictionary_rules_executable_idx
    ON technology_dictionary_rules (dictionary_version_id, overlap_priority DESC, alias)
    WHERE state = 'enabled';

CREATE TABLE technology_dictionary_rule_decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id UUID NOT NULL REFERENCES technology_dictionary_rules(id),
    decision technology_rule_decision NOT NULL,
    reviewer TEXT NOT NULL CHECK (char_length(reviewer) BETWEEN 1 AND 256),
    rationale TEXT NOT NULL CHECK (char_length(rationale) <= 1024),
    decided_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX technology_dictionary_rule_decisions_rule_time_idx
    ON technology_dictionary_rule_decisions (rule_id, decided_at DESC);

-- Candidate strings that lack an approved canonical technology identity stay
-- distinct from executable rules. They are review inputs, never statistics.
CREATE TABLE technology_dictionary_candidates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES technology_dictionary_versions(id),
    candidate_text TEXT NOT NULL CHECK (char_length(candidate_text) BETWEEN 1 AND 256),
    proposed_technology_identity_id UUID REFERENCES technology_identities(id),
    state technology_rule_state NOT NULL DEFAULT 'candidate' CHECK (state <> 'enabled'),
    candidate_summary TEXT NOT NULL CHECK (char_length(candidate_summary) <= 1024),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dictionary_version_id, candidate_text)
);
