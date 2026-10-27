-- A hard response contract now protects framing, schema and request/body
-- binding only. These tables retain structurally valid model projections, so
-- they must not reject a result merely because its semantic state, relation,
-- or redundancy is unconventional. Numeric bounds and enum types still
-- protect the normalized relational shape.

ALTER TABLE ai_experience_summaries
    DROP CONSTRAINT IF EXISTS ai_experience_summaries_value_matches_status;

ALTER TABLE ai_experience_summaries
    ADD CONSTRAINT ai_experience_summaries_base_year_is_positive_when_present
    CHECK (base_year IS NULL OR base_year > 0);

ALTER TABLE ai_minimum_education_requirements
    DROP CONSTRAINT IF EXISTS ai_minimum_education_requirements_value_matches_status;

ALTER TABLE ai_technology_requirement_groups
    DROP CONSTRAINT IF EXISTS ai_technology_requirement_groups_relation_matches_role;
