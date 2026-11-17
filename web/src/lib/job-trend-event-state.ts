import { reactive } from 'vue'

import type {
  TrendCondition,
  TrendCustomRange,
  TrendCurveRow,
  TrendEventComparison,
  TrendEventStatsResponse,
  TrendEventType,
  TrendRange,
} from './job-trend-dashboard-state'

export interface TrendEventRequestClient {
  readTrendEvents(
    request: {
      condition: TrendCondition
      endDate?: string
      range: TrendRange
      startDate?: string
    },
    signal?: AbortSignal,
  ): Promise<TrendEventStatsResponse>
}

export interface TrendEventStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
}

export interface TrendEventState {
  error: string
  isLoading: boolean
  response?: TrendEventStatsResponse
  retry(): Promise<void>
  selectedCurveId: string
  selectCurve(id: string, rows: TrendCurveRow[]): Promise<void>
  selectNext(rows: TrendCurveRow[]): Promise<void>
  selectPrevious(rows: TrendCurveRow[]): Promise<void>
  status: 'idle' | 'loading' | 'ready' | 'error'
  load(
    rows: TrendCurveRow[],
    range: TrendRange,
    customRange?: TrendCustomRange,
    force?: boolean,
  ): Promise<void>
}

export const trendEventTypes: TrendEventType[] = ['entered', 'modified', 'left', 'reopened']

const cacheLifetimeMilliseconds = 10 * 60 * 1000
const maximumCacheEntries = 64
const storageKey = 'mrolescope.job-trend-events.v1'
const eventSemanticsVersion = 'events-v1'

interface TrendEventWindowKey {
  end: string
  start: string
}

interface TrendEventPeriodsKey {
  current: TrendEventWindowKey
  previous: TrendEventWindowKey
  yearOverYear: TrendEventWindowKey
}

export function supportsTrendEvents(row: TrendCurveRow | undefined): boolean {
  const dimension = row?.appliedCondition?.dimension ?? row?.condition.dimension
  return (
    dimension === 'technology' ||
    dimension === 'job_family' ||
    dimension === 'project' ||
    dimension === 'location'
  )
}

export function trendEventCacheKey(
  row: TrendCurveRow,
  range: TrendRange,
  customRange?: TrendCustomRange,
): string | undefined {
  const condition = row.appliedCondition
  if (condition === undefined || !supportsTrendEvents(row)) return undefined
  const rangeKey =
    range === 'custom' && customRange
      ? `${range}:${customRange.startDate}:${customRange.endDate}`
      : range
  const periods = trendEventPeriodsKey(row, range, customRange)
  const freshness = row.series?.freshness ?? ''
  const coverage = row.series?.coverage
  return [
    eventSemanticsVersion,
    rangeKey,
    periods === undefined
      ? ''
      : [
          periods.current.start,
          periods.current.end,
          periods.previous.start,
          periods.previous.end,
          periods.yearOverYear.start,
          periods.yearOverYear.end,
        ].join(':'),
    freshness,
    coverage?.first_date ?? '',
    coverage?.last_date ?? '',
    condition.dimension,
    condition.includeSecondary,
    condition.operator,
    condition.dimension === 'technology'
      ? condition.value.trim().toLocaleLowerCase()
      : condition.value.trim(),
  ].join('|')
}

function parseCalendarDate(value: string | undefined): Date | undefined {
  if (value === undefined || !/^\d{4}-\d{2}-\d{2}$/u.test(value)) return undefined
  const [year, month, day] = value.split('-').map(Number)
  const date = new Date(Date.UTC(year!, month! - 1, day!))
  return date.toISOString().slice(0, 10) === value ? date : undefined
}

function formatCalendarDate(value: Date): string {
  return value.toISOString().slice(0, 10)
}

function addDays(value: Date, days: number): Date {
  const result = new Date(value)
  result.setUTCDate(result.getUTCDate() + days)
  return result
}

function addYears(value: Date, years: number): Date {
  const result = new Date(Date.UTC(value.getUTCFullYear() + years, value.getUTCMonth(), 1))
  const lastDay = new Date(
    Date.UTC(result.getUTCFullYear(), result.getUTCMonth() + 1, 0),
  ).getUTCDate()
  result.setUTCDate(Math.min(value.getUTCDate(), lastDay))
  return result
}

