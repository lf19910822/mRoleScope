import { describe, expect, it, vi } from 'vitest'

import { createRequestActivityCoordinator } from '@/lib/request-activity'

describe('request activity coordinator', () => {
  it('keeps one visible loading state until all overlapping requests settle', () => {
    vi.useFakeTimers()
    const activity = createRequestActivityCoordinator(160)
    const settleFirst = activity.begin()
    vi.advanceTimersByTime(80)
    const settleSecond = activity.begin()
    vi.advanceTimersByTime(80)

    expect(activity.activeCount.value).toBe(2)
    expect(activity.visible.value).toBe(true)

    settleFirst()
    expect(activity.activeCount.value).toBe(1)
    expect(activity.visible.value).toBe(true)

    settleSecond()
    expect(activity.activeCount.value).toBe(0)
    expect(activity.visible.value).toBe(false)
    vi.useRealTimers()
  })

  it('does not flash a global loader for requests that settle before the delay', () => {
    vi.useFakeTimers()
    const activity = createRequestActivityCoordinator(160)
    activity.begin()()
    vi.advanceTimersByTime(200)

    expect(activity.activeCount.value).toBe(0)
    expect(activity.visible.value).toBe(false)
    vi.useRealTimers()
  })

  it('settles failed work once without hiding a different outstanding request', () => {
    vi.useFakeTimers()
    const activity = createRequestActivityCoordinator(0)
    const settleFirst = activity.begin()
    const settleSecond = activity.begin()
    vi.runAllTimers()
    expect(activity.visible.value).toBe(true)

    settleFirst()
    settleFirst()
    expect(activity.activeCount.value).toBe(1)
    expect(activity.visible.value).toBe(true)

    settleSecond()
    expect(activity.activeCount.value).toBe(0)
    expect(activity.visible.value).toBe(false)
    vi.useRealTimers()
  })
})
