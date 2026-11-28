package jdserving

// The positive predicates are evaluated against one membership, before job
// deduplication. Exclusions inspect every current effective membership of the
// same job: an explicit match or an unknown excluded dimension disqualifies
// that job. Group values are read only from positive matching memberships.
const crossStatisticsSQL = `
WITH condition_groups AS (
    SELECT false AS excluded, group_item->>'dimension' AS dimension,
           group_item->>'operator' AS operator,
           ARRAY(SELECT jsonb_array_elements_text(group_item->'values')) AS values
    FROM jsonb_array_elements($1::jsonb) AS group_item
    UNION ALL
    SELECT true, group_item->>'dimension', group_item->>'operator',
           ARRAY(SELECT jsonb_array_elements_text(group_item->'values'))
    FROM jsonb_array_elements($2::jsonb) AS group_item
), membership_facts AS MATERIALIZED (
    SELECT membership.id, membership.job_id, membership.locations,
           COALESCE(metadata.metadata_payload #>> '{locations,state}' = 'array'
                    AND EXISTS (SELECT 1 FROM unnest(membership.locations) AS location(value)
                                WHERE btrim(location.value) <> ''), false) AS location_known,
           COALESCE(technology.known, false) AS technology_known,
           COALESCE(technology.values, '{}'::text[]) AS technology_values,
           COALESCE(project.known, false) AS project_known,
           COALESCE(project.values, '{}'::text[]) AS project_values,
           COALESCE(family.known, false) AS family_known,
           COALESCE(family.values, '{}'::text[]) AS family_values
    FROM serving_jd_current_inventory_memberships AS membership
    JOIN job_source_metadata_versions AS metadata ON metadata.id = membership.source_metadata_version_id
    LEFT JOIN LATERAL (
        SELECT true AS known,
               array_agg(DISTINCT identity.technology_key) FILTER (WHERE identity.technology_key IS NOT NULL) AS values
        FROM serving_jd_current_dimension_resolutions AS resolution
        JOIN technology_analysis_results AS result ON result.id = resolution.technology_analysis_result_id
        JOIN technology_analysis_publication_state AS publication
          ON publication.singleton = true AND publication.configuration_id = result.configuration_id
        LEFT JOIN technology_mentions AS mention ON mention.analysis_result_id = result.id
        LEFT JOIN technology_identities AS identity ON identity.id = mention.technology_identity_id
        WHERE resolution.serving_jd_inventory_membership_id = membership.id
          AND resolution.dimension = 'technology' AND result.status = 'complete'
        GROUP BY result.id
    ) AS technology ON true
    LEFT JOIN LATERAL (
		SELECT count(assignment.project_key) > 0 AS known,
               array_agg(DISTINCT assignment.project_key) FILTER (WHERE assignment.project_key IS NOT NULL) AS values
        FROM serving_jd_current_dimension_resolutions AS resolution
        JOIN project_tag_results AS result ON result.id = resolution.project_tag_result_id
        LEFT JOIN project_tag_assignments AS assignment ON assignment.project_tag_result_id = result.id
        WHERE resolution.serving_jd_inventory_membership_id = membership.id
          AND resolution.dimension = 'project' AND result.status = 'complete'
        GROUP BY result.id
    ) AS project ON true
    LEFT JOIN LATERAL (
        SELECT true AS known,
               ARRAY(
                   SELECT result.primary_family_key
                   UNION
                   SELECT match.family_key FROM job_family_matches AS match
                   WHERE match.job_family_result_id = result.id AND NOT $4::boolean
               ) AS values
        FROM serving_jd_current_dimension_resolutions AS resolution
        JOIN job_family_results AS result ON result.id = resolution.job_family_result_id
        WHERE resolution.serving_jd_inventory_membership_id = membership.id
          AND resolution.dimension = 'job_family' AND result.status = 'classified'
    ) AS family ON true
    WHERE membership.activity_state = 'effective'
), positive_memberships AS (
    SELECT membership.*
    FROM membership_facts AS membership
    WHERE ($3::text <> 'technology' OR membership.technology_known)
      AND NOT EXISTS (
          SELECT 1 FROM condition_groups AS condition
          WHERE NOT condition.excluded
            AND NOT (
                CASE condition.dimension
                    WHEN 'technology' THEN membership.technology_known
                    WHEN 'project' THEN membership.project_known
                    WHEN 'job_family' THEN membership.family_known
                    WHEN 'location' THEN membership.location_known
                    ELSE false
                END
                AND CASE condition.operator
                    WHEN 'any' THEN
                        CASE condition.dimension
                            WHEN 'technology' THEN membership.technology_values
                            WHEN 'project' THEN membership.project_values
                            WHEN 'job_family' THEN membership.family_values
                            WHEN 'location' THEN membership.locations
                            ELSE '{}'::text[]
                        END && condition.values
                    WHEN 'all' THEN
                        CASE condition.dimension
                            WHEN 'technology' THEN membership.technology_values
                            WHEN 'project' THEN membership.project_values
                            WHEN 'job_family' THEN membership.family_values
                            WHEN 'location' THEN membership.locations
                            ELSE '{}'::text[]
                        END @> condition.values
                    ELSE false
                END
            )
      )
), matched_memberships AS (
    SELECT membership.* FROM positive_memberships AS membership
    WHERE NOT EXISTS (
        SELECT 1 FROM membership_facts AS other
        JOIN condition_groups AS condition ON condition.excluded
        WHERE other.job_id = membership.job_id
          AND (
              NOT CASE condition.dimension
                  WHEN 'technology' THEN other.technology_known
                  WHEN 'project' THEN other.project_known
                  WHEN 'job_family' THEN other.family_known
                  WHEN 'location' THEN other.location_known
                  ELSE false
              END
              OR CASE condition.operator
                  WHEN 'any' THEN
                      CASE condition.dimension
                          WHEN 'technology' THEN other.technology_values
                          WHEN 'project' THEN other.project_values
                          WHEN 'job_family' THEN other.family_values
                          WHEN 'location' THEN other.locations
                          ELSE '{}'::text[]
                      END && condition.values
                  WHEN 'all' THEN
                      CASE condition.dimension
                          WHEN 'technology' THEN other.technology_values
                          WHEN 'project' THEN other.project_values
                          WHEN 'job_family' THEN other.family_values
                          WHEN 'location' THEN other.locations
                          ELSE '{}'::text[]
                      END @> condition.values
                  ELSE false
              END
          )
    )
)
SELECT NULL::text AS value, count(DISTINCT job_id)::integer AS count
FROM matched_memberships
UNION ALL
SELECT grouped.value, count(DISTINCT membership.job_id)::integer
FROM matched_memberships AS membership
CROSS JOIN LATERAL unnest(
    CASE $3::text
        WHEN 'technology' THEN membership.technology_values
        WHEN 'project' THEN membership.project_values
        WHEN 'job_family' THEN membership.family_values
        WHEN 'location' THEN membership.locations
        ELSE '{}'::text[]
    END
) AS grouped(value)
WHERE btrim(grouped.value) <> ''
GROUP BY grouped.value`
