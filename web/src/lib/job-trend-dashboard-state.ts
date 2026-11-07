import { reactive } from 'vue'

export type TrendRange = 'week' | 'month' | 'quarter' | 'year' | 'all' | 'custom'

export type TrendDimension =
  | 'education'
  | 'experience'
  | 'job_family'
  | 'location'
  | 'project'
  | 'technology'

export type TrendComparison = '' | 'eq' | 'gt' | 'gte' | 'lt' | 'lte'

export interface TrendCondition {
  dimension: TrendDimension
  includeSecondary: boolean
  operator: TrendComparison
  value: string
}

export interface TrendRequest {
  condition: TrendCondition
  endDate?: string
  range: TrendRange
  startDate?: string
}

export interface TrendCustomRange {
  endDate: string
  startDate: string
}

export interface TrendPoint {
  count: number | null
  date: string
}

export interface TrendCoverage {
  first_date: string
  last_date: string
}

export interface TrendSeries {
  curve: {
    dimension: TrendDimension
    include_secondary?: boolean
    label: string
    operator?: Exclude<TrendComparison, ''>
    value: string
  }
  freshness: string
  points: TrendPoint[]
  coverage?: TrendCoverage
}

export interface TrendRequestClient {
  readTrend(query: TrendRequest, signal?: AbortSignal): Promise<TrendSeries>
}

export interface TrendCurveRow {
  appliedCondition?: TrendCondition
  condition: TrendCondition
  error: string
  id: string
  series?: TrendSeries
  status: 'error' | 'idle' | 'loading' | 'ready'
  visible?: boolean
}

export interface TrendDashboardState {
  addCurve(condition: TrendCondition): TrendCurveRow | null
  activate(): Promise<void>
  applyCurve(id: string, override?: TrendCurveRequestOverride): Promise<void>
  changeDimension(id: string, dimension: TrendDimension): boolean
  customRange?: TrendCustomRange
  range: TrendRange
  refresh(): Promise<void>
  removeCurve(id: string): boolean
  rows: TrendCurveRow[]
  setCurveVisibility(id: string, visible: boolean): boolean
  setCustomRange(startDate: string, endDate: string): Promise<void>
  setRange(range: TrendRange): Promise<void>
  updateCurve(id: string, condition: TrendCondition): boolean
}

type TrendCurveRequestOverride = {
  commitCondition?: boolean
  condition: TrendCondition
  customRange?: TrendCustomRange
  range: TrendRange
}

const maximumTrendCurves = 8
const trendCacheLifetimeMilliseconds = 10 * 60 * 1000
const trendDashboardStorageKey = 'mrolescope.job-trend-dashboard.v1'

export interface TrendDashboardStorage {
  getItem(key: string): string | null
  removeItem(key: string): void
  setItem(key: string, value: string): void
}

export interface TrendDashboardStateOptions {
  now?: () => number
  storage?: TrendDashboardStorage
}

type PersistedTrendDashboard = {
  cachedAt: number
  customRange?: TrendCustomRange
  range: TrendRange
  rows: Array<{
    appliedCondition?: TrendCondition
    condition: TrendCondition
    id: string
    series?: TrendSeries
    visible?: boolean
  }>
}

function trendResultCacheKey(
  range: TrendRange,
  condition: TrendCondition,
  customRange?: TrendCustomRange,
): string {
  const rangeKey =
    range === 'custom' && customRange !== undefined
      ? `custom:${customRange.startDate}:${customRange.endDate}`
      : range
  return `${rangeKey}:${normalizedCurveKey(condition)}`
}

function normalizedCurveKey(condition: TrendCondition): string {
  const value =
    condition.dimension === 'technology'
      ? condition.value.trim().toLocaleLowerCase()
      : condition.value.trim()
  return [condition.dimension, condition.includeSecondary, condition.operator, value].join('|')
}

function copiedCondition(condition: TrendCondition): TrendCondition {
  return {
    dimension: condition.dimension,
    includeSecondary: condition.includeSecondary,
    operator: condition.operator,
    value:
      condition.dimension === 'technology'
        ? condition.value.trim().toLocaleLowerCase()
        : condition.value.trim(),
  }
}