function addMonths(value: Date, months: number): Date {
  const result = new Date(Date.UTC(value.getUTCFullYear(), value.getUTCMonth() + months, 1))
  const lastDay = new Date(
    Date.UTC(result.getUTCFullYear(), result.getUTCMonth() + 1, 0),
  ).getUTCDate()
  result.setUTCDate(Math.min(value.getUTCDate(), lastDay))
  return result
}

function lastDayOfMonth(value: Date): Date {
  return new Date(Date.UTC(value.getUTCFullYear(), value.getUTCMonth() + 1, 0))
}

function daysInclusive(start: Date, end: Date): number {
  return Math.round((end.getTime() - start.getTime()) / 86_400_000) + 1
}

function naturalPeriodIsComplete(range: TrendRange, start: Date, end: Date): boolean {
  switch (range) {
    case 'week':
      return daysInclusive(start, end) === 7
    case 'month':
      return start.getUTCDate() === 1 && end.getTime() === lastDayOfMonth(end).getTime()
    case 'quarter': {
      const quarterEndMonth = Math.floor(start.getUTCMonth() / 3) * 3 + 2
      const quarterEnd = lastDayOfMonth(
        new Date(Date.UTC(start.getUTCFullYear(), quarterEndMonth, 1)),
      )
      return start.getUTCDate() === 1 && end.getTime() === quarterEnd.getTime()
    }
    case 'year':
      return (
        start.getUTCMonth() === 0 &&
        start.getUTCDate() === 1 &&
        end.getUTCMonth() === 11 &&
        end.getUTCDate() === 31
      )
    default:
      return false
  }
}

function trendEventPeriodsKey(
  row: TrendCurveRow,
  range: TrendRange,
  customRange?: TrendCustomRange,
): TrendEventPeriodsKey | undefined {
  let currentStart: Date | undefined
  let currentEnd: Date | undefined
  if (range === 'custom') {
    currentStart = parseCalendarDate(customRange?.startDate)
    currentEnd = parseCalendarDate(customRange?.endDate)
  } else {
    const latestPoint = [...(row.series?.points ?? [])]
      .reverse()
      .find((point) => point.count !== null)
    currentEnd = parseCalendarDate(latestPoint?.date ?? row.series?.coverage?.last_date)
    if (currentEnd !== undefined) {
      currentStart = new Date(currentEnd)
      switch (range) {
        case 'week':
          currentStart = addDays(currentEnd, -((currentEnd.getUTCDay() + 6) % 7))
          break
        case 'month':
          currentStart = new Date(
            Date.UTC(currentEnd.getUTCFullYear(), currentEnd.getUTCMonth(), 1),
          )
          break
        case 'quarter':
          currentStart = new Date(
            Date.UTC(currentEnd.getUTCFullYear(), Math.floor(currentEnd.getUTCMonth() / 3) * 3, 1),
          )
          break
        case 'year':
          currentStart = new Date(Date.UTC(currentEnd.getUTCFullYear(), 0, 1))
          break
        default:
          return undefined
      }
    }
  }
  if (currentStart === undefined || currentEnd === undefined || currentStart > currentEnd) {
    return undefined
  }

  const length = daysInclusive(currentStart, currentEnd)
  let previousStart: Date
  let previousEnd: Date
  if (range === 'month' && naturalPeriodIsComplete(range, currentStart, currentEnd)) {
    previousStart = new Date(
      Date.UTC(currentStart.getUTCFullYear(), currentStart.getUTCMonth() - 1, 1),
    )
    previousEnd = addDays(previousStart, -1)
    previousEnd = lastDayOfMonth(previousStart)
  } else if (range === 'quarter' && naturalPeriodIsComplete(range, currentStart, currentEnd)) {
    previousStart = addMonths(currentStart, -3)
    previousEnd = addDays(currentStart, -1)
  } else if (range === 'year' && naturalPeriodIsComplete(range, currentStart, currentEnd)) {
    previousStart = addYears(currentStart, -1)
    previousEnd = addDays(currentStart, -1)
  } else if (range === 'month') {
    previousStart = addMonths(currentStart, -1)
    previousEnd = addDays(previousStart, length - 1)
  } else {
    previousStart = addDays(currentStart, -length)
    previousEnd = addDays(currentStart, -1)
  }

  let yearOverYearStart = addYears(currentStart, -1)
  let yearOverYearEnd = addYears(currentEnd, -1)
  if (range === 'week') {
    while (yearOverYearStart.getUTCDay() !== currentStart.getUTCDay()) {
      yearOverYearStart = addDays(yearOverYearStart, 1)
      yearOverYearEnd = addDays(yearOverYearEnd, 1)
    }
  }
  const windowKey = (start: Date, end: Date): TrendEventWindowKey => ({
    end: formatCalendarDate(end),
    start: formatCalendarDate(start),
  })
  return {
    current: windowKey(currentStart, currentEnd),
    previous: windowKey(previousStart, previousEnd),
    yearOverYear: windowKey(yearOverYearStart, yearOverYearEnd),
  }
}

