import type { LocationQuery, LocationQueryRaw } from 'vue-router'

export type ActivityState = 'effective' | 'confirmed_closed'
export type EducationLevel = 'associate_or_lower' | 'bachelor' | 'master' | 'doctor'
export type SortKind =
  | 'education'
  | 'experience'
  | 'first_discovered'
  | 'urgent'
  | 'activity_state'
  | 'title'
export type SortDirection = 'asc' | 'desc'
export type RequirementSort =
  | 'education_asc'
  | 'education_desc'
  | 'experience_asc'
  | 'experience_desc'
  | 'first_discovered_asc'
  | 'first_discovered_desc'
  | 'urgent_asc'
  | 'urgent_desc'
  | 'activity_state_asc'
  | 'activity_state_desc'
  | 'title_asc'
  | 'title_desc'

const allActivityStates: ActivityState[] = ['effective', 'confirmed_closed']

export interface JDListState {
  activityStates: ActivityState[]
  educationLevels: EducationLevel[]
  minimumExperienceYear: number | undefined
  firstDiscoveredFrom: string
  firstDiscoveredUntil: string
  isUrgent: boolean | undefined
  locations: string[]
  jobFamilies: string[]
  page: number
  pageSize: number
  projects: string[]
  sortRules: RequirementSort[]
  technologyKeywords: string[]
  title: string
}

const defaultListState: JDListState = {
  activityStates: ['effective'],
  educationLevels: [],
  minimumExperienceYear: undefined,
  firstDiscoveredFrom: '',
  firstDiscoveredUntil: '',
  isUrgent: undefined,
  locations: [],
  jobFamilies: [],
  page: 1,
  pageSize: 20,
  projects: [],
  sortRules: [],
  technologyKeywords: [],
  title: '',
}

const permittedActivityStates = new Set<ActivityState>(['effective', 'confirmed_closed'])
const permittedEducationLevels = new Set<EducationLevel>([
  'associate_or_lower',
  'bachelor',
  'master',
  'doctor',
])
const permittedRequirementSorts = new Set<RequirementSort>([
  'education_asc',
  'education_desc',
  'experience_asc',
  'experience_desc',
  'first_discovered_asc',
  'first_discovered_desc',
  'urgent_asc',
  'urgent_desc',
  'activity_state_asc',
  'activity_state_desc',
  'title_asc',
  'title_desc',
])

function asValues(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.filter((item): item is string => typeof item === 'string' && item.length > 0)
  }
  return typeof value === 'string' && value.length > 0 ? [value] : []
}

function firstValue(value: unknown): string | undefined {
  return asValues(value)[0]
}

function positiveInteger(value: string | undefined, fallback: number): number {
  if (value === undefined || !/^\d+$/.test(value)) {
    return fallback
  }
  const parsed = Number(value)
  return parsed > 0 ? parsed : fallback
}

function selectedValues(values: string[]): string | string[] | undefined {
  if (values.length === 0) {
    return undefined
  }
  return values.length === 1 ? values[0] : values
}

function uniqueValues(...valueSets: string[][]): string[] {
  return [...new Set(valueSets.flat())]
}

function requirementSortRules(values: string[]): RequirementSort[] {
  const rules: RequirementSort[] = []
  for (const value of values) {
    if (
      permittedRequirementSorts.has(value as RequirementSort) &&
      !rules.includes(value as RequirementSort) &&
      rules.length < 2
    ) {
      rules.push(value as RequirementSort)
    }
  }
  return rules
}

function minimumExperienceYear(value: string | undefined): number | undefined {
  if (value === undefined || !/^\d+$/.test(value)) {
    return undefined
  }
  const parsed = Number(value)
  return parsed >= 0 && parsed <= 9 ? parsed : undefined
}

/**
 * Produces the list state represented by a JD browser URL. Invalid state is
 * ignored locally; the API remains the authority for request validation.
 */
export function listStateFromRoute(query: LocationQuery | LocationQueryRaw): JDListState {
  const requestedActivityStates = asValues(query.activity_state)
  const activityStates = requestedActivityStates.filter((value): value is ActivityState =>
    permittedActivityStates.has(value as ActivityState),
  )
  const educationLevels = asValues(query.education).filter((value): value is EducationLevel =>
    permittedEducationLevels.has(value as EducationLevel),
  )
  const urgent = firstValue(query.urgent)
  const legacyJobFamilies = uniqueValues(
    asValues(query.main_job_family),
    asValues(query.other_job_family),
  )

  return {
    activityStates: requestedActivityStates.includes('all')
      ? [...allActivityStates]
      : activityStates.length > 0
        ? activityStates
        : defaultListState.activityStates,
    educationLevels,
    minimumExperienceYear: minimumExperienceYear(firstValue(query.minimum_experience_year)),
    firstDiscoveredFrom: firstValue(query.first_discovered_from) ?? '',
    firstDiscoveredUntil: firstValue(query.first_discovered_until) ?? '',
    isUrgent: urgent === 'true' ? true : urgent === 'false' ? false : undefined,
    locations: asValues(query.location),
    jobFamilies: uniqueValues(asValues(query.job_family), legacyJobFamilies),
    page: positiveInteger(firstValue(query.page), defaultListState.page),
    pageSize: positiveInteger(firstValue(query.page_size), defaultListState.pageSize),
    projects: asValues(query.project),
    sortRules: requirementSortRules(asValues(query.sort)),
    technologyKeywords: asValues(query.technology),
    title: firstValue(query.title) ?? '',
  }
}

/**
 * Serializes all list controls, including the default activity scope, so a
 * copied URL has the same meaning as the currently visible list.
 */
export function listStateToRoute(state: JDListState): LocationQueryRaw {
  return {
    activity_state: selectedValues(activityStateQueryValues(state.activityStates)),
    education: selectedValues(state.educationLevels),
    minimum_experience_year:
      state.minimumExperienceYear === undefined ? undefined : String(state.minimumExperienceYear),
    first_discovered_from: state.firstDiscoveredFrom || undefined,
    first_discovered_until: state.firstDiscoveredUntil || undefined,
    urgent: state.isUrgent === undefined ? undefined : String(state.isUrgent),
    location: selectedValues(state.locations),
    job_family: selectedValues(state.jobFamilies),
    page: state.page === defaultListState.page ? undefined : String(state.page),
    page_size: state.pageSize === defaultListState.pageSize ? undefined : String(state.pageSize),
    project: selectedValues(state.projects),
    sort: selectedValues(state.sortRules),
    technology: selectedValues(state.technologyKeywords),
    title: state.title || undefined,
  }
}

export function activityStateQueryValues(activityStates: ActivityState[]): string[] {
  const selected = new Set(activityStates)
  if (allActivityStates.every((state) => selected.has(state))) {
    return ['all']
  }
  return activityStates
}

export function createDefaultListState(): JDListState {
  return {
    ...defaultListState,
    activityStates: [...defaultListState.activityStates],
  }
}
