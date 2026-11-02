import { activityStateQueryValues, type JDListState } from '@/lib/list-state'

export type ActivityState = 'effective' | 'confirmed_closed'
export type EducationLevel = 'associate_or_lower' | 'bachelor' | 'master' | 'doctor'

export interface FilterOption {
  label: string
  value: string
}

export interface TechnologyKeywordFilterOption extends FilterOption {
  category: FilterOption
}

export interface EducationRequirement {
  explicit_requirement: boolean
  lower_bound?: EducationLevel
  upper_bound?: EducationLevel
}

export interface ExperienceRequirement {
  explicit_requirement: boolean
  minimum_years?: number
}

export interface JobListItem {
  activity_state: ActivityState
  education?: EducationRequirement
  experience?: ExperienceRequirement
  first_discovered_date: string
  id: string
  is_urgent: boolean
  locations: string[]
  main_job_family?: FilterOption
  other_job_families: FilterOption[]
  projects: FilterOption[]
  technology_keywords: FilterOption[]
  title: string
}

export interface PageInfo {
  number: number
  page_size: number
  total: number
}

export interface JobListResponse {
  items: JobListItem[]
  page: PageInfo
}

export interface JobDetailResponse extends JobListItem {
  body: {
    requirements: string
    responsibilities: string
    supplemental?: string
  }
}

export interface FilterOptionsResponse {
  activity_states: FilterOption[]
  education_levels: FilterOption[]
  experience_years: FilterOption[]
  job_families: FilterOption[]
  locations: FilterOption[]
  main_job_families: FilterOption[]
  other_job_families: FilterOption[]
  projects: FilterOption[]
  technology_keywords: TechnologyKeywordFilterOption[]
  urgent_options: FilterOption[]
}

export class JDAPIError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message)
  }
}

async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, { headers: { Accept: 'application/json' } })
  if (!response.ok) {
    throw new JDAPIError(`请求岗位数据失败（${response.status}）`, response.status)
  }
  return (await response.json()) as T
}

function appendAll(parameters: URLSearchParams, name: string, values: string[]): void {
  for (const value of values) {
    parameters.append(name, value)
  }
}

export function jobListPath(state: JDListState): string {
  const parameters = new URLSearchParams()
  appendAll(parameters, 'activity_state', activityStateQueryValues(state.activityStates))
  appendAll(parameters, 'education', state.educationLevels)
  appendAll(parameters, 'location', state.locations)
  appendAll(parameters, 'job_family', state.jobFamilies)
  appendAll(parameters, 'project', state.projects)
  appendAll(parameters, 'technology', state.technologyKeywords)
  if (state.firstDiscoveredFrom) parameters.set('first_discovered_from', state.firstDiscoveredFrom)
  if (state.firstDiscoveredUntil)
    parameters.set('first_discovered_until', state.firstDiscoveredUntil)
  if (state.isUrgent !== undefined) parameters.set('urgent', String(state.isUrgent))
  if (state.minimumExperienceYear !== undefined) {
    parameters.set('minimum_experience_year', String(state.minimumExperienceYear))
  }
  if (state.title) parameters.set('title', state.title)
  parameters.set('page', String(state.page))
  parameters.set('page_size', String(state.pageSize))
  appendAll(parameters, 'sort', state.sortRules)
  return `/api/v1/jobs?${parameters.toString()}`
}

export function getJobList(state: JDListState): Promise<JobListResponse> {
  return getJSON<JobListResponse>(jobListPath(state))
}

export function getJobDetail(id: string): Promise<JobDetailResponse> {
  return getJSON<JobDetailResponse>(`/api/v1/jobs/${encodeURIComponent(id)}`)
}

export function getFilterOptions(): Promise<FilterOptionsResponse> {
  return getJSON<FilterOptionsResponse>('/api/v1/jobs/filter-options')
}
