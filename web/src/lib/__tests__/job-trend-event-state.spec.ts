import { describe, expect, it, vi } from 'vitest'

import {
  createTrendEventState,
  formatTrendEventComparison,
  formatTrendEventRate,
  trendEventCacheKey,
} from '../job-trend-event-state'
import type { TrendCurveRow, TrendEventStatsResponse } from '../job-trend-dashboard-state'

const response: TrendEventStatsResponse = {
  current: { start: '2026-11-01', end: '2026-11-12' },
  curve: { dimension: 'technology', label: 'C++', value: 'c++' },
  freshness: 'fresh-1',
  previous: { start: '2026-10-01', end: '2026-10-12' },
  rows: [
    {
      current: 9,
      previous: { count: 7, rate: 28.6 },
      type: 'entered',
      year_over_year: { count: 10, rate: -10 },
    },
  ],
  status: 'available',
  year_over_year: { start: '2025-11-01', end: '2025-11-12' },
}

function row(
  id: string,
  dimension: TrendCurveRow['condition']['dimension'] = 'technology',
): TrendCurveRow {
  const condition = {
    dimension,
    includeSecondary: false,
    operator: '',
    value: 'c++',
  } as TrendCurveRow['condition']
  return {
    appliedCondition: condition,
    condition,
    error: '',
    id,
    series: { curve: { dimension, label: 'C++', value: 'c++' }, freshness: 'fresh-1', points: [] },
    status: 'ready',
  }
}

