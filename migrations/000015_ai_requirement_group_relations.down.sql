-- A rollback must not coerce non-qualification facts into candidate
-- constraints. Refuse it while such facts exist so the operator can retire the
-- AI configuration deliberately instead of silently corrupting semantics.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ai_technology_requirement_groups
        WHERE role <> 'qualification'
    ) THEN
        RAISE EXCEPTION 'cannot roll back 000015 while non-qualification AI requirement groups exist';
    END IF;
END
$$;

ALTER TABLE ai_technology_requirement_groups
    DROP CONSTRAINT IF EXISTS ai_technology_requirement_groups_relation_matches_role;

ALTER TABLE ai_technology_requirement_groups
    ALTER COLUMN operator SET NOT NULL,
    ALTER COLUMN min_satisfied SET NOT NULL;
