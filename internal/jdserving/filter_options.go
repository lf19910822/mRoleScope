package jdserving

import (
	"context"
	"fmt"
	"sort"

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
		TechnologyKeywords: make([]TechnologyKeywordFilterOption, 0),
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
	if response.TechnologyKeywords, err = repository.technologyFilterOptions(ctx); err != nil {
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

type technologyCategoryDefinition struct {
	category FilterOption
	rank     int
}

var technologyCategoryDefinitions = map[string]technologyCategoryDefinition{
	"programming-language": {
		category: FilterOption{Value: "programming-languages", Label: "编程语言"},
		rank:     10,
	},
	"frontend-framework": {
		category: FilterOption{Value: "frontend-frameworks", Label: "前端框架"},
		rank:     20,
	},
	"graphics-api-or-language": {
		category: FilterOption{Value: "graphics-api-and-shaders", Label: "图形 API 与着色语言"},
		rank:     30,
	},
	"graphics-technique": {
		category: FilterOption{Value: "graphics-techniques", Label: "图形技术"},
		rank:     40,
	},
	"game-engine": {
		category: FilterOption{Value: "game-engines", Label: "游戏引擎"},
		rank:     50,
	},
	"database-or-storage": {
		category: FilterOption{Value: "databases-and-storage", Label: "数据库与存储"},
		rank:     60,
	},
	"data-processing-or-queue": {
		category: FilterOption{Value: "data-processing-and-queues", Label: "数据计算与队列"},
		rank:     70,
	},
	"distributed-datastore": {
		category: FilterOption{Value: "distributed-datastores", Label: "分布式数据存储"},
		rank:     80,
	},
	"storage": {
		category: FilterOption{Value: "storage", Label: "存储"},
		rank:     90,
	},
	"cloud-native-component": {
		category: FilterOption{Value: "cloud-native", Label: "容器与云原生组件"},
		rank:     100,
	},
	"container-orchestration": {
		category: FilterOption{Value: "container-orchestration", Label: "容器编排"},
		rank:     110,
	},
	"gateway-or-service-mesh": {
		category: FilterOption{Value: "gateway-and-service-mesh", Label: "网关与服务治理"},
		rank:     120,
	},
	"observability": {
		category: FilterOption{Value: "observability", Label: "可观测性"},
		rank:     130,
	},
	"engineering-or-delivery-tool": {
		category: FilterOption{Value: "engineering-and-delivery", Label: "工程与交付工具"},
		rank:     140,
	},
	"developer-tool": {
		category: FilterOption{Value: "developer-tools", Label: "开发工具"},
		rank:     150,
	},
	"operations-concept": {
		category: FilterOption{Value: "operations-concepts", Label: "运维理念"},
		rank:     160,
	},
	"platform": {
		category: FilterOption{Value: "platforms", Label: "平台"},
		rank:     170,
	},
	"machine-learning-framework": {
		category: FilterOption{Value: "machine-learning-frameworks", Label: "机器学习框架"},
		rank:     180,
	},
	"model-or-training-tool": {
		category: FilterOption{Value: "model-and-training-tools", Label: "模型与训练工具"},
		rank:     190,
	},
	"training-tool": {
		category: FilterOption{Value: "training-tools", Label: "训练工具"},
		rank:     200,
	},
	"model-serving": {
		category: FilterOption{Value: "model-serving", Label: "模型推理服务"},
		rank:     210,
	},
	"ai-method-or-mechanism": {
		category: FilterOption{Value: "ai-methods-and-mechanisms", Label: "AI 方法与机制"},
		rank:     220,
	},
	"evaluation-tool-or-benchmark": {
		category: FilterOption{Value: "evaluation-tools-and-benchmarks", Label: "评测工具与基准"},
		rank:     230,
	},
	"gpu-or-parallel-computing": {
		category: FilterOption{Value: "gpu-and-parallel-computing", Label: "GPU 与并行计算"},
		rank:     240,
	},
	"protocol-or-interface-specification": {
		category: FilterOption{Value: "protocols-and-interfaces", Label: "协议与接口规范"},
		rank:     250,
	},
	"rpc-framework": {
		category: FilterOption{Value: "rpc-frameworks", Label: "RPC 框架"},
		rank:     260,
	},
	"security-tool": {
		category: FilterOption{Value: "security-tools", Label: "安全工具"},
		rank:     270,
	},
	"scripting-environment": {
		category: FilterOption{Value: "scripting-environments", Label: "脚本环境"},
		rank:     280,
	},
}

var otherTechnologyCategory = technologyCategoryDefinition{
	category: FilterOption{Value: "other", Label: "其他技术"},
	rank:     1_000,
}

func technologyCategoryForKind(kind string) technologyCategoryDefinition {
	if definition, found := technologyCategoryDefinitions[kind]; found {
		return definition
	}
	return otherTechnologyCategory
}

func (repository *Repository) technologyFilterOptions(ctx context.Context) ([]TechnologyKeywordFilterOption, error) {
	rows, err := repository.pool.Query(ctx, technologyOptionsSQL)
	if err != nil {
		return nil, fmt.Errorf("read serving technology filter options: %w", err)
	}
	defer rows.Close()

	type categorizedOption struct {
		option TechnologyKeywordFilterOption
		rank   int
	}
	options := make([]categorizedOption, 0)
	for rows.Next() {
		var technologyKey, label, kind string
		if err := rows.Scan(&technologyKey, &label, &kind); err != nil {
			return nil, fmt.Errorf("scan serving technology filter option: %w", err)
		}
		definition := technologyCategoryForKind(kind)
		options = append(options, categorizedOption{
			option: TechnologyKeywordFilterOption{
				Category: definition.category,
				Label:    label,
				Value:    technologyKey,
			},
			rank: definition.rank,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate serving technology filter options: %w", err)
	}
	sort.SliceStable(options, func(left, right int) bool {
		return options[left].rank < options[right].rank
	})
	result := make([]TechnologyKeywordFilterOption, 0, len(options))
	for _, option := range options {
		result = append(result, option.option)
	}
	return result, nil
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
SELECT DISTINCT identity.technology_key, identity.canonical_name, identity.technology_kind
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
