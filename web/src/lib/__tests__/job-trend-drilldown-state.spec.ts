import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  createTrendDrilldownState,
  type TrendDrilldownClient,
} from '../job-trend-drilldown-state'

const condition = {
  dimension: 'technology' as const,
  includeSecondary: false,
  operator: '' as const,
  value: 'go',
}

function response(total = 1) {
  return {
    items: total === 0 ? [] : [{ can_view_historical: false, can_view_latest: true, id: 'jd_opaque', status: 'current' as const, title: 'Go 工程师' }],
    matched_total: total,
    node_date: '2026-10-15',
    page: { number: 1, page_size: 20, total },
  }
}

describe('trend drilldown state', () => {
  beforeEach(() => vi.useRealTimers())

  it('ignores an unavailable node, opens a trusted zero, and toggles the same node closed', async () => {
    const readNode = vi.fn<TrendDrilldownClient['readTrendDrilldown']>().mockResolvedValue(response(0))
    const state = createTrendDrilldownState({ readTrendDrilldown: readNode })

    await state.open(condition, '2026-10-15', null)
    expect(readNode).not.toHaveBeenCalled()
    expect(state.isOpen).toBe(false)

    await state.open(condition, '2026-10-15', 0)
    expect(readNode).toHaveBeenCalledWith(
      expect.objectContaining({ count: 0, date: '2026-10-15', page: 1, pageSize: 20 }),
      expect.any(AbortSignal),
    )
    expect(state.isOpen).toBe(true)
    expect(state.response?.matched_total).toBe(0)

    await state.open(condition, '2026-10-15', 0)
    expect(state.isOpen).toBe(false)
  })

  it('reuses the container for another node and debounces a title search onto page one', async () => {
    vi.useFakeTimers()
    const readNode = vi.fn<TrendDrilldownClient['readTrendDrilldown']>().mockResolvedValue(response())
    const state = createTrendDrilldownState({ readTrendDrilldown: readNode })

    await state.open(condition, '2026-10-15', 1)
    await state.open(condition, '2026-10-16', 1)
    expect(state.node?.date).toBe('2026-10-16')
    expect(state.isOpen).toBe(true)

    state.setTitle('平台')
    expect(readNode).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(300)
    expect(readNode).toHaveBeenLastCalledWith(
      expect.objectContaining({ date: '2026-10-16', page: 1, title: '平台' }),
      expect.any(AbortSignal),
    )
  })

  it('clears an earlier result before a failed search so stale jobs cannot be acted on', async () => {
    vi.useFakeTimers()
    const readNode = vi
      .fn<TrendDrilldownClient['readTrendDrilldown']>()
      .mockResolvedValueOnce(response(3))
      .mockRejectedValueOnce(new Error('temporary failure'))
    const state = createTrendDrilldownState({ readTrendDrilldown: readNode })

    await state.open(condition, '2026-10-15', 3)
    expect(state.response?.matched_total).toBe(3)

    state.setTitle('不存在的岗位')
    await vi.advanceTimersByTimeAsync(300)

    expect(state.response).toBeUndefined()
    expect(state.error).toBe('无法读取该日岗位，请重试。')
  })

  it('ignores a stale first-node response after another node takes ownership', async () => {
    let resolveFirst: ((value: ReturnType<typeof response>) => void) | undefined
    let resolveSecond: ((value: ReturnType<typeof response>) => void) | undefined
    const readNode = vi
      .fn<TrendDrilldownClient['readTrendDrilldown']>()
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveFirst = resolve
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveSecond = resolve
          }),
      )
    const state = createTrendDrilldownState({ readTrendDrilldown: readNode })

    const firstOpen = state.open(condition, '2026-10-15', 1)
    const secondOpen = state.open(condition, '2026-10-16', 1)
    resolveFirst?.({ ...response(1), node_date: '2026-10-15' })
    await firstOpen

    expect(state.node?.date).toBe('2026-10-16')
    expect(state.response).toBeUndefined()

    resolveSecond?.({ ...response(1), node_date: '2026-10-16' })
    await secondOpen
    expect(state.response?.node_date).toBe('2026-10-16')
  })
})
