package jdserving

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"mrolescope/internal/jdanalysis"
)

const (
	defaultPageNumber = 1
	defaultPageSize   = 20
	// fuzzyTitleSimilarityThreshold is intentionally fixed rather than client
	// controlled: title typo tolerance must not become broad body-style search.
	fuzzyTitleSimilarityThreshold = 0.30
	// MaximumPageSize bounds every list query, including repository callers
	// that bypass the HTTP handler.
	MaximumPageSize = 100
	// MaximumListSortRules bounds the public ordered list-sort interface. It
	// keeps ordering expressive enough for a reader while preserving a small,
	// predictable set of fixed SQL fragments.
	MaximumListSortRules = 4
)

// DefaultPageSize exposes the existing bounded public pagination default to
// other HTTP adapters without duplicating a magic number.
func DefaultPageSize() int { return defaultPageSize }

// ListSort is one of the supported deterministic list orders.
type ListSort string

const (
	ListSortEducationAscending        ListSort = "education_asc"
	ListSortEducationDescending       ListSort = "education_desc"
	ListSortExperienceAscending       ListSort = "experience_asc"
	ListSortExperienceDescending      ListSort = "experience_desc"
	ListSortFirstDiscoveredAscending  ListSort = "first_discovered_asc"
	ListSortFirstDiscoveredDescending ListSort = "first_discovered_desc"
	ListSortUrgentAscending           ListSort = "urgent_asc"
	ListSortUrgentDescending          ListSort = "urgent_desc"
	ListSortActivityStateAscending    ListSort = "activity_state_asc"
	ListSortActivityStateDescending   ListSort = "activity_state_desc"
	ListSortTitleAscending            ListSort = "title_asc"
	ListSortTitleDescending           ListSort = "title_desc"
	ListSortTitle                     ListSort = "title"
)

// SortField is the modeled scalar JD fact that can participate in an ordered
// list sort. Collection-valued facts remain filters only.
type SortField string

const (
	SortFieldEducation       SortField = "education"
	SortFieldExperience      SortField = "experience"
	SortFieldFirstDiscovered SortField = "first_discovered"
	SortFieldUrgent          SortField = "urgent"
	SortFieldActivityState   SortField = "activity_state"
	SortFieldTitle           SortField = "title"
)

// SortDirection controls one modeled requirement sort.
type SortDirection string

const (
	SortDirectionAscending  SortDirection = "asc"
	SortDirectionDescending SortDirection = "desc"
)

// SortRule is one ordered, validated requirement sort. Earlier rules have
// higher priority than later rules.
type SortRule struct {
	Direction SortDirection
	Field     SortField
}

// ParseListSortRules converts the public repeated sort representation into
// the small internal sort interface. It retains the legacy scalar sort so
// callers that have not moved to ordered sorts preserve their old meaning.
func ParseListSortRules(values []string) ([]SortRule, ListSort, error) {
	if len(values) == 0 {
		return nil, ListSortTitle, nil
	}
	if len(values) > MaximumListSortRules {
		return nil, "", fmt.Errorf("at most %d list sort rules are allowed", MaximumListSortRules)
	}
	if len(values) == 1 && ListSort(values[0]) == ListSortTitle {
		return nil, ListSortTitle, nil
	}

	rules := make([]SortRule, 0, len(values))
	for _, value := range values {
		var rule SortRule
		switch ListSort(value) {
		case ListSortEducationAscending:
			rule = SortRule{Field: SortFieldEducation, Direction: SortDirectionAscending}
		case ListSortEducationDescending:
			rule = SortRule{Field: SortFieldEducation, Direction: SortDirectionDescending}
		case ListSortExperienceAscending:
			rule = SortRule{Field: SortFieldExperience, Direction: SortDirectionAscending}
		case ListSortExperienceDescending:
			rule = SortRule{Field: SortFieldExperience, Direction: SortDirectionDescending}
		case ListSortFirstDiscoveredAscending:
			rule = SortRule{Field: SortFieldFirstDiscovered, Direction: SortDirectionAscending}
		case ListSortFirstDiscoveredDescending:
			rule = SortRule{Field: SortFieldFirstDiscovered, Direction: SortDirectionDescending}
		case ListSortUrgentAscending:
			rule = SortRule{Field: SortFieldUrgent, Direction: SortDirectionAscending}
		case ListSortUrgentDescending:
			rule = SortRule{Field: SortFieldUrgent, Direction: SortDirectionDescending}
		case ListSortActivityStateAscending:
			rule = SortRule{Field: SortFieldActivityState, Direction: SortDirectionAscending}
		case ListSortActivityStateDescending:
			rule = SortRule{Field: SortFieldActivityState, Direction: SortDirectionDescending}
		case ListSortTitleAscending:
			rule = SortRule{Field: SortFieldTitle, Direction: SortDirectionAscending}
		case ListSortTitleDescending:
			rule = SortRule{Field: SortFieldTitle, Direction: SortDirectionDescending}
		default:
			return nil, "", fmt.Errorf("unsupported list sort")
		}
		rules = append(rules, rule)
	}
	if err := validateSortRules(rules); err != nil {
		return nil, "", err
	}

	legacySort := ListSortTitle
	if len(values) == 1 {
		legacySort = ListSort(values[0])
	}
	return rules, legacySort, nil
}

