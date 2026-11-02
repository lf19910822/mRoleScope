package jdserving

import (
	"context"
	"fmt"

	"mrolescope/internal/jdanalysis"
)

// ListFilterOptions returns only current, display-safe options for the JD
// browser. It does not read source payloads, provenance, audit facts, or AI
// results. Values are controlled logical keys rather than database IDs.
func (repository *Repository) ListFilterOptions(ctx context.Context) (FilterOptionsResponse, error) {
	if repository == nil || repository.pool == nil {
		return FilterOptionsResponse{}, fmt.Errorf("serving repository is unavailable")
	}
	response := FilterOptionsResponse{
		ActivityStates: []FilterOption{
			{Value: "effective", Label: "仍有效"},
			{Value: "confirmed_closed", Label: "已确认下线"},
			{Value: "all", Label: "全部"},
		},
		EducationLevels: []FilterOption{
			{Value: string(EducationLevelAssociateOrLower), Label: "专科及以下"},
			{Value: string(EducationLevelBachelor), Label: "本科"},
			{Value: string(EducationLevelMaster), Label: "硕士"},
			{Value: string(EducationLevelDoctor), Label: "博士"},
		},
		ExperienceYears:    make([]FilterOption, 0),
		JobFamilies:        make([]FilterOption, 0),
		Locations:          make([]FilterOption, 0),
		MainJobFamilies:    make([]FilterOption, 0),
		OtherJobFamilies:   make([]FilterOption, 0),
		Projects:           make([]FilterOption, 0),
		TechnologyKeywords: make([]FilterOption, 0),
		UrgentOptions: []FilterOption{
			{Value: "true", Label: "紧急招聘"},
			{Value: "false", Label: "非紧急"},
		},
	}

	jobFamilyKeys, err := repository.filterOptionStrings(ctx, jobFamilyOptionsSQL)
	if err != nil {
		return FilterOptionsResponse{}, err
	}
	for _, key := range jobFamilyKeys {
		response.JobFamilies = append(response.JobFamilies, FilterOption{
			Value: key,
			Label: jdanalysis.JobFamilyDisplayName(key),
		})
	}
	mainFamilyKeys, err := repository.filterOptionStrings(ctx, mainJobFamilyOptionsSQL)
	if err != nil {
		return FilterOptionsResponse{}, err
	}
	for _, key := range mainFamilyKeys {
		response.MainJobFamilies = append(response.MainJobFamilies, FilterOption{
			Value: key,
			Label: jdanalysis.JobFamilyDisplayName(key),
		})
	}
	otherFamilyKeys, err := repository.filterOptionStrings(ctx, otherJobFamilyOptionsSQL)
	if err != nil {
		return FilterOptionsResponse{}, err
	}
	for _, key := range otherFamilyKeys {
		response.OtherJobFamilies = append(response.OtherJobFamilies, FilterOption{
			Value: key,
			Label: jdanalysis.JobFamilyDisplayName(key),
		})
	}
	if response.TechnologyKeywords, err = repository.filterOptions(ctx, technologyOptionsSQL); err != nil {
		return FilterOptionsResponse{}, err
	}
	if response.Projects, err = repository.filterOptions(ctx, projectOptionsSQL); err != nil {
		return FilterOptionsResponse{}, err
	}
	if response.Locations, err = repository.filterOptions(ctx, locationOptionsSQL); err != nil {
		return FilterOptionsResponse{}, err
	}
	if response.ExperienceYears, err = repository.filterOptions(ctx, experienceOptionsSQL); err != nil {
		return FilterOptionsResponse{}, err
	}
	var hasUnrecognizedProject bool
	if err := repository.pool.QueryRow(ctx, unrecognizedProjectOptionSQL).Scan(&hasUnrecognizedProject); err != nil {
		return FilterOptionsResponse{}, fmt.Errorf("read serving unrecognized-project option: %w", err)
	}
	if hasUnrecognizedProject {
		response.Projects = append(response.Projects, FilterOption{Value: "unrecognized", Label: "未识别项目"})
	}
	return response, nil
}