function isFormalTrendDimension(dimension: TrendDimension): boolean {
  return dimension !== 'location'
}

function isCompleteTrendCondition(condition: TrendCondition): boolean {
  if (condition.value.trim() === '') return false
  return (
    (condition.dimension !== 'education' && condition.dimension !== 'experience') ||
    condition.operator !== ''
  )
}

function defaultTrendDashboardStorage(): TrendDashboardStorage | undefined {
  if (typeof window === 'undefined') return undefined
  try {
    return window.sessionStorage
  } catch {
    return undefined
  }
}

function validTrendRange(value: unknown): value is TrendRange {
  return (
    value === 'week' ||
    value === 'month' ||
    value === 'quarter' ||
    value === 'year' ||
    value === 'all' ||
    value === 'custom'
  )
}

function validTrendDimension(value: unknown): value is TrendDimension {
  return (
    value === 'education' ||
    value === 'experience' ||
    value === 'job_family' ||
    value === 'location' ||
    value === 'project' ||
    value === 'technology'
  )
}

function validTrendOperator(value: unknown): value is TrendComparison {
  return (
    value === '' ||
    value === 'eq' ||
    value === 'gt' ||
    value === 'gte' ||
    value === 'lt' ||
    value === 'lte'
  )
}

function validTrendCondition(value: unknown): value is TrendCondition {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Partial<TrendCondition>
  return (
    validTrendDimension(candidate.dimension) &&
    typeof candidate.includeSecondary === 'boolean' &&
    validTrendOperator(candidate.operator) &&
    typeof candidate.value === 'string'
  )
}

function validCustomRange(value: unknown): value is TrendCustomRange {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Partial<TrendCustomRange>
  return (
    typeof candidate.startDate === 'string' &&
    typeof candidate.endDate === 'string' &&
    /^\d{4}-\d{2}-\d{2}$/u.test(candidate.startDate) &&
    /^\d{4}-\d{2}-\d{2}$/u.test(candidate.endDate) &&
    candidate.startDate <= candidate.endDate
  )
}

function copiedCustomRange(customRange: TrendCustomRange): TrendCustomRange {
  return { endDate: customRange.endDate, startDate: customRange.startDate }
}

function loadPersistedTrendDashboard(
  storage: TrendDashboardStorage | undefined,
): PersistedTrendDashboard | undefined {
  if (storage === undefined) return undefined
  try {
    const parsed: unknown = JSON.parse(storage.getItem(trendDashboardStorageKey) ?? 'null')
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return undefined
    const candidate = parsed as Partial<PersistedTrendDashboard>
    if (
      typeof candidate.cachedAt !== 'number' ||
      !Number.isFinite(candidate.cachedAt) ||
      !validTrendRange(candidate.range) ||
      (candidate.customRange !== undefined && !validCustomRange(candidate.customRange)) ||
      (candidate.range === 'custom' && !validCustomRange(candidate.customRange)) ||
      !Array.isArray(candidate.rows)
    ) {
      return undefined
    }
    const rows = candidate.rows.flatMap((row) => {
      if (
        typeof row !== 'object' ||
        row === null ||
        typeof row.id !== 'string' ||
        !validTrendCondition(row.condition) ||
        (row.appliedCondition !== undefined && !validTrendCondition(row.appliedCondition))
      ) {
        return []
      }
      return [
        {
          appliedCondition:
            row.appliedCondition === undefined
              ? row.series === undefined
                ? undefined
                : copiedCondition(row.condition)
              : copiedCondition(row.appliedCondition),
          condition: copiedCondition(row.condition),
          id: row.id,
          series: row.series,
          visible: row.visible !== false,
        },
      ]
    })
    return {
      cachedAt: candidate.cachedAt,
      customRange:
        candidate.customRange === undefined ? undefined : copiedCustomRange(candidate.customRange),
      range: candidate.range,
      rows: rows.slice(0, maximumTrendCurves),
    }
  } catch {
    return undefined
  }
}