func validateSortRules(rules []SortRule) error {
	if len(rules) > MaximumListSortRules {
		return fmt.Errorf("at most %d list sort rules are allowed", MaximumListSortRules)
	}
	seenFields := make(map[SortField]struct{}, len(rules))
	for _, rule := range rules {
		if !isSupportedSortField(rule.Field) ||
			(rule.Direction != SortDirectionAscending && rule.Direction != SortDirectionDescending) {
			return fmt.Errorf("unsupported list sort rule")
		}
		if _, exists := seenFields[rule.Field]; exists {
			return fmt.Errorf("duplicate list sort field")
		}
		seenFields[rule.Field] = struct{}{}
	}
	return nil
}

func isSupportedSortField(field SortField) bool {
	switch field {
	case SortFieldEducation, SortFieldExperience, SortFieldFirstDiscovered,
		SortFieldUrgent, SortFieldActivityState, SortFieldTitle:
		return true
	default:
		return false
	}
}

// ListJobsQuery contains only public filter semantics. Values within any one
// slice are ORed; populated filter fields are ANDed by ListJobs.
type ListJobsQuery struct {
	ActivityStates           []ActivityState
	IncludeAllActivityStates bool
	EducationLevels          []EducationLevel
	ExperienceYears          []int
	MinimumExperienceYears   *int
	FirstDiscoveredFrom      *time.Time
	FirstDiscoveredUntil     *time.Time
	IsUrgent                 *bool
	Locations                []string
	JobFamilies              []string
	MainJobFamilies          []string
	OtherJobFamilies         []string
	PageNumber               int
	PageSize                 int
	Projects                 []string
	Sort                     ListSort
	SortRules                []SortRule
	TechnologyKeywords       []string
	Title                    string
}

type listMembershipRow struct {
	ActivityState        string
	BodyContentVersionID string
	FirstDiscoveredDate  string
	ID                   string
	IsUrgent             bool
	Locations            []string
	MembershipID         string
	Title                string
	Total                int
}