describe('trend event state', () => {
  it('selects the first applied curve and only requests supported dimensions', async () => {
    const readTrendEvents = vi.fn().mockResolvedValue(response)
    const state = createTrendEventState(
      { readTrendEvents },
      { storage: { getItem: () => null, setItem: () => undefined } },
    )
    const rows = [row('trend-1'), row('trend-2', 'education')]

    await state.load(rows, 'month')

    expect(state.selectedCurveId).toBe('trend-1')
    expect(readTrendEvents).toHaveBeenCalledTimes(1)
    await state.selectNext(rows)
    expect(state.selectedCurveId).toBe('trend-2')
    expect(readTrendEvents).toHaveBeenCalledTimes(1)
    expect(state.response).toBeUndefined()
    await state.selectNext(rows)
    expect(state.selectedCurveId).toBe('trend-2')
    await state.selectPrevious(rows)
    expect(state.selectedCurveId).toBe('trend-1')
    await state.selectPrevious(rows)
    expect(state.selectedCurveId).toBe('trend-1')
  })

  it('clears stale numbers, ignores late responses, and retries only event reads', async () => {
    let resolveFirst: ((value: TrendEventStatsResponse) => void) | undefined
    let resolveSecond: ((value: TrendEventStatsResponse) => void) | undefined
    const first = new Promise<TrendEventStatsResponse>((resolve) => {
      resolveFirst = resolve
    })
    const second = new Promise<TrendEventStatsResponse>((resolve) => {
      resolveSecond = resolve
    })
    const readTrendEvents = vi.fn().mockReturnValueOnce(first).mockReturnValueOnce(second)
    const state = createTrendEventState({ readTrendEvents })
    const rows = [row('trend-1'), row('trend-2')]

    const firstLoad = state.load(rows, 'month')
    const secondLoad = state.selectCurve('trend-2', rows)
    expect(state.response).toBeUndefined()
    resolveFirst?.(response)
    await firstLoad
    resolveSecond?.(response)
    await secondLoad
    expect(state.selectedCurveId).toBe('trend-2')
    expect(state.response).toEqual(response)

    readTrendEvents.mockRejectedValueOnce(new Error('temporary'))
    await state.load(rows, 'month', undefined, true)
    expect(state.status).toBe('error')
    readTrendEvents.mockResolvedValueOnce(response)
    await state.retry()
    expect(state.status).toBe('ready')
  })

  it('uses an isolated ten-minute cache key and formats zero denominators safely', async () => {
    const storage = new Map<string, string>()
    const readTrendEvents = vi.fn().mockResolvedValue(response)
    const state = createTrendEventState(
      { readTrendEvents },
      {
        now: () => 1000,
        storage: {
          getItem: (key) => storage.get(key) ?? null,
          setItem: (key, value) => storage.set(key, value),
        },
      },
    )
    const rows = [row('trend-1')]
    await state.load(rows, 'month')
    await state.load(rows, 'month')
    expect(readTrendEvents).toHaveBeenCalledTimes(1)
    expect(trendEventCacheKey(rows[0]!, 'month')).toContain('fresh-1')
    expect(formatTrendEventComparison({ count: 0, rate: null })).toBe('0（-）')
    expect(formatTrendEventComparison({ count: 7, rate: 28.6 })).toBe('7（+28.6%）')
    expect(formatTrendEventRate(null)).toBe('-')
    expect(formatTrendEventRate(-100)).toBe('-100.0%')
  })

  it('persists only the display-safe event response fields', async () => {
    const storage = new Map<string, string>()
    const unsafeResponse = {
      ...response,
      job_id: 'internal-job-id',
      source_metadata_version_id: 'internal-source-version',
    } as TrendEventStatsResponse & {
      job_id: string
      source_metadata_version_id: string
    }
    const state = createTrendEventState(
      { readTrendEvents: vi.fn().mockResolvedValue(unsafeResponse) },
      {
        storage: {
          getItem: (key) => storage.get(key) ?? null,
          setItem: (key, value) => storage.set(key, value),
        },
      },
    )

    await state.load([row('trend-1')], 'month')

    const persisted = storage.get('mrolescope.job-trend-events.v1')
    expect(persisted).toBeDefined()
    expect(persisted).not.toContain('internal-job-id')
    expect(persisted).not.toContain('source_metadata_version_id')
    expect(JSON.stringify(state.response)).not.toContain('internal-job-id')
  })

  it('ignores malformed persisted responses instead of exposing stale cache data', async () => {
    const storage = new Map<string, string>([
      [
        'mrolescope.job-trend-events.v1',
        JSON.stringify({
          'invalid-entry': {
            cachedAt: 1000,
            response: { current: { start: 'not-a-period' } },
          },
        }),
      ],
    ])
    const readTrendEvents = vi.fn().mockResolvedValue(response)
    const state = createTrendEventState(
      { readTrendEvents },
      {
        now: () => 1000,
        storage: {
          getItem: (key) => storage.get(key) ?? null,
          setItem: (key, value) => storage.set(key, value),
        },
      },
    )

    await state.load([row('trend-1')], 'month')

    expect(readTrendEvents).toHaveBeenCalledTimes(1)
    expect(state.response?.current).toEqual(response.current)
  })

  it('keeps nullable comparison counts for an unavailable formal result', async () => {
    const unavailable = {
      ...response,
      rows: response.rows.map((row) => ({
        ...row,
        previous: { count: null, rate: null },
        year_over_year: { count: null, rate: null },
      })),
      status: 'unavailable' as const,
    }
    const state = createTrendEventState(
      { readTrendEvents: vi.fn().mockResolvedValue(unavailable) },
      { storage: { getItem: () => null, setItem: () => undefined } },
    )

    await state.load([row('trend-1')], 'month')

    expect(state.status).toBe('ready')
    expect(state.response?.status).toBe('unavailable')
    expect(state.response?.rows[0]?.previous.count).toBeNull()
  })

  it('keys event caches by event semantics, range, coverage, and freshness', () => {
    const curve = row('trend-1')
    curve.series = {
      ...curve.series!,
      coverage: { first_date: '2026-10-22', last_date: '2026-11-12' },
      points: [{ count: 3, date: '2026-11-12' }],
    }
    const key = trendEventCacheKey(curve, 'month')!
    expect(key).toContain('events-v1')
    expect(key).toContain('month')
    expect(key).toContain('2026-11-01:2026-11-12:2026-10-01:2026-10-12:2025-11-01:2025-11-12')
    expect(key).toContain('2026-10-22')
    expect(key).toContain('fresh-1')
  })

  it('derives cache windows using the same natural-period rules as the service', () => {
    const curve = row('trend-1', 'job_family')
    curve.appliedCondition!.includeSecondary = true
    curve.condition.includeSecondary = true
    curve.series = {
      ...curve.series!,
      points: [{ count: 3, date: '2026-11-12' }],
    }
    expect(trendEventCacheKey(curve, 'week')).toContain(
      '2026-11-09:2026-11-12:2026-11-05:2026-11-08:2025-11-10:2025-11-13',
    )
    expect(trendEventCacheKey(curve, 'quarter')).toContain(
      '2026-10-01:2026-11-12:2026-08-19:2026-09-30:2025-10-01:2025-11-12',
    )
    expect(trendEventCacheKey(curve, 'year')).toContain(
      '2026-01-01:2026-11-12:2025-02-19:2025-12-31:2025-01-01:2025-11-12',
    )
    expect(trendEventCacheKey(curve, 'custom', { startDate: '2026-10-01', endDate: '2026-10-15' })).toContain(
      '2026-10-01:2026-10-15:2026-09-16:2026-09-30:2025-10-01:2025-10-15',
    )
    expect(trendEventCacheKey(curve, 'week')).toContain('|true|')
  })

  it('preserves case-sensitive formal values outside technology identities', () => {
    const upper = row('trend-1', 'project')
    const lower = row('trend-2', 'project')
    upper.condition.value = 'Project-A'
    upper.appliedCondition!.value = 'Project-A'
    lower.condition.value = 'project-a'
    lower.appliedCondition!.value = 'project-a'
    expect(trendEventCacheKey(upper, 'month')).not.toBe(trendEventCacheKey(lower, 'month'))
  })

  it('expires event caches after ten minutes and restores safe session results', async () => {
    const storage = new Map<string, string>()
    let timestamp = 1000
    const client = { readTrendEvents: vi.fn().mockResolvedValue(response) }
    const options = {
      now: () => timestamp,
      storage: {
        getItem: (key: string) => storage.get(key) ?? null,
        setItem: (key: string, value: string) => storage.set(key, value),
      },
    }
    const first = createTrendEventState(client, options)
    await first.load([row('trend-1')], 'month')
    const restored = createTrendEventState(client, options)
    await restored.load([row('trend-1')], 'month')
    expect(client.readTrendEvents).toHaveBeenCalledTimes(1)

    timestamp += 10 * 60 * 1000 + 1
    await restored.load([row('trend-1')], 'month')
    expect(client.readTrendEvents).toHaveBeenCalledTimes(2)
  })

  it('does not let an older range request overwrite a cached range', async () => {
    let resolvePending: ((value: TrendEventStatsResponse) => void) | undefined
    const readTrendEvents = vi
      .fn()
      .mockResolvedValueOnce(response)
      .mockImplementationOnce(
        () =>
          new Promise<TrendEventStatsResponse>((resolve) => {
            resolvePending = resolve
          }),
      )
    const state = createTrendEventState(
      { readTrendEvents },
      { storage: { getItem: () => null, setItem: () => undefined } },
    )
    const rows = [row('trend-1')]

    await state.load(rows, 'month')
    const pending = state.load(rows, 'week')
    const pendingSignal = readTrendEvents.mock.calls[1]![1] as AbortSignal
    await state.load(rows, 'month')
    expect(pendingSignal.aborted).toBe(true)
    resolvePending?.({ ...response, freshness: 'old-week' })
    await pending

    expect(state.response?.freshness).toBe('fresh-1')
    expect(state.status).toBe('ready')
  })

  it('stops an in-flight request when all applied curves disappear', async () => {
    const readTrendEvents = vi.fn().mockResolvedValue(response)
    const state = createTrendEventState({ readTrendEvents })
    const pendingRows = [row('trend-1')]
    await state.load(pendingRows, 'month')
    await state.load([], 'month')
    expect(state.status).toBe('idle')
    expect(state.isLoading).toBe(false)
    expect(state.response).toBeUndefined()
  })
})
