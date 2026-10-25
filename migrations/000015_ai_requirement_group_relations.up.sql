-- Only qualification groups represent candidate-selection relations. Other
-- model-visible roles remain reviewable semantic facts, but must not invent an
-- at_least relationship or a minimum candidate count.

ALTER TABLE ai_technology_requirement_groups
    ALTER COLUMN operator DROP NOT NULL,
    ALTER COLUMN min_satisfied DROP NOT NULL;

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