// ListJobs reads only the current Serving inventory and its selected formal
// results. It deliberately never joins an AI semantic relation or an ODS raw
// payload. The internal hard limit keeps repository callers bounded while the
// HTTP boundary validates client pagination separately.
func (repository *Repository) ListJobs(ctx context.Context, query ListJobsQuery) (JobListResponse, error) {
	if repository == nil || repository.pool == nil {
		return JobListResponse{}, fmt.Errorf("serving repository is unavailable")
	}
	if err := validateSortRules(query.SortRules); err != nil {
		return JobListResponse{}, err
	}
	if err := validateMinimumExperienceYears(query.MinimumExperienceYears); err != nil {
		return JobListResponse{}, err
	}
	query = normalizeListJobsQuery(query)
	rows, err := repository.pool.Query(ctx, listJobsSQL+listJobsOrder(query.SortRules, query.Sort)+listJobsLimit,
		activityStateValues(query.ActivityStates),
		query.Title,
		query.JobFamilies,
		query.MainJobFamilies,
		query.OtherJobFamilies,
		educationLevelValues(query.EducationLevels),
		query.ExperienceYears,
		query.Projects,
		query.TechnologyKeywords,
		query.Locations,
		query.FirstDiscoveredFrom,
		query.FirstDiscoveredUntil,
		query.IsUrgent,
		query.PageSize,
		(query.PageNumber-1)*query.PageSize,
		fuzzyTitleSimilarityThreshold,
		query.MinimumExperienceYears,
	)
	if err != nil {
		return JobListResponse{}, fmt.Errorf("select serving JD list: %w", err)
	}
	defer rows.Close()

	response := JobListResponse{Items: make([]JobListItem, 0), Page: PageInfo{Number: query.PageNumber, PageSize: query.PageSize}}
	for rows.Next() {
		var row listMembershipRow
		if err := rows.Scan(
			&row.MembershipID,
			&row.ID,
			&row.Title,
			&row.Locations,
			&row.ActivityState,
			&row.FirstDiscoveredDate,
			&row.IsUrgent,
			&row.Total,
		); err != nil {
			return JobListResponse{}, fmt.Errorf("scan serving JD list: %w", err)
		}
		item, err := repository.loadListItem(ctx, row)
		if err != nil {
			return JobListResponse{}, err
		}
		response.Items = append(response.Items, item)
		response.Page.Total = row.Total
	}
	if err := rows.Err(); err != nil {
		return JobListResponse{}, fmt.Errorf("iterate serving JD list: %w", err)
	}
	return response, nil
}

func validateMinimumExperienceYears(years *int) error {
	if years != nil && (*years < 0 || *years > 9) {
		return fmt.Errorf("minimum experience years must be between 0 and 9")
	}
	return nil
}

func (repository *Repository) loadListItem(ctx context.Context, row listMembershipRow) (JobListItem, error) {
	mainFamily, otherFamilies, err := repository.loadJobFamilies(ctx, row.MembershipID)
	if err != nil {
		return JobListItem{}, err
	}
	technologies, err := repository.loadTechnologies(ctx, row.MembershipID)
	if err != nil {
		return JobListItem{}, err
	}
	education, err := repository.loadEducation(ctx, row.MembershipID)
	if err != nil {
		return JobListItem{}, err
	}
	experience, err := repository.loadExperience(ctx, row.MembershipID)
	if err != nil {
		return JobListItem{}, err
	}
	projects, err := repository.loadProjects(ctx, row.MembershipID)
	if err != nil {
		return JobListItem{}, err
	}
	return JobListItem{
		ActivityState:       ActivityState(row.ActivityState),
		Education:           education,
		Experience:          experience,
		FirstDiscoveredDate: row.FirstDiscoveredDate,
		ID:                  row.ID,
		IsUrgent:            row.IsUrgent,
		Locations:           row.Locations,
		MainJobFamily:       mainFamily,
		OtherJobFamilies:    otherFamilies,
		Projects:            projects,
		TechnologyKeywords:  technologies,
		Title:               row.Title,
	}, nil
}

func (repository *Repository) loadEducation(ctx context.Context, membershipID string) (*EducationRequirement, error) {
	var status string
	var lowerBound, upperBound *string
	err := repository.pool.QueryRow(ctx, `
SELECT result.status::text, result.lower_bound_level::text, result.upper_bound_level::text
FROM (
    SELECT *
    FROM serving_jd_dimension_resolutions
    WHERE serving_jd_inventory_membership_id = $1
      AND dimension = 'education'
    ORDER BY selection_priority, created_at DESC, id DESC
    LIMIT 1
) AS resolution
JOIN education_requirement_results AS result ON result.id = resolution.education_requirement_result_id
`, membershipID).Scan(&status, &lowerBound, &upperBound)
	if err == pgx.ErrNoRows || status == "uncertain" {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load serving education: %w", err)
	}
	requirement := &EducationRequirement{ExplicitRequirement: status == "configured"}
	if lowerBound != nil {
		requirement.LowerBound = EducationLevel(*lowerBound)
	}
	if upperBound != nil {
		requirement.UpperBound = EducationLevel(*upperBound)
	}
	return requirement, nil
}

