import { activityStateQueryValues, type JDListState } from '@/lib/list-state'
import type { JDDetailBodySection, TechnologyHighlightRange } from '@/lib/highlighted-text'
import type {
  TrendEventStatsResponse,
  TrendRequest,
  TrendSeries,
} from '@/lib/job-trend-dashboard-state'
import { requestActivity } from '@/lib/request-activity'

export type { JDDetailBodySection, TechnologyHighlightRange }

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
  last_modified_date?: string | null
  id: string
  is_urgent: boolean
  locations: string[]
  main_job_family?: FilterOption
  other_job_families: FilterOption[]
  projects: FilterOption[]
  technology_keywords: FilterOption[]
  title: string
}

export type ComparisonStatus = 'available' | 'unavailable'
export type ComparisonFragmentKind = 'unchanged' | 'deleted' | 'inserted'

export interface ComparisonFragment {
  kind: ComparisonFragmentKind
  text: string
}

export interface ComparisonDimensionChange {
  field: string
  historical: string
  latest: string
}

export interface ComparisonBodySection {
  fragments: ComparisonFragment[]
  label: string
  section: JDDetailBodySection
}

export interface JobComparison {
  body_sections?: ComparisonBodySection[]
  dimension_changes?: ComparisonDimensionChange[]
  message?: string
  retryable?: boolean
  status: ComparisonStatus
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
    technology_highlights?: TechnologyHighlightRange[]
  }
  comparison?: JobComparison
  history_date?: string
  history_node_type?: HistoryNodeType
}

export type HistoryNodeType = 'discovered' | 'modified' | 'confirmed_closed' | 'reopened' | 'latest'

export type HistoryNodeTag =
  | 'discovered'
  | 'latest'
  | 'dimension'
  | 'body_responsibilities'
  | 'body_requirements'
  | 'body_supplemental'
  | 'confirmed_closed'
  | 'reopened'

export interface HistoryNode {
  can_view_historical: boolean
  connect_previous: boolean
  date: string
  summary: string
  tags: HistoryNodeTag[]
  type: HistoryNodeType
}

export interface JobHistoryResponse {
  nodes: HistoryNode[]
}

export type TrendDrilldownStatus = 'current' | 'changed' | 'confirmed_closed'

export interface TrendDrilldownRequest {
  condition: TrendRequest['condition']
  count: number
  date?: string
  page: number
  pageSize: number
  title: string
  event?: 'entered' | 'modified' | 'left' | 'reopened'
  endDate?: string
  startDate?: string
}

export interface TrendDrilldownItem {
  can_view_historical: boolean
  can_view_latest: boolean
  id: string
  event_date?: string
  reason?: string
  status: TrendDrilldownStatus
  title: string
}

export interface TrendDrilldownResponse {
  count_matches_expected?: boolean
  items: TrendDrilldownItem[]
  matched_total: number
  node_date?: string
  page: PageInfo
  event_type?: 'entered' | 'modified' | 'left' | 'reopened'
  mode?: 'event'
  period_end?: string
  period_start?: string
  reason_counts?: Record<string, number>
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

export interface AITrendDraftCurve {
  condition: TrendRequest['condition']
  label: string
  match_status: 'exact' | 'alias' | 'possible' | 'expansion'
  reason: string
  scope?: string
  selected: boolean
  sub_query_ids: string[]
}

export interface AITrendDraft {
  contract_version: string
  status: 'complete' | 'partial' | 'stopped'
  curves: AITrendDraftCurve[]
  warnings?: string[]
  explanation?: string
}

export interface AITrendStreamEvent {
  type: 'operation' | 'validation' | 'draft' | 'done'
  status: 'running' | 'completed' | 'failed' | 'success' | 'partial' | 'failure' | 'user_stopped'
  tool?: string
  params?: Record<string, unknown>
  draft?: AITrendDraft
  error?: { code: string; field?: string; message: string }
}

export async function streamAITrendDraft(
  query: string,
  onEvent: (event: AITrendStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const settle = requestActivity.begin()
  try {
    const response = await fetch('/api/v1/job-trends/ai-draft', {
      method: 'POST',
      headers: { Accept: 'application/x-ndjson', 'Content-Type': 'application/json' },
      body: JSON.stringify({ query }),
      signal,
    })
    if (!response.ok) throw new JDAPIError(`生成曲线草稿失败（${response.status}）`, response.status)
    if (response.body === null) throw new JDAPIError('服务器没有返回操作流', response.status)
    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let pending = ''
    while (true) {
      const chunk = await reader.read()
      pending += decoder.decode(chunk.value ?? new Uint8Array(), { stream: !chunk.done })
      const lines = pending.split('\n')
      pending = lines.pop() ?? ''
      for (const line of lines) {
        if (line.trim() !== '') onEvent(JSON.parse(line) as AITrendStreamEvent)
      }
      if (chunk.done) break
    }
    if (pending.trim() !== '') onEvent(JSON.parse(pending) as AITrendStreamEvent)
  } finally {
    settle()
  }
}

export class JDAPIError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message)
  }
}

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const settle = requestActivity.begin()
  try {
    const response = await fetch(path, { headers: { Accept: 'application/json' }, signal })
    if (!response.ok) {
      if (
        response.status === 401 &&
        typeof window !== 'undefined' &&
        window.location.pathname !== '/access'
      ) {
        window.location.assign('/access')
      }
      throw new JDAPIError(`请求岗位数据失败（${response.status}）`, response.status)
    }
    return (await response.json()) as T
  } finally {
    settle()
  }
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

