-- A release is a durable, append-only fact.  The first baseline has no
-- predecessor to restore, so withdrawal is deliberately limited to two
-- enumerated incident categories rather than an unbounded operator message.

CREATE TYPE technology_analysis_release_event_type AS ENUM ('published', 'withdrawn');
CREATE TYPE technology_analysis_withdrawal_category AS ENUM (
    'material_result_quality_defect',
    'security_privacy_incident'
);

CREATE TABLE technology_analysis_release_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    configuration_id UUID NOT NULL REFERENCES technology_analysis_configurations(id),
    dictionary_version_id UUID NOT NULL REFERENCES technology_dictionary_versions(id),
    event_type technology_analysis_release_event_type NOT NULL,
    rule_set_sha256 CHAR(64) NOT NULL CHECK (rule_set_sha256 ~ '^[0-9a-f]{64}$'),
    release_gate_version TEXT NOT NULL CHECK (char_length(release_gate_version) BETWEEN 1 AND 128),
    publisher TEXT NOT NULL CHECK (char_length(publisher) BETWEEN 1 AND 256),
    withdrawal_category technology_analysis_withdrawal_category,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT technology_analysis_release_events_category_matches_event CHECK (
        (event_type = 'published' AND withdrawal_category IS NULL)
        OR (event_type = 'withdrawn' AND withdrawal_category IS NOT NULL)
    ),
    UNIQUE (configuration_id, event_type)
);

CREATE INDEX technology_analysis_release_events_dictionary_time_idx
    ON technology_analysis_release_events (dictionary_version_id, occurred_at DESC);

-- Release attestations must remain evidence, rather than an editable status
-- record. TRUNCATE is intentionally not blocked so disposable integration
-- databases can reset between tests.
CREATE FUNCTION reject_technology_analysis_release_event_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'technology analysis release events are immutable';
END;
$$;

CREATE TRIGGER technology_analysis_release_events_immutable
BEFORE UPDATE OR DELETE ON technology_analysis_release_events
FOR EACH ROW EXECUTE FUNCTION reject_technology_analysis_release_event_mutation();

-- Candidate rules are mutable only until their dictionary version is frozen
-- by publication or retirement. This keeps rule-set provenance stable even
-- for callers that accidentally bypass the Go lifecycle helpers.
CREATE FUNCTION reject_frozen_technology_dictionary_rule_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_dictionary_id UUID;
    target_status technology_dictionary_status;
BEGIN
    target_dictionary_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.dictionary_version_id ELSE NEW.dictionary_version_id END;
    SELECT status INTO target_status
    FROM technology_dictionary_versions
    WHERE id = target_dictionary_id;

    IF target_status IN ('published', 'retired') THEN
        RAISE EXCEPTION 'published or retired dictionary rules are immutable';
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

CREATE TRIGGER technology_dictionary_rules_frozen_immutable
BEFORE INSERT OR UPDATE OR DELETE ON technology_dictionary_rules
FOR EACH ROW EXECUTE FUNCTION reject_frozen_technology_dictionary_rule_mutation();

CREATE FUNCTION reject_frozen_technology_dictionary_candidate_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_dictionary_id UUID;
    target_status technology_dictionary_status;
BEGIN
    target_dictionary_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.dictionary_version_id ELSE NEW.dictionary_version_id END;
    SELECT status INTO target_status
    FROM technology_dictionary_versions
    WHERE id = target_dictionary_id;

    IF target_status IN ('published', 'retired') THEN
        RAISE EXCEPTION 'published or retired dictionary candidates are immutable';
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

CREATE TRIGGER technology_dictionary_candidates_frozen_immutable
BEFORE INSERT OR UPDATE OR DELETE ON technology_dictionary_candidates
FOR EACH ROW EXECUTE FUNCTION reject_frozen_technology_dictionary_candidate_mutation();

CREATE FUNCTION reject_frozen_technology_dictionary_decision_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_rule_id UUID;
    target_status technology_dictionary_status;