func (repository *Repository) loadExperience(ctx context.Context, membershipID string) (*ExperienceRequirement, error) {
	var status string
	var years *int
	err := repository.pool.QueryRow(ctx, `
SELECT result.status::text, result.base_year
FROM (
    SELECT *
    FROM serving_jd_dimension_resolutions
    WHERE serving_jd_inventory_membership_id = $1
      AND dimension = 'experience'
    ORDER BY selection_priority, created_at DESC, id DESC
    LIMIT 1
) AS resolution
JOIN experience_requirement_results AS result ON result.id = resolution.experience_requirement_result_id
`, membershipID).Scan(&status, &years)
	if err == pgx.ErrNoRows || status == "uncertain" {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load serving experience: %w", err)
	}
	requirement := &ExperienceRequirement{ExplicitRequirement: status == "configured"}
	if years != nil {
		requirement.MinimumYears = *years
	}
	return requirement, nil
}

func (repository *Repository) loadJobFamilies(ctx context.Context, membershipID string) (*FilterOption, []FilterOption, error) {
	var primary *string
	err := repository.pool.QueryRow(ctx, `
SELECT result.primary_family_key
FROM (
    SELECT *
    FROM serving_jd_dimension_resolutions
    WHERE serving_jd_inventory_membership_id = $1
      AND dimension = 'job_family'
    ORDER BY selection_priority, created_at DESC, id DESC
    LIMIT 1
) AS resolution
JOIN job_family_results AS result ON result.id = resolution.job_family_result_id
`, membershipID).Scan(&primary)
	if err == pgx.ErrNoRows {
		return nil, []FilterOption{}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load serving main job family: %w", err)
	}
	if primary == nil {
		return nil, []FilterOption{}, nil
	}
	main := &FilterOption{Value: *primary, Label: jdanalysis.JobFamilyDisplayName(*primary)}
	rows, err := repository.pool.Query(ctx, `
SELECT match.family_key
FROM (
    SELECT *
    FROM serving_jd_dimension_resolutions
    WHERE serving_jd_inventory_membership_id = $1
      AND dimension = 'job_family'
    ORDER BY selection_priority, created_at DESC, id DESC
    LIMIT 1
) AS resolution
JOIN job_family_results AS result ON result.id = resolution.job_family_result_id
JOIN job_family_matches AS match ON match.job_family_result_id = result.id
WHERE match.family_key <> result.primary_family_key
ORDER BY match.family_key`, membershipID)
	if err != nil {
		return nil, nil, fmt.Errorf("load serving other job families: %w", err)
	}
	defer rows.Close()
	other := make([]FilterOption, 0)
	for rows.Next() {
		var familyKey string
		if err := rows.Scan(&familyKey); err != nil {
			return nil, nil, fmt.Errorf("scan serving other job family: %w", err)
		}
		other = append(other, FilterOption{Value: familyKey, Label: jdanalysis.JobFamilyDisplayName(familyKey)})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate serving other job families: %w", err)
	}
	return main, other, nil
}

func (repository *Repository) loadProjects(ctx context.Context, membershipID string) ([]FilterOption, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT assignment.project_key, assignment.display_name
FROM (
    SELECT *
    FROM serving_jd_dimension_resolutions
    WHERE serving_jd_inventory_membership_id = $1
      AND dimension = 'project'
    ORDER BY selection_priority, created_at DESC, id DESC
    LIMIT 1
) AS resolution
JOIN project_tag_results AS result ON result.id = resolution.project_tag_result_id
JOIN project_tag_assignments AS assignment ON assignment.project_tag_result_id = result.id
ORDER BY assignment.display_name, assignment.project_key`, membershipID)
	if err != nil {
		return nil, fmt.Errorf("load serving projects: %w", err)
	}
	defer rows.Close()
	projects := make([]FilterOption, 0)
	for rows.Next() {
		var project FilterOption
		if err := rows.Scan(&project.Value, &project.Label); err != nil {
			return nil, fmt.Errorf("scan serving project: %w", err)
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate serving projects: %w", err)
	}
	return projects, nil
}

func (repository *Repository) loadTechnologies(ctx context.Context, membershipID string) ([]FilterOption, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT identity.technology_key, identity.canonical_name
FROM (
    SELECT *
    FROM serving_jd_dimension_resolutions
    WHERE serving_jd_inventory_membership_id = $1
      AND dimension = 'technology'
    ORDER BY selection_priority, created_at DESC, id DESC
    LIMIT 1
) AS resolution
JOIN technology_analysis_results AS result ON result.id = resolution.technology_analysis_result_id
JOIN technology_mentions AS mention ON mention.analysis_result_id = result.id
JOIN technology_identities AS identity ON identity.id = mention.technology_identity_id
ORDER BY identity.canonical_name, identity.technology_key`, membershipID)
	if err != nil {
		return nil, fmt.Errorf("load serving technologies: %w", err)
	}
	defer rows.Close()
	technologies := make([]FilterOption, 0)
	for rows.Next() {
		var technology FilterOption
		if err := rows.Scan(&technology.Value, &technology.Label); err != nil {
			return nil, fmt.Errorf("scan serving technology: %w", err)
		}
		technologies = append(technologies, technology)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate serving technologies: %w", err)
	}
	return technologies, nil
}

