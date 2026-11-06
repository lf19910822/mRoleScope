import { beforeEach, describe, expect, it } from 'vitest'

import {
  createTrendDashboardState,
  type TrendRequestClient,
  type TrendSeries,
} from '../job-trend-dashboard-state'

const client: TrendRequestClient = {
  readTrend: async () => {
    throw new Error('the initial configuration test must not request a curve')
  },
}

beforeEach(() => sessionStorage.clear())

function series(label: string, freshness = 'version-a'): TrendSeries {
  return {
    curve: { dimension: 'technology', label, value: label.toLocaleLowerCase() },
    freshness,
    points: [{ count: 2, date: '2026-11-03' }],
  }
}

function deferred<T>() {
  let reject!: (reason?: unknown) => void
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, reject, resolve }
}

describe('trend dashboard state', () => {
  it('starts at the current week and accepts at most eight distinct normalized curves', () => {
    const dashboard = createTrendDashboardState(client)

    expect(dashboard.range).toBe('week')
    expect(dashboard.rows).toEqual([])

    for (const value of ['go', 'python', 'rust', 'java', 'csharp', 'cpp', 'lua', 'sql']) {
      expect(
        dashboard.addCurve({
          dimension: 'technology',
          includeSecondary: false,
          operator: '',
          value,
        }),
      ).toBeTruthy()
    }

    expect(dashboard.rows).toHaveLength(8)
    expect(
      dashboard.addCurve({
        dimension: 'technology',
        includeSecondary: false,
        operator: '',
        value: 'Go',
      }),
    ).toBeNull()
    expect(
      dashboard.addCurve({
        dimension: 'location',
        includeSecondary: false,
        operator: '',
        value: '上海',
      }),
    ).toBeNull()
    expect(dashboard.rows).toHaveLength(8)
  })

  it('clears controls that are incompatible with a newly selected dimension', () => {
    const dashboard = createTrendDashboardState(client)
    const curve = dashboard.addCurve({
      dimension: 'education',
      includeSecondary: false,
      operator: 'gte',
      value: 'bachelor',
    })
    if (curve === null) throw new Error('test curve was not added')

    expect(dashboard.changeDimension(curve.id, 'location')).toBe(true)
    expect(dashboard.rows[0]?.condition).toEqual({
      dimension: 'location',
      includeSecondary: false,
      operator: '',
      value: '',
    })
  })

  it('allows several empty draft cards before requiring distinct selected conditions', () => {
    const dashboard = createTrendDashboardState(client)

    expect(
      dashboard.addCurve({
        dimension: 'technology',
        includeSecondary: false,
        operator: '',
        value: '',
      }),
    ).toBeTruthy()
    expect(
      dashboard.addCurve({
        dimension: 'technology',
        includeSecondary: false,
        operator: '',
        value: '',
      }),
    ).toBeTruthy()
    expect(dashboard.rows).toHaveLength(2)

    expect(
      dashboard.updateCurve('trend-1', {
        dimension: 'technology',
        includeSecondary: false,
        operator: '',
        value: 'go',
      }),
    ).toBe(true)
    expect(
      dashboard.updateCurve('trend-2', {
        dimension: 'technology',
        includeSecondary: false,
        operator: '',
        value: 'go',
      }),
    ).toBe(false)
  })

  it('updates only the applied row and never lets a late result overwrite its latest condition', async () => {
    const go = deferred<TrendSeries>()
    const python = deferred<TrendSeries>()
    const dashboard = createTrendDashboardState({
      readTrend: async ({ condition }) => {
        if (condition.value === 'go') return go.promise
        if (condition.value === 'python') return python.promise
        throw new Error('unexpected curve')
      },
    })
    const curve = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    const untouched = dashboard.addCurve({
      dimension: 'location',
      includeSecondary: false,
      operator: '',
      value: '上海',
    })
    if (curve === null || untouched === null) throw new Error('test curves were not added')

    const firstRequest = dashboard.applyCurve(curve.id)
    expect(dashboard.rows[0]?.status).toBe('loading')
    expect(dashboard.rows[1]?.status).toBe('idle')

    expect(
      dashboard.updateCurve(curve.id, {
        dimension: 'technology',
        includeSecondary: false,
        operator: '',
        value: 'python',
      }),
    ).toBe(true)
    const latestRequest = dashboard.applyCurve(curve.id)
    python.resolve(series('Python'))
    await latestRequest
    go.resolve(series('Go'))
    await firstRequest

    expect(dashboard.rows[0]?.series?.curve.label).toBe('Python')
    expect(dashboard.rows[0]?.status).toBe('ready')
    expect(dashboard.rows[1]?.series).toBeUndefined()
  })

  it('keeps a row’s previous successful line when its later update fails', async () => {
    let fail = false
    const dashboard = createTrendDashboardState({
      readTrend: async ({ condition }) => {
        if (fail) throw new Error('unavailable')
        return series(condition.value === 'go' ? 'Go' : 'Python')
      },
    })
    const curve = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    if (curve === null) throw new Error('test curve was not added')

    await dashboard.applyCurve(curve.id)
    fail = true
    dashboard.updateCurve(curve.id, {
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'python',
    })
    await dashboard.applyCurve(curve.id)

    expect(dashboard.rows[0]?.series?.curve.label).toBe('Go')
    expect(dashboard.rows[0]?.status).toBe('error')
  })

  it('immediately refreshes each existing curve when the shared time range changes', async () => {
    const requests: string[] = []
    const dashboard = createTrendDashboardState({
      readTrend: async ({ condition, range }) => {
        requests.push(`${range}:${condition.value}`)
        return series(condition.value)
      },
    })
    const go = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    const shanghai = dashboard.addCurve({
      dimension: 'location',
      includeSecondary: false,
      operator: '',
      value: '上海',
    })
    if (go === null || shanghai === null) throw new Error('test curves were not added')
    await dashboard.applyCurve(go.id)
    await dashboard.applyCurve(shanghai.id)
    requests.length = 0

    await dashboard.setRange('all')

    expect(dashboard.range).toBe('all')
    expect(requests).toEqual(['all:go', 'all:上海'])
    expect(dashboard.rows.map((row) => row.status)).toEqual(['ready', 'ready'])
  })

  it('requests an inclusive custom date range and preserves it as the active range', async () => {
    const requests: Array<{ endDate?: string; range: string; startDate?: string }> = []
    const dashboard = createTrendDashboardState({
      readTrend: async (request) => {
        requests.push({
          endDate: request.endDate,
          range: request.range,
          startDate: request.startDate,
        })
        return series(request.condition.value)
      },
    })
    const curve = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    if (curve === null) throw new Error('test curve was not added')

    await dashboard.applyCurve(curve.id)
    requests.length = 0
    await dashboard.setCustomRange('2026-10-01', '2026-10-15')

    expect(dashboard.range).toBe('custom')
    expect(dashboard.customRange).toEqual({ endDate: '2026-10-15', startDate: '2026-10-01' })
    expect(requests).toEqual([
      { endDate: '2026-10-15', range: 'custom', startDate: '2026-10-01' },
    ])
  })

  it('reuses a fresh cached curve when returning to a previously viewed range', async () => {
    const requests: string[] = []
    const dashboard = createTrendDashboardState({
      readTrend: async ({ condition, range }) => {
        requests.push(`${range}:${condition.value}`)
        return series(`${condition.value}-${range}`)
      },
    })
    const curve = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    if (curve === null) throw new Error('test curve was not added')

    await dashboard.applyCurve(curve.id)
    await dashboard.setRange('month')
    await dashboard.setRange('week')

    expect(requests).toEqual(['week:go', 'month:go'])
    expect(dashboard.rows[0]?.series?.curve.label).toBe('go-week')
  })

  it('never automatically requests an incomplete restored configuration', async () => {
    let requests = 0
    const dashboard = createTrendDashboardState({
      readTrend: async () => {
        requests += 1
        return series('unexpected')
      },
    })
    dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: '',
    })

    await dashboard.activate()
    await dashboard.setRange('month')

    expect(requests).toBe(0)
    expect(dashboard.rows[0]?.status).toBe('idle')
  })

  it('keeps an edited condition out of an immediate range refresh until it is applied', async () => {
    const requests: string[] = []
    const dashboard = createTrendDashboardState({
      readTrend: async ({ condition, range }) => {
        requests.push(`${range}:${condition.value}`)
        return series(`${condition.value}-${range}`)
      },
    })
    const curve = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    if (curve === null) throw new Error('test curve was not added')

    await dashboard.applyCurve(curve.id)
    expect(
      dashboard.updateCurve(curve.id, {
        dimension: 'technology',
        includeSecondary: false,
        operator: '',
        value: 'python',
      }),
    ).toBe(true)
    await dashboard.setRange('month')

    expect(requests).toEqual(['week:go', 'month:go'])
    expect(dashboard.rows[0]?.series?.curve.label).toBe('go-month')
  })

  it('refreshes only an applied condition rather than an un-applied draft', async () => {
    const requests: string[] = []
    const dashboard = createTrendDashboardState({
      readTrend: async ({ condition, range }) => {
        requests.push(`${range}:${condition.value}`)
        return series(`${condition.value}-${range}`)
      },
    })
    const curve = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    if (curve === null) throw new Error('test curve was not added')

    await dashboard.applyCurve(curve.id)
    dashboard.updateCurve(curve.id, {
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'python',
    })
    await dashboard.refresh()

    expect(requests).toEqual(['week:go', 'week:go'])
    expect(dashboard.rows[0]?.series?.curve.label).toBe('go-week')
  })

  it('reuses a fresh in-memory curve on ordinary return, while a new page load restores it then revalidates', async () => {
    const values = new Map<string, string>()
    const storage = {
      getItem: (key: string) => values.get(key) ?? null,
      removeItem: (key: string) => values.delete(key),
      setItem: (key: string, value: string) => values.set(key, value),
    }
    let now = 0
    let requests = 0
    const makeClient = (): TrendRequestClient => ({
      readTrend: async () => {
        requests += 1
        return series(requests === 1 ? 'Go' : 'Go (updated)', `version-${requests}`)
      },
    })
    const dashboard = createTrendDashboardState(makeClient(), { now: () => now, storage })
    await dashboard.activate()
    const curve = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    if (curve === null) throw new Error('test curve was not added')
    await dashboard.applyCurve(curve.id)

    now = 5 * 60 * 1000
    await dashboard.activate()
    expect(requests).toBe(1)

    const reloaded = createTrendDashboardState(makeClient(), { now: () => now, storage })
    expect(reloaded.rows[0]?.series?.curve.label).toBe('Go')
    await reloaded.activate()
    expect(requests).toBe(2)
    expect(reloaded.rows[0]?.series?.curve.label).toBe('Go (updated)')
  })

  it('keeps an expired line visible and refreshes it when the dashboard is revisited', async () => {
    const values = new Map<string, string>()
    const storage = {
      getItem: (key: string) => values.get(key) ?? null,
      removeItem: (key: string) => values.delete(key),
      setItem: (key: string, value: string) => values.set(key, value),
    }
    let now = 0
    let requests = 0
    const dashboard = createTrendDashboardState(
      {
        readTrend: async () => {
          requests += 1
          return series(requests === 1 ? 'Go' : 'Go (refreshed)')
        },
      },
      { now: () => now, storage },
    )
    await dashboard.activate()
    const curve = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    if (curve === null) throw new Error('test curve was not added')
    await dashboard.applyCurve(curve.id)

    now = 11 * 60 * 1000
    await dashboard.activate()

    expect(dashboard.rows[0]?.series?.curve.label).toBe('Go (refreshed)')
    expect(requests).toBe(2)
  })

  it('persists an explicit legend visibility choice without changing the curve data', async () => {
    const dashboard = createTrendDashboardState({ readTrend: async () => series('Go') })
    const curve = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    if (curve === null) throw new Error('test curve was not added')
    await dashboard.applyCurve(curve.id)

    expect(dashboard.setCurveVisibility(curve.id, false)).toBe(true)
    expect(dashboard.rows[0]?.series?.curve.label).toBe('Go')

    const restored = createTrendDashboardState({ readTrend: async () => series('Go') })
    expect(restored.rows[0]?.visible).toBe(false)
  })

  it('removes a configured curve and frees a slot for another condition', () => {
    const dashboard = createTrendDashboardState(client)
    const curve = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    if (curve === null) throw new Error('test curve was not added')

    expect(dashboard.removeCurve(curve.id)).toBe(true)
    expect(dashboard.rows).toEqual([])
    expect(
      dashboard.addCurve({
        dimension: 'technology',
        includeSecondary: false,
        operator: '',
        value: 'go',
      }),
    ).toBeTruthy()
  })

  it('replaces formal curves together after an opaque freshness change instead of mixing releases', async () => {
    const firstRust = deferred<TrendSeries>()
    const groupRust = deferred<TrendSeries>()
    const groupPython = deferred<TrendSeries>()
    let updating = false
    let rustRequests = 0
    const dashboard = createTrendDashboardState({
      readTrend: async ({ condition }) => {
        if (!updating) return series(condition.value === 'go' ? 'Go' : 'Python', 'version-a')
        if (condition.value === 'rust') {
          rustRequests += 1
          return rustRequests === 1 ? firstRust.promise : groupRust.promise
        }
        return groupPython.promise
      },
    })
    const go = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    const python = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'python',
    })
    if (go === null || python === null) throw new Error('test curves were not added')
    await dashboard.applyCurve(go.id)
    await dashboard.applyCurve(python.id)

    updating = true
    dashboard.updateCurve(go.id, { ...go.condition, value: 'rust' })
    const update = dashboard.applyCurve(go.id)
    firstRust.resolve(series('Rust', 'version-b'))
    await Promise.resolve()

    expect(dashboard.rows.map((row) => row.series?.curve.label)).toEqual(['Go', 'Python'])

    groupRust.resolve(series('Rust', 'version-b'))
    groupPython.resolve(series('Python', 'version-b'))
    await update

    expect(dashboard.rows.map((row) => row.series?.curve.label)).toEqual(['Rust', 'Python'])
    expect(dashboard.rows.map((row) => row.series?.freshness)).toEqual(['version-b', 'version-b'])
  })
})