function defaultStorage(): TrendEventStorage | undefined {
  if (typeof window === 'undefined') return undefined
  try {
    return window.sessionStorage
  } catch {
    return undefined
  }
}

function isTrendEventType(value: unknown): value is TrendEventType {
  return typeof value === 'string' && trendEventTypes.includes(value as TrendEventType)
}

function isFiniteNumberOrNull(value: unknown): value is number | null {
  return value === null || (typeof value === 'number' && Number.isFinite(value))
}

function isNonNegativeIntegerOrNull(value: unknown): value is number | null {
  return value === null || (typeof value === 'number' && Number.isSafeInteger(value) && value >= 0)
}

function isNonNegativeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
}

function isTrendEventPeriod(value: unknown): value is { start: string; end: string } {
  if (typeof value !== 'object' || value === null) return false
  const period = value as { start?: unknown; end?: unknown }
  return typeof period.start === 'string' && typeof period.end === 'string'
}

function copyResponse(response: unknown): TrendEventStatsResponse | undefined {
  if (typeof response !== 'object' || response === null) return undefined
  const candidate = response as Record<string, unknown>
  const rawCurve = candidate.curve
  const current = candidate.current
  const previous = candidate.previous
  const yearOverYear = candidate.year_over_year
  const rawRows = candidate.rows
  if (
    typeof rawCurve !== 'object' ||
    rawCurve === null ||
    !isTrendEventPeriod(current) ||
    !isTrendEventPeriod(previous) ||
    !isTrendEventPeriod(yearOverYear) ||
    !Array.isArray(rawRows) ||
    (candidate.status !== 'available' && candidate.status !== 'unavailable')
  )
    return undefined
  const curve = rawCurve as Record<string, unknown>
  if (
    typeof curve.dimension !== 'string' ||
    typeof curve.label !== 'string' ||
    typeof curve.value !== 'string' ||
    (curve.include_secondary !== undefined && typeof curve.include_secondary !== 'boolean') ||
    (curve.operator !== undefined && typeof curve.operator !== 'string')
  )
    return undefined
  const rows = rawRows
    .filter((row): row is Record<string, unknown> => {
      if (typeof row !== 'object' || row === null) return false
      const candidateRow = row as Record<string, unknown>
      const previousRow = candidateRow.previous
      const yearOverYearRow = candidateRow.year_over_year
      return (
        isTrendEventType(candidateRow.type) &&
        isNonNegativeInteger(candidateRow.current) &&
        typeof previousRow === 'object' &&
        previousRow !== null &&
        isNonNegativeIntegerOrNull((previousRow as Record<string, unknown>).count) &&
        isFiniteNumberOrNull((previousRow as Record<string, unknown>).rate) &&
        typeof yearOverYearRow === 'object' &&
        yearOverYearRow !== null &&
        isNonNegativeIntegerOrNull((yearOverYearRow as Record<string, unknown>).count) &&
        isFiniteNumberOrNull((yearOverYearRow as Record<string, unknown>).rate)
      )
    })
    .map((row) => ({
      current: row.current as number,
      previous: {
        count: (row.previous as Record<string, unknown>).count as number | null,
        rate: (row.previous as Record<string, unknown>).rate as number | null,
      },
      type: row.type as TrendEventType,
      year_over_year: {
        count: (row.year_over_year as Record<string, unknown>).count as number | null,
        rate: (row.year_over_year as Record<string, unknown>).rate as number | null,
      },
    }))
  if (rows.length !== rawRows.length) return undefined
  const safeCurve = {
    dimension: curve.dimension as TrendEventStatsResponse['curve']['dimension'],
    label: curve.label as string,
    value: curve.value as string,
    ...(curve.include_secondary === undefined
      ? {}
      : { include_secondary: curve.include_secondary as boolean }),
    ...(curve.operator === undefined
      ? {}
      : { operator: curve.operator as TrendEventStatsResponse['curve']['operator'] }),
  }
  return {
    ...(typeof candidate.as_of === 'string' ? { as_of: candidate.as_of } : {}),
    current: { end: current.end, start: current.start },
    curve: safeCurve,
    ...(typeof candidate.freshness === 'string' ? { freshness: candidate.freshness } : {}),
    previous: { end: previous.end, start: previous.start },
    rows,
    status: candidate.status as TrendEventStatsResponse['status'],
    year_over_year: { end: yearOverYear.end, start: yearOverYear.start },
  }
}