func (repository *Repository) filterOptionStrings(ctx context.Context, query string) ([]string, error) {
	rows, err := repository.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read serving filter-option keys: %w", err)
	}
	defer rows.Close()
	values := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("scan serving filter-option key: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate serving filter-option keys: %w", err)
	}
	return values, nil
}

func (repository *Repository) filterOptions(ctx context.Context, query string) ([]FilterOption, error) {
	rows, err := repository.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read serving filter options: %w", err)
	}
	defer rows.Close()
	options := make([]FilterOption, 0)
	for rows.Next() {
		var option FilterOption
		if err := rows.Scan(&option.Value, &option.Label); err != nil {
			return nil, fmt.Errorf("scan serving filter option: %w", err)
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate serving filter options: %w", err)
	}
	return options, nil
}

const jobFamilyOptionsSQL = `
SELECT DISTINCT match.family_key
FROM serving_jd_current_dimension_resolutions AS resolution
JOIN job_family_results AS result ON result.id = resolution.job_family_result_id
JOIN job_family_matches AS match ON match.job_family_result_id = result.id
WHERE resolution.dimension = 'job_family'
ORDER BY match.family_key`

const mainJobFamilyOptionsSQL = `
SELECT DISTINCT result.primary_family_key
FROM serving_jd_current_dimension_resolutions AS resolution
JOIN job_family_results AS result ON result.id = resolution.job_family_result_id
WHERE resolution.dimension = 'job_family'
  AND result.primary_family_key IS NOT NULL
ORDER BY result.primary_family_key`

const otherJobFamilyOptionsSQL = `
SELECT DISTINCT match.family_key
FROM serving_jd_current_dimension_resolutions AS resolution
JOIN job_family_results AS result ON result.id = resolution.job_family_result_id
JOIN job_family_matches AS match ON match.job_family_result_id = result.id
WHERE resolution.dimension = 'job_family'
  AND (result.primary_family_key IS NULL OR match.family_key <> result.primary_family_key)
ORDER BY match.family_key`

const technologyOptionsSQL = `
SELECT DISTINCT identity.technology_key, identity.canonical_name
FROM serving_jd_current_dimension_resolutions AS resolution
JOIN technology_analysis_results AS result ON result.id = resolution.technology_analysis_result_id
JOIN technology_mentions AS mention ON mention.analysis_result_id = result.id
JOIN technology_identities AS identity ON identity.id = mention.technology_identity_id
WHERE resolution.dimension = 'technology'
ORDER BY identity.canonical_name, identity.technology_key`

const projectOptionsSQL = `
SELECT DISTINCT assignment.project_key, assignment.display_name
FROM serving_jd_current_dimension_resolutions AS resolution
JOIN project_tag_results AS result ON result.id = resolution.project_tag_result_id
JOIN project_tag_assignments AS assignment ON assignment.project_tag_result_id = result.id
WHERE resolution.dimension = 'project'
ORDER BY assignment.display_name, assignment.project_key`

const locationOptionsSQL = `
SELECT DISTINCT location, location
FROM serving_jd_current_inventory_memberships AS membership
CROSS JOIN LATERAL unnest(membership.locations) AS location
ORDER BY location`

const experienceOptionsSQL = `
SELECT option_value, option_label
FROM (
    SELECT DISTINCT result.base_year AS sort_year,
           result.base_year::text AS option_value,
           result.base_year::text || ' 年' AS option_label
    FROM serving_jd_current_dimension_resolutions AS resolution
    JOIN experience_requirement_results AS result ON result.id = resolution.experience_requirement_result_id
    WHERE resolution.dimension = 'experience'
      AND result.status = 'configured'
      AND result.base_year IS NOT NULL
) AS options
ORDER BY sort_year`

const unrecognizedProjectOptionSQL = `
SELECT EXISTS (
    SELECT 1
    FROM serving_jd_current_inventory_memberships AS membership
    WHERE NOT EXISTS (
        SELECT 1
        FROM serving_jd_current_dimension_resolutions AS resolution
        JOIN project_tag_results AS result ON result.id = resolution.project_tag_result_id
        JOIN project_tag_assignments AS assignment ON assignment.project_tag_result_id = result.id
        WHERE resolution.serving_jd_inventory_membership_id = membership.id
          AND resolution.dimension = 'project'
    )
)`