func normalizeListJobsQuery(query ListJobsQuery) ListJobsQuery {
	if query.IncludeAllActivityStates {
		query.ActivityStates = []ActivityState{ActivityStateEffective, ActivityStateConfirmedClosed}
	} else if len(query.ActivityStates) == 0 {
		query.ActivityStates = []ActivityState{ActivityStateEffective}
	}
	if query.PageNumber < 1 {
		query.PageNumber = defaultPageNumber
	}
	if query.PageSize < 1 {
		query.PageSize = defaultPageSize
	}
	if query.PageSize > MaximumPageSize {
		query.PageSize = MaximumPageSize
	}
	if query.Sort == "" {
		query.Sort = ListSortTitle
	}
	return query
}

func listJobsOrder(sortRules []SortRule, legacySort ListSort) string {
	if len(sortRules) > 0 {
		fragments := make([]string, 0, len(sortRules)+2)
		for _, rule := range sortRules {
			switch rule {
			case SortRule{Field: SortFieldEducation, Direction: SortDirectionAscending}:
				fragments = append(fragments, listJobsEducationAscendingSortRule)
			case SortRule{Field: SortFieldEducation, Direction: SortDirectionDescending}:
				fragments = append(fragments, listJobsEducationDescendingSortRule)
			case SortRule{Field: SortFieldExperience, Direction: SortDirectionAscending}:
				fragments = append(fragments, listJobsExperienceAscendingSortRule)
			case SortRule{Field: SortFieldExperience, Direction: SortDirectionDescending}:
				fragments = append(fragments, listJobsExperienceDescendingSortRule)
			case SortRule{Field: SortFieldFirstDiscovered, Direction: SortDirectionAscending}:
				fragments = append(fragments, listJobsFirstDiscoveredAscendingSortRule)
			case SortRule{Field: SortFieldFirstDiscovered, Direction: SortDirectionDescending}:
				fragments = append(fragments, listJobsFirstDiscoveredDescendingSortRule)
			case SortRule{Field: SortFieldUrgent, Direction: SortDirectionAscending}:
				fragments = append(fragments, listJobsUrgentAscendingSortRule)
			case SortRule{Field: SortFieldUrgent, Direction: SortDirectionDescending}:
				fragments = append(fragments, listJobsUrgentDescendingSortRule)
			case SortRule{Field: SortFieldActivityState, Direction: SortDirectionAscending}:
				fragments = append(fragments, listJobsActivityStateAscendingSortRule)
			case SortRule{Field: SortFieldActivityState, Direction: SortDirectionDescending}:
				fragments = append(fragments, listJobsActivityStateDescendingSortRule)
			case SortRule{Field: SortFieldTitle, Direction: SortDirectionAscending}:
				fragments = append(fragments, listJobsTitleAscendingSortRule)
			case SortRule{Field: SortFieldTitle, Direction: SortDirectionDescending}:
				fragments = append(fragments, listJobsTitleDescendingSortRule)
			}
		}
		fragments = append(fragments, "lower(membership.title)", "identifier.public_id")
		return "\nORDER BY " + strings.Join(fragments, ",\n         ")
	}
	switch legacySort {
	case ListSortEducationAscending:
		return listJobsEducationAscendingOrder
	case ListSortEducationDescending:
		return listJobsEducationDescendingOrder
	case ListSortExperienceAscending:
		return listJobsExperienceAscendingOrder
	case ListSortExperienceDescending:
		return listJobsExperienceDescendingOrder
	case ListSortFirstDiscoveredAscending:
		return listJobsFirstDiscoveredAscendingOrder
	case ListSortFirstDiscoveredDescending:
		return listJobsFirstDiscoveredDescendingOrder
	case ListSortUrgentAscending:
		return listJobsUrgentAscendingOrder
	case ListSortUrgentDescending:
		return listJobsUrgentDescendingOrder
	case ListSortActivityStateAscending:
		return listJobsActivityStateAscendingOrder
	case ListSortActivityStateDescending:
		return listJobsActivityStateDescendingOrder
	case ListSortTitleAscending:
		return listJobsTitleAscendingOrder
	case ListSortTitleDescending:
		return listJobsTitleDescendingOrder
	default:
		return listJobsTitleOrder
	}
}

