-- Do not reintroduce the prior semantic gates while rows that rely on the
-- projection-tolerant contract remain. A downgrade is intentionally blocked
-- rather than silently deleting or rewriting immutable results.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ai_experience_summaries
        WHERE (status = 'configured' AND base_year IS NULL)
           OR (status IN ('not_configured', 'uncertain') AND base_year IS NOT NULL)
    ) THEN
        RAISE EXCEPTION 'cannot roll back 000023 while relaxed AI experience projections exist';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM ai_minimum_education_requirements
        WHERE (status = 'configured' AND minimum_level IS NULL)
           OR (status IN ('not_configured', 'uncertain') AND minimum_level IS NOT NULL)
    ) THEN
        RAISE EXCEPTION 'cannot roll back 000023 while relaxed AI education projections exist';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM ai_technology_requirement_groups
        WHERE (role = 'qualification' AND (operator IS DISTINCT FROM 'at_least' OR min_satisfied IS NULL))
           OR (role <> 'qualification' AND (operator IS NOT NULL OR min_satisfied IS NOT NULL))
    ) THEN
        RAISE EXCEPTION 'cannot roll back 000023 while relaxed AI requirement relations exist';
    END IF;
END
$$;

ALTER TABLE ai_experience_summaries
    DROP CONSTRAINT IF EXISTS ai_experience_summaries_base_year_is_positive_when_present;

ALTER TABLE ai_experience_summaries
    ADD CONSTRAINT ai_experience_summaries_value_matches_status CHECK (
        (status = 'configured' AND base_year IS NOT NULL AND base_year > 0)
        OR (status IN ('not_configured', 'uncertain') AND base_year IS NULL)
    );

ALTER TABLE ai_minimum_education_requirements
    ADD CONSTRAINT ai_minimum_education_requirements_value_matches_status CHECK (
        (status = 'configured' AND minimum_level IS NOT NULL)
        OR (status IN ('not_configured', 'uncertain') AND minimum_level IS NULL)
    );

ALTER TABLE ai_technology_requirement_groups
    ADD CONSTRAINT ai_technology_requirement_groups_relation_matches_role CHECK (
        (role = 'qualification'
         AND operator = 'at_least'
         AND min_satisfied IS NOT NULL
         AND min_satisfied >= 1)
        OR
        (role <> 'qualification'
         AND operator IS NULL
         AND min_satisfied IS NULL)
    );