BEGIN
    target_rule_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.rule_id ELSE NEW.rule_id END;
    SELECT version.status INTO target_status
    FROM technology_dictionary_rules AS rule
    JOIN technology_dictionary_versions AS version ON version.id = rule.dictionary_version_id
    WHERE rule.id = target_rule_id;

    IF target_status IN ('published', 'retired') THEN
        RAISE EXCEPTION 'published or retired dictionary decisions are immutable';
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

CREATE TRIGGER technology_dictionary_rule_decisions_frozen_immutable
BEFORE INSERT OR UPDATE OR DELETE ON technology_dictionary_rule_decisions
FOR EACH ROW EXECUTE FUNCTION reject_frozen_technology_dictionary_decision_mutation();

-- The configuration snapshot is the other half of a released analysis
-- version. It may be populated only while a newly-created candidate exists;
-- after that, its members are append-free and cannot be replaced or removed.
CREATE FUNCTION reject_technology_analysis_configuration_rule_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_configuration_id UUID;
    target_status technology_analysis_configuration_status;
BEGIN
    target_configuration_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.configuration_id ELSE NEW.configuration_id END;
    SELECT status INTO target_status
    FROM technology_analysis_configurations
    WHERE id = target_configuration_id;

    IF TG_OP = 'INSERT' AND target_status = 'candidate' THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'analysis configuration rule-set snapshots are immutable';
END;
$$;

CREATE TRIGGER technology_analysis_configuration_rules_immutable
BEFORE INSERT OR UPDATE OR DELETE ON technology_analysis_configuration_rules
FOR EACH ROW EXECUTE FUNCTION reject_technology_analysis_configuration_rule_mutation();

-- A release may transition its dictionary/configuration pair to published,
-- then a controlled first-baseline withdrawal may transition it to retired.
-- Once frozen, provenance fields cannot be rewritten or deleted.
CREATE FUNCTION reject_frozen_technology_dictionary_version_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status = 'published' AND NEW.status = 'retired'
       AND NEW.id = OLD.id
       AND NEW.version_label = OLD.version_label
       AND NEW.content_sha256 = OLD.content_sha256
       AND NEW.source_document = OLD.source_document
       AND NEW.created_by = OLD.created_by
       AND NEW.created_at = OLD.created_at
       AND NEW.published_at = OLD.published_at
       AND NEW.retired_at IS NOT NULL THEN
        RETURN NEW;
    END IF;
    IF OLD.status IN ('published', 'retired') THEN
        RAISE EXCEPTION 'published or retired dictionary versions are immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER technology_dictionary_versions_frozen_immutable
BEFORE UPDATE OR DELETE ON technology_dictionary_versions
FOR EACH ROW EXECUTE FUNCTION reject_frozen_technology_dictionary_version_mutation();

CREATE FUNCTION reject_frozen_technology_analysis_configuration_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status = 'published' AND NEW.status IN ('retired', 'superseded')
       AND NEW.id = OLD.id
       AND NEW.parser_version = OLD.parser_version
       AND NEW.dictionary_version_id = OLD.dictionary_version_id
       AND NEW.rule_set_sha256 = OLD.rule_set_sha256
       AND NEW.created_by = OLD.created_by
       AND NEW.created_at = OLD.created_at
       AND NEW.published_at = OLD.published_at
       AND NEW.failed_at IS NOT DISTINCT FROM OLD.failed_at
       AND NEW.safe_failure_code IS NOT DISTINCT FROM OLD.safe_failure_code THEN
        RETURN NEW;
    END IF;
    IF OLD.status IN ('published', 'retired') THEN
        RAISE EXCEPTION 'published or retired analysis configurations are immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER technology_analysis_configurations_frozen_immutable
BEFORE UPDATE OR DELETE ON technology_analysis_configurations
FOR EACH ROW EXECUTE FUNCTION reject_frozen_technology_analysis_configuration_mutation();