export function formatTrendEventComparison(value: TrendEventComparison): string {
  if (value.count === null) return '暂无可比数据'
  return `${value.count}（${formatTrendEventRate(value.rate)}）`
}

export function formatTrendEventRate(rate: number | null): string {
  if (rate === null) return '-'
  const sign = rate > 0 ? '+' : ''
  return `${sign}${rate.toFixed(1)}%`
}

export function trendEventTypeLabel(type: TrendEventType): string {
  switch (type) {
    case 'entered':
      return '新增'
    case 'modified':
      return '修改'
    case 'left':
      return '下线'
    case 'reopened':
      return '重上线'
  }
}

export function createTrendEventState(
  client: TrendEventRequestClient,
  options: { now?: () => number; storage?: TrendEventStorage } = {},
): TrendEventState {
  const now = options.now ?? Date.now
  const storage = options.storage ?? defaultStorage()
  const cache = new Map<string, { cachedAt: number; response: TrendEventStatsResponse }>()
  let controller: AbortController | undefined
  let generation = 0
  try {
    const persisted = JSON.parse(storage?.getItem(storageKey) ?? 'null') as Record<
      string,
      unknown
    > | null
    if (persisted !== null && typeof persisted === 'object') {
      for (const [key, value] of Object.entries(persisted)) {
        if (typeof value !== 'object' || value === null) continue
        const entry = value as { cachedAt?: unknown; response?: unknown }
        const safeResponse = copyResponse(entry.response)
        if (typeof entry.cachedAt === 'number' && safeResponse !== undefined) {
          cache.set(key, { cachedAt: entry.cachedAt, response: safeResponse })
        }
      }
    }
  } catch {
    // Invalid session state is ignored and replaced by the next successful read.
  }

  pruneCache()

  function pruneCache(): void {
    const timestamp = now()
    for (const [key, entry] of cache) {
      if (timestamp - entry.cachedAt > cacheLifetimeMilliseconds) cache.delete(key)
    }
    if (cache.size <= maximumCacheEntries) return
    const oldest = [...cache.entries()]
      .sort((left, right) => left[1].cachedAt - right[1].cachedAt)
      .slice(0, cache.size - maximumCacheEntries)
    for (const [key] of oldest) cache.delete(key)
  }

  const persist = (): void => {
    if (storage === undefined) return
    pruneCache()
    try {
      const payload: Record<string, unknown> = {}
      for (const [key, entry] of cache) payload[key] = entry
      storage.setItem(storageKey, JSON.stringify(payload))
    } catch {
      // Storage is an optimization and never blocks the event read.
    }
  }

  const state: TrendEventState = reactive({
    error: '',
    isLoading: false,
    response: undefined,
    async retry(): Promise<void> {
      await state.load(currentRows, currentRange, currentCustomRange, true)
    },
    selectedCurveId: '',
    async selectCurve(id: string, rows: TrendCurveRow[]): Promise<void> {
      const row = rows.find((candidate) => candidate.id === id)
      if (row === undefined) return
      state.selectedCurveId = id
      await state.load(rows, currentRange, currentCustomRange)
    },
    async selectNext(rows: TrendCurveRow[]): Promise<void> {
      const appliedRows = rows.filter((row) => row.appliedCondition !== undefined)
      const index = appliedRows.findIndex((row) => row.id === state.selectedCurveId)
      const next = index >= 0 ? appliedRows[index + 1] : appliedRows[0]
      if (next !== undefined) await state.selectCurve(next.id, rows)
    },
    async selectPrevious(rows: TrendCurveRow[]): Promise<void> {
      const appliedRows = rows.filter((row) => row.appliedCondition !== undefined)
      const index = appliedRows.findIndex((row) => row.id === state.selectedCurveId)
      const previous = index > 0 ? appliedRows[index - 1] : index < 0 ? appliedRows[0] : undefined
      if (previous !== undefined) await state.selectCurve(previous.id, rows)
    },
    status: 'idle',
    async load(
      rows: TrendCurveRow[],
      range: TrendRange,
      customRange?: TrendCustomRange,
      force = false,
    ): Promise<void> {
      currentRows = rows
      currentRange = range
      currentCustomRange = customRange
      if (range === 'all') {
        controller?.abort()
        controller = undefined
        generation += 1
        state.response = undefined
        state.status = 'idle'
        state.error = ''
        state.isLoading = false
        return
      }
      const appliedRows = rows.filter((row) => row.appliedCondition !== undefined)
      if (appliedRows.length === 0) {
        controller?.abort()
        controller = undefined
        generation += 1
        state.selectedCurveId = ''
        state.response = undefined
        state.status = 'idle'
        state.error = ''
        state.isLoading = false
        return
      }
      if (!state.selectedCurveId || !appliedRows.some((row) => row.id === state.selectedCurveId)) {
        state.selectedCurveId = appliedRows[0]!.id
      }
      const row = appliedRows.find((candidate) => candidate.id === state.selectedCurveId)
      if (!supportsTrendEvents(row)) {
        controller?.abort()
        controller = undefined
        generation += 1
        state.response = undefined
        state.status = 'idle'
        state.error = ''
        state.isLoading = false
        return
      }
      if (row!.status !== 'ready') {
        controller?.abort()
        controller = undefined
        generation += 1
        state.response = undefined
        state.error = ''
        state.status = row!.status === 'loading' ? 'loading' : 'idle'
        state.isLoading = row!.status === 'loading'
        return
      }
      const condition = row!.appliedCondition!
      const key = trendEventCacheKey(row!, range, customRange)
      controller?.abort()
      controller = undefined
      const requestGeneration = ++generation
      const cached = key === undefined ? undefined : cache.get(key)
      const cachedResponse = cached === undefined ? undefined : copyResponse(cached.response)
      if (cached !== undefined && cachedResponse === undefined) cache.delete(key!)
      if (
        !force &&
        cached !== undefined &&
        cachedResponse !== undefined &&
        now() - cached.cachedAt <= cacheLifetimeMilliseconds
      ) {
        state.response = cachedResponse
        state.status = 'ready'
        state.error = ''
        state.isLoading = false
        return
      }
      const nextController = new AbortController()
      controller = nextController
      state.response = undefined
      state.error = ''
      state.status = 'loading'
      state.isLoading = true
      try {
        const response = await client.readTrendEvents(
          {
            condition,
            endDate: customRange?.endDate,
            range,
            startDate: customRange?.startDate,
          },
          nextController.signal,
        )
        if (requestGeneration !== generation || state.selectedCurveId !== row!.id) return
        const safeResponse = copyResponse(response)
        if (safeResponse === undefined) throw new Error('invalid event response')
        state.response = safeResponse
        state.status = 'ready'
        if (key !== undefined) cache.set(key, { cachedAt: now(), response: safeResponse })
        persist()
      } catch {
        if (requestGeneration !== generation) return
        state.error = '统计暂不可用'
        state.status = 'error'
      } finally {
        if (requestGeneration === generation) {
          state.isLoading = false
          controller = undefined
        }
      }
    },
  })
  let currentRows: TrendCurveRow[] = []
  let currentRange: TrendRange = 'week'
  let currentCustomRange: TrendCustomRange | undefined
  return state
}