func activityStateValues(states []ActivityState) []string {
	values := make([]string, 0, len(states))
	for _, state := range states {
		values = append(values, string(state))
	}
	return values
}

func educationLevelValues(levels []EducationLevel) []string {
	values := make([]string, 0, len(levels))
	for _, level := range levels {
		values = append(values, string(level))
	}
	return values
}

const listJobsSQL = `
SELECT membership.id::text,
       identifier.public_id::text,
       membership.title,
       membership.locations,
       membership.activity_state::text,
       membership.first_discovered_at::date::text,
       membership.is_urgent,
       count(*) OVER ()::integer
FROM serving_jd_current_inventory_memberships AS membership
JOIN serving_jd_public_identifiers AS identifier ON identifier.job_id = membership.job_id
LEFT JOIN LATERAL (
    SELECT CASE
               WHEN result.status <> 'configured' THEN NULL
               WHEN COALESCE(result.lower_bound_level::text, result.upper_bound_level::text) = 'associate_or_lower' THEN 1
               WHEN COALESCE(result.lower_bound_level::text, result.upper_bound_level::text) = 'bachelor' THEN 2
               WHEN COALESCE(result.lower_bound_level::text, result.upper_bound_level::text) = 'master' THEN 3
               WHEN COALESCE(result.lower_bound_level::text, result.upper_bound_level::text) = 'doctor' THEN 4
           END AS rank
    FROM serving_jd_current_dimension_resolutions AS resolution
    JOIN education_requirement_results AS result ON result.id = resolution.education_requirement_result_id
    WHERE resolution.serving_jd_inventory_membership_id = membership.id
      AND resolution.dimension = 'education'
) AS education_sort ON true
LEFT JOIN LATERAL (
    SELECT CASE WHEN result.status = 'configured' THEN result.base_year END AS rank
    FROM serving_jd_current_dimension_resolutions AS resolution
    JOIN experience_requirement_results AS result ON result.id = resolution.experience_requirement_result_id
    WHERE resolution.serving_jd_inventory_membership_id = membership.id
      AND resolution.dimension = 'experience'
) AS experience_sort ON true
WHERE (COALESCE(cardinality($1::text[]), 0) = 0 OR membership.activity_state::text = ANY($1::text[]))
  AND (
      $2::text = ''
      OR lower(membership.title) LIKE '%' || lower($2::text) || '%'
      OR similarity(lower(membership.title), lower($2::text)) >= $16::real
  )
  AND (COALESCE(cardinality($3::text[]), 0) = 0 OR EXISTS (
	  SELECT 1
	  FROM serving_jd_current_dimension_resolutions AS resolution
	  JOIN job_family_results AS result ON result.id = resolution.job_family_result_id
	  JOIN job_family_matches AS match ON match.job_family_result_id = result.id
	  WHERE resolution.serving_jd_inventory_membership_id = membership.id
	    AND resolution.dimension = 'job_family'
	    AND match.family_key = ANY($3::text[])
  ))
  AND (COALESCE(cardinality($4::text[]), 0) = 0 OR EXISTS (
      SELECT 1
      FROM serving_jd_current_dimension_resolutions AS resolution
      JOIN job_family_results AS result ON result.id = resolution.job_family_result_id
      WHERE resolution.serving_jd_inventory_membership_id = membership.id
        AND resolution.dimension = 'job_family'
        AND result.primary_family_key = ANY($4::text[])
  ))
  AND (COALESCE(cardinality($5::text[]), 0) = 0 OR EXISTS (
      SELECT 1
      FROM serving_jd_current_dimension_resolutions AS resolution
      JOIN job_family_results AS result ON result.id = resolution.job_family_result_id
      JOIN job_family_matches AS match ON match.job_family_result_id = result.id
      WHERE resolution.serving_jd_inventory_membership_id = membership.id
        AND resolution.dimension = 'job_family'
        AND match.family_key <> result.primary_family_key
        AND match.family_key = ANY($5::text[])
  ))
  AND (COALESCE(cardinality($6::text[]), 0) = 0 OR EXISTS (
      SELECT 1
      FROM serving_jd_current_dimension_resolutions AS resolution
      JOIN education_requirement_results AS result ON result.id = resolution.education_requirement_result_id
      WHERE resolution.serving_jd_inventory_membership_id = membership.id
        AND resolution.dimension = 'education'
        AND (result.lower_bound_level::text = ANY($6::text[])
             OR result.upper_bound_level::text = ANY($6::text[]))
  ))
  AND (COALESCE(cardinality($7::integer[]), 0) = 0 OR EXISTS (
      SELECT 1
      FROM serving_jd_current_dimension_resolutions AS resolution
      JOIN experience_requirement_results AS result ON result.id = resolution.experience_requirement_result_id
      WHERE resolution.serving_jd_inventory_membership_id = membership.id
        AND resolution.dimension = 'experience'
        AND result.base_year = ANY($7::integer[])
  ))
  AND ($17::integer IS NULL OR EXISTS (
      SELECT 1
      FROM serving_jd_current_dimension_resolutions AS resolution
      JOIN experience_requirement_results AS result ON result.id = resolution.experience_requirement_result_id
      WHERE resolution.serving_jd_inventory_membership_id = membership.id
        AND resolution.dimension = 'experience'
        AND result.status = 'configured'
        AND result.base_year >= $17::integer
  ))
  AND (COALESCE(cardinality($8::text[]), 0) = 0 OR (
      ('unrecognized' = ANY($8::text[])
       AND NOT EXISTS (
           SELECT 1
           FROM serving_jd_current_dimension_resolutions AS resolution
           JOIN project_tag_results AS result ON result.id = resolution.project_tag_result_id
           JOIN project_tag_assignments AS assignment ON assignment.project_tag_result_id = result.id
          WHERE resolution.serving_jd_inventory_membership_id = membership.id
            AND resolution.dimension = 'project'
       ))
      OR EXISTS (
          SELECT 1
          FROM serving_jd_current_dimension_resolutions AS resolution
          JOIN project_tag_results AS result ON result.id = resolution.project_tag_result_id
          JOIN project_tag_assignments AS assignment ON assignment.project_tag_result_id = result.id
          WHERE resolution.serving_jd_inventory_membership_id = membership.id
            AND resolution.dimension = 'project'
            AND assignment.project_key = ANY($8::text[])
      )
  ))
  AND (COALESCE(cardinality($9::text[]), 0) = 0 OR EXISTS (
      SELECT 1
      FROM serving_jd_current_dimension_resolutions AS resolution
      JOIN technology_analysis_results AS result ON result.id = resolution.technology_analysis_result_id
      JOIN technology_mentions AS mention ON mention.analysis_result_id = result.id
      JOIN technology_identities AS identity ON identity.id = mention.technology_identity_id
      WHERE resolution.serving_jd_inventory_membership_id = membership.id
        AND resolution.dimension = 'technology'
        AND identity.technology_key = ANY($9::text[])
  ))
  AND (COALESCE(cardinality($10::text[]), 0) = 0 OR membership.locations && $10::text[])
  AND ($11::date IS NULL OR membership.first_discovered_at::date >= $11::date)
  AND ($12::date IS NULL OR membership.first_discovered_at::date <= $12::date)
  AND ($13::boolean IS NULL OR membership.is_urgent = $13::boolean)`