export function createTrendDashboardState(
  _client: TrendRequestClient,
  options: TrendDashboardStateOptions = {},
): TrendDashboardState {
  const storage = options.storage ?? defaultTrendDashboardStorage()
  const now = options.now ?? Date.now
  const persisted = loadPersistedTrendDashboard(storage)
  const cachedTrendResults = new Map<string, TrendSeries>()
  for (const row of persisted?.rows ?? []) {
    if (row.series !== undefined && row.appliedCondition !== undefined) {
      cachedTrendResults.set(
        trendResultCacheKey(persisted?.range ?? 'week', row.appliedCondition, persisted?.customRange),
        row.series,
      )
    }
  }
  const requestControllers = new Map<string, AbortController>()
  const requestVersions = new Map<string, number>()
  let activated = false
  let cachedAt = persisted?.cachedAt
  const nextRequestVersion = (id: string): number => {
    const version = (requestVersions.get(id) ?? 0) + 1
    requestVersions.set(id, version)
    return version
  }
  const invalidateRequest = (id: string): void => {
    requestControllers.get(id)?.abort()
    requestControllers.delete(id)
    nextRequestVersion(id)
  }
  const persist = (): void => {
    if (storage === undefined) return
    const payload: PersistedTrendDashboard = {
      cachedAt: cachedAt ?? 0,
      customRange:
        state.customRange === undefined ? undefined : copiedCustomRange(state.customRange),
      range: state.range,
      rows: state.rows.map((row) => ({
        appliedCondition:
          row.appliedCondition === undefined ? undefined : copiedCondition(row.appliedCondition),
        condition: copiedCondition(row.condition),
        id: row.id,
        series: row.series,
        visible: row.visible !== false,
      })),
    }
    try {
      storage.setItem(trendDashboardStorageKey, JSON.stringify(payload))
    } catch {
      // Browser storage exhaustion never blocks a user-requested trend read.
    }
  }
  const cacheExpired = (): boolean =>
    cachedAt === undefined || now() - cachedAt > trendCacheLifetimeMilliseconds
  const restoreCachedResult = (
    row: TrendCurveRow,
    range: TrendRange,
    condition: TrendCondition,
    customRange?: TrendCustomRange,
  ): boolean => {
    if (cacheExpired() || !isCompleteTrendCondition(condition)) return false
    const cached = cachedTrendResults.get(trendResultCacheKey(range, condition, customRange))
    if (cached === undefined) return false
    row.error = ''
    row.series = cached
    row.status = 'ready'
    return true
  }
  const state: TrendDashboardState = reactive({
    addCurve(condition: TrendCondition): TrendCurveRow | null {
      const normalized = copiedCondition(condition)
      const hasSelectedValue = normalized.value.trim() !== ''
      if (
        state.rows.length >= maximumTrendCurves ||
        (hasSelectedValue &&
          state.rows.some(
            (row) => normalizedCurveKey(row.condition) === normalizedCurveKey(normalized),
          ))
      ) {
        return null
      }
      let ordinal = 1
      while (state.rows.some((row) => row.id === `trend-${ordinal}`)) ordinal += 1
      const row: TrendCurveRow = {
        condition: normalized,
        error: '',
        id: `trend-${ordinal}`,
        status: 'idle',
        visible: true,
      }
      state.rows.push(row)
      persist()
      return row
    },
    async activate(): Promise<void> {
      const mustRefresh = !activated || cacheExpired()
      activated = true
      if (!mustRefresh || state.rows.length === 0) return
      await Promise.all(
        state.rows
          .filter(
            (row): row is TrendCurveRow & { appliedCondition: TrendCondition } =>
              row.appliedCondition !== undefined && isCompleteTrendCondition(row.appliedCondition),
          )
          .map((row) =>
            state.applyCurve(row.id, {
              commitCondition: false,
              condition: row.appliedCondition,
              customRange: state.range === 'custom' ? state.customRange : undefined,
              range: state.range,
            }),
          ),
      )
    },
    async applyCurve(id: string, override?: TrendCurveRequestOverride): Promise<void> {
      const row = state.rows.find((candidate) => candidate.id === id)
      if (row === undefined) return
      requestControllers.get(id)?.abort()
      const controller = new AbortController()
      requestControllers.set(id, controller)
      const version = nextRequestVersion(id)
      const requestedCondition = copiedCondition(override?.condition ?? row.condition)
      const requestedRange = override?.range ?? state.range
      const requestedCustomRange =
        override?.customRange ?? (requestedRange === 'custom' ? state.customRange : undefined)
      const commitCondition = override?.commitCondition ?? true
      row.error = ''
      row.status = 'loading'
      try {
        const result = await _client.readTrend(
          {
            condition: requestedCondition,
            endDate: requestedCustomRange?.endDate,
            range: requestedRange,
            startDate: requestedCustomRange?.startDate,
          },
          controller.signal,
        )
        if (requestVersions.get(id) !== version) return
        const olderFormalFreshness = state.rows
          .filter(
            (candidate) =>
              candidate.id !== id &&
              candidate.series !== undefined &&
              candidate.appliedCondition !== undefined &&
              isFormalTrendDimension(candidate.appliedCondition.dimension),
          )
          .map((candidate) => candidate.series?.freshness)
          .filter((freshness): freshness is string => freshness !== undefined)
        if (
          isFormalTrendDimension(requestedCondition.dimension) &&
          olderFormalFreshness.some((freshness) => freshness !== result.freshness)
        ) {
          await refreshFormalCurveGroup(
            new Map([[row.id, requestedCondition]]),
            requestedRange,
            requestedCustomRange,
          )
          return
        }
        row.series = result
        row.status = 'ready'
        if (commitCondition) row.appliedCondition = requestedCondition
        cachedTrendResults.set(
          trendResultCacheKey(requestedRange, requestedCondition, requestedCustomRange),
          result,
        )
        cachedAt = now()
        persist()
      } catch {
        if (requestVersions.get(id) !== version) return
        row.error = '无法更新这条曲线'
        row.status = 'error'
        persist()
      } finally {
        if (requestVersions.get(id) === version) requestControllers.delete(id)
      }
    },
    changeDimension(id: string, dimension: TrendDimension): boolean {
      const row = state.rows.find((candidate) => candidate.id === id)
      if (row === undefined) return false
      invalidateRequest(id)
      row.condition = {
        dimension,
        includeSecondary: false,
        operator: '',
        value: '',
      }
      row.status = 'idle'
      return true
    },
    customRange: persisted?.customRange,
    range: persisted?.range ?? 'week',
    async refresh(): Promise<void> {
      await Promise.all(
        state.rows.flatMap((row) => {
          const condition = row.appliedCondition
          if (condition === undefined || !isCompleteTrendCondition(condition)) return []
          return [
            state.applyCurve(row.id, {
              commitCondition: false,
              condition,
              customRange: state.range === 'custom' ? state.customRange : undefined,
              range: state.range,
            }),
          ]
        }),
      )
    },
    removeCurve(id: string): boolean {
      const index = state.rows.findIndex((candidate) => candidate.id === id)
      if (index < 0) return false
      invalidateRequest(id)
      state.rows.splice(index, 1)
      persist()
      return true
    },
    rows: (persisted?.rows ?? []).map((row) => ({
      appliedCondition: row.appliedCondition,
      condition: copiedCondition(row.condition),
      error: '',
      id: row.id,
      series: row.series,
      status: row.series === undefined ? 'idle' : 'ready',
      visible: row.visible !== false,
    })),
    setCurveVisibility(id: string, visible: boolean): boolean {
      const row = state.rows.find((candidate) => candidate.id === id)
      if (row === undefined) return false
      row.visible = visible
      persist()
      return true
    },
    async setCustomRange(startDate: string, endDate: string): Promise<void> {
      const customRange = { endDate, startDate }
      if (!validCustomRange(customRange)) return
      state.customRange = copiedCustomRange(customRange)
      state.range = 'custom'
      const pending = state.rows.flatMap((row) => {
        const condition = row.appliedCondition
        if (condition === undefined || !isCompleteTrendCondition(condition)) return []
        if (restoreCachedResult(row, 'custom', condition, customRange)) return []
        return [
          state.applyCurve(row.id, {
            commitCondition: false,
            condition,
            customRange,
            range: 'custom',
          }),
        ]
      })
      persist()
      await Promise.all(pending)
    },
    async setRange(range: TrendRange): Promise<void> {
      if (range === 'custom') {
        if (state.customRange !== undefined)
          await state.setCustomRange(state.customRange.startDate, state.customRange.endDate)
        return
      }
      if (state.range === range) return
      state.range = range
      const pending = state.rows.flatMap((row) => {
        const condition = row.appliedCondition
        if (condition === undefined || !isCompleteTrendCondition(condition)) return []
        if (restoreCachedResult(row, range, condition)) return []
        return [
          state.applyCurve(row.id, {
            commitCondition: false,
            condition,
            range,
          }),
        ]
      })
      persist()
      await Promise.all(pending)
    },
    updateCurve(id: string, condition: TrendCondition): boolean {
      const row = state.rows.find((candidate) => candidate.id === id)
      if (row === undefined) return false
      const normalized = copiedCondition(condition)
      if (
        state.rows.some(
          (candidate) =>
            candidate.id !== id &&
            normalizedCurveKey(candidate.condition) === normalizedCurveKey(normalized),
        )
      ) {
        return false
      }
      invalidateRequest(id)
      row.condition = normalized
      row.error = ''
      row.status = 'idle'
      return true
    },
  })
  const refreshFormalCurveGroup = async (
    requestedConditions: Map<string, TrendCondition>,
    requestedRange: TrendRange,
    requestedCustomRange?: TrendCustomRange,
  ): Promise<void> => {
    const group = state.rows.flatMap((row) => {
      const condition = requestedConditions.get(row.id) ?? row.appliedCondition
      if (
        condition === undefined ||
        !isFormalTrendDimension(condition.dimension) ||
        !isCompleteTrendCondition(condition)
      ) {
        return []
      }
      return [{ condition, row }]
    })
    if (group.length === 0) return
    const requests = group.map(({ condition, row }) => {
      requestControllers.get(row.id)?.abort()
      const controller = new AbortController()
      requestControllers.set(row.id, controller)
      const version = nextRequestVersion(row.id)
      row.error = ''
      row.status = 'loading'
      return {
        controller,
        condition,
        row,
        version,
        promise: _client.readTrend(
          {
            condition: copiedCondition(condition),
            endDate: requestedCustomRange?.endDate,
            range: requestedRange,
            startDate: requestedCustomRange?.startDate,
          },
          controller.signal,
        ),
      }
    })
    try {
      const results = await Promise.all(
        requests.map(async (request) => ({
          request,
          series: await request.promise,
        })),
      )
      if (results.some(({ request }) => requestVersions.get(request.row.id) !== request.version))
        return
      const freshness = results[0]?.series.freshness
      if (
        freshness === undefined ||
        results.some((result) => result.series.freshness !== freshness)
      ) {
        for (const { request } of results) {
          request.row.error = '数据版本正在更新，请稍后重新应用。'
          request.row.status = 'error'
        }
        persist()
        return
      }
      for (const [cacheKey, cached] of cachedTrendResults) {
        if (cached.freshness !== freshness) cachedTrendResults.delete(cacheKey)
      }
      for (const { request, series } of results) {
        request.row.series = series
        request.row.appliedCondition = request.condition
        request.row.status = 'ready'
        cachedTrendResults.set(
          trendResultCacheKey(requestedRange, request.condition, requestedCustomRange),
          series,
        )
      }
      cachedAt = now()
      persist()
    } catch {
      for (const { row, version } of requests) {
        if (requestVersions.get(row.id) !== version) continue
        row.error = '无法更新这条曲线'
        row.status = 'error'
      }
      persist()
    } finally {
      for (const { row, version } of requests) {
        if (requestVersions.get(row.id) === version) requestControllers.delete(row.id)
      }
    }
  }
  return state
}