export function getJobDetail(
  id: string,
  version: { date: string; mode: 'historical' } | { mode: 'latest' } = { mode: 'latest' },
): Promise<JobDetailResponse> {
  if (version.mode === 'latest')
    return getJSON<JobDetailResponse>(`/api/v1/jobs/${encodeURIComponent(id)}`)
  const parameters = new URLSearchParams({ date: version.date, version: version.mode })
  return getJSON<JobDetailResponse>(
    `/api/v1/jobs/${encodeURIComponent(id)}/detail?${parameters.toString()}`,
  )
}

export function getJobHistory(id: string, signal?: AbortSignal): Promise<JobHistoryResponse> {
  return getJSON<JobHistoryResponse>(jobHistoryPath(id), signal)
}

export function jobHistoryPath(id: string): string {
  return `/api/v1/jobs/${encodeURIComponent(id)}/history`
}

export function getJobHistoryDetail(
  id: string,
  date: string,
  type: HistoryNodeType,
  signal?: AbortSignal,
): Promise<JobDetailResponse> {
  return getJSON<JobDetailResponse>(jobHistoryDetailPath(id, date, type), signal)
}

export function jobHistoryDetailPath(id: string, date: string, type: HistoryNodeType): string {
  const parameters = new URLSearchParams({ date, type })
  return `/api/v1/jobs/${encodeURIComponent(id)}/history/detail?${parameters.toString()}`
}

export function getFilterOptions(): Promise<FilterOptionsResponse> {
  return getJSON<FilterOptionsResponse>('/api/v1/jobs/filter-options')
}

export function jobTrendPath(query: TrendRequest): string {
  const parameters = new URLSearchParams()
  parameters.set('range', query.range)
  parameters.set('dimension', query.condition.dimension)
  parameters.set('value', query.condition.value)
  if (query.range === 'custom') {
    if (query.startDate) parameters.set('start_date', query.startDate)
    if (query.endDate) parameters.set('end_date', query.endDate)
  }
  if (query.condition.operator) parameters.set('operator', query.condition.operator)
  if (query.condition.dimension === 'job_family') {
    parameters.set('include_secondary', String(query.condition.includeSecondary))
  }
  return `/api/v1/job-trends?${parameters.toString()}`
}

export function getJobTrend(query: TrendRequest, signal?: AbortSignal): Promise<TrendSeries> {
  return getJSON<TrendSeries>(jobTrendPath(query), signal)
}

export function jobTrendEventsPath(query: TrendRequest): string {
  return `/api/v1/job-trends/events?${new URLSearchParams({
    range: query.range,
    dimension: query.condition.dimension,
    value: query.condition.value,
    ...(query.range === 'custom' && query.startDate ? { start_date: query.startDate } : {}),
    ...(query.range === 'custom' && query.endDate ? { end_date: query.endDate } : {}),
    ...(query.condition.operator ? { operator: query.condition.operator } : {}),
    ...(query.condition.dimension === 'job_family' ? { include_secondary: String(query.condition.includeSecondary) } : {}),
  }).toString()}`
}

export function getJobTrendEvents(
  query: TrendRequest,
  signal?: AbortSignal,
): Promise<TrendEventStatsResponse> {
  return getJSON<TrendEventStatsResponse>(jobTrendEventsPath(query), signal)
}

export function jobTrendDrilldownPath(query: TrendDrilldownRequest): string {
  const parameters = new URLSearchParams()
  if (query.date) parameters.set('date', query.date)
  parameters.set('count', String(query.count))
  parameters.set('page', String(query.page))
  parameters.set('page_size', String(query.pageSize))
  if (query.title.trim()) parameters.set('title', query.title.trim())
  parameters.set('dimension', query.condition.dimension)
  parameters.set('value', query.condition.value)
  if (query.condition.operator) parameters.set('operator', query.condition.operator)
  if (query.condition.dimension === 'job_family') {
    parameters.set('include_secondary', String(query.condition.includeSecondary))
  }
  if (query.event) {
    parameters.set('event', query.event)
    if (query.startDate) parameters.set('start_date', query.startDate)
    if (query.endDate) parameters.set('end_date', query.endDate)
    parameters.delete('date')
  }
  return `/api/v1/job-trends/drilldown?${parameters.toString()}`
}

export function getJobTrendDrilldown(
  query: TrendDrilldownRequest,
  signal?: AbortSignal,
): Promise<TrendDrilldownResponse> {
  return getJSON<TrendDrilldownResponse>(jobTrendDrilldownPath(query), signal)
}