const listJobsTitleOrder = `
ORDER BY CASE
             WHEN $2::text = '' THEN 0
             WHEN lower(membership.title) = lower($2::text) THEN 0
             WHEN lower(membership.title) LIKE '%' || lower($2::text) || '%' THEN 1
             ELSE 2
         END,
         CASE
             WHEN $2::text = '' OR lower(membership.title) LIKE '%' || lower($2::text) || '%'
                 THEN 0
             ELSE similarity(lower(membership.title), lower($2::text))
         END DESC,
         lower(membership.title),
         identifier.public_id`

const listJobsEducationAscendingOrder = `
ORDER BY CASE WHEN education_sort.rank IS NULL THEN 1 ELSE 0 END,
         education_sort.rank ASC,
         lower(membership.title),
         identifier.public_id`

const listJobsEducationDescendingOrder = `
ORDER BY CASE WHEN education_sort.rank IS NULL THEN 1 ELSE 0 END,
         education_sort.rank DESC,
         lower(membership.title),
         identifier.public_id`

const listJobsExperienceAscendingOrder = `
ORDER BY CASE WHEN experience_sort.rank IS NULL THEN 1 ELSE 0 END,
         experience_sort.rank ASC,
         lower(membership.title),
         identifier.public_id`

const listJobsExperienceDescendingOrder = `
ORDER BY CASE WHEN experience_sort.rank IS NULL THEN 1 ELSE 0 END,
         experience_sort.rank DESC,
	         lower(membership.title),
	         identifier.public_id`

const listJobsFirstDiscoveredAscendingOrder = `
ORDER BY membership.first_discovered_at ASC NULLS LAST,
         lower(membership.title),
         identifier.public_id`

const listJobsFirstDiscoveredDescendingOrder = `
ORDER BY membership.first_discovered_at DESC NULLS LAST,
         lower(membership.title),
         identifier.public_id`

const listJobsUrgentAscendingOrder = `
ORDER BY membership.is_urgent ASC,
         lower(membership.title),
         identifier.public_id`

const listJobsUrgentDescendingOrder = `
ORDER BY membership.is_urgent DESC,
         lower(membership.title),
         identifier.public_id`

const listJobsActivityStateAscendingOrder = `
ORDER BY CASE membership.activity_state
             WHEN 'effective' THEN 0
             WHEN 'confirmed_closed' THEN 1
             ELSE 2
         END ASC,
         lower(membership.title),
         identifier.public_id`

const listJobsActivityStateDescendingOrder = `
ORDER BY CASE membership.activity_state
             WHEN 'effective' THEN 0
             WHEN 'confirmed_closed' THEN 1
             ELSE 2
         END DESC,
         lower(membership.title),
         identifier.public_id`

const listJobsTitleAscendingOrder = `
ORDER BY lower(membership.title) ASC,
         identifier.public_id`

const listJobsTitleDescendingOrder = `
ORDER BY lower(membership.title) DESC,
         identifier.public_id`

const listJobsEducationAscendingSortRule = `CASE WHEN education_sort.rank IS NULL THEN 1 ELSE 0 END,
         education_sort.rank ASC`

const listJobsEducationDescendingSortRule = `CASE WHEN education_sort.rank IS NULL THEN 1 ELSE 0 END,
         education_sort.rank DESC`

const listJobsExperienceAscendingSortRule = `CASE WHEN experience_sort.rank IS NULL THEN 1 ELSE 0 END,
         experience_sort.rank ASC`

const listJobsExperienceDescendingSortRule = `CASE WHEN experience_sort.rank IS NULL THEN 1 ELSE 0 END,
         experience_sort.rank DESC`

const listJobsFirstDiscoveredAscendingSortRule = `membership.first_discovered_at ASC NULLS LAST`

const listJobsFirstDiscoveredDescendingSortRule = `membership.first_discovered_at DESC NULLS LAST`

const listJobsUrgentAscendingSortRule = `membership.is_urgent ASC`

const listJobsUrgentDescendingSortRule = `membership.is_urgent DESC`

const listJobsActivityStateAscendingSortRule = `CASE membership.activity_state
             WHEN 'effective' THEN 0
             WHEN 'confirmed_closed' THEN 1
             ELSE 2
         END ASC`

const listJobsActivityStateDescendingSortRule = `CASE membership.activity_state
             WHEN 'effective' THEN 0
             WHEN 'confirmed_closed' THEN 1
             ELSE 2
         END DESC`

const listJobsTitleAscendingSortRule = `lower(membership.title) ASC`

const listJobsTitleDescendingSortRule = `lower(membership.title) DESC`

const listJobsLimit = `
LIMIT $14::integer OFFSET $15::integer`
