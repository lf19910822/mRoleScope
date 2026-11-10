import { afterEach, describe, expect, it } from 'vitest'

import {
  clearHistoryCache,
  historyNodeAccessibleLabel,
  historyNodeChartColors,
  historyNodeChartLabel,
  historyNodeChartSymbols,
  historyNodeTagLabel,
  historyTimelineMinWidth,
  readHistoryCache,
  writeHistoryCache,
} from '../job-history-timeline'
import type { HistoryNode } from '@/api/jd-api'

afterEach(() => clearHistoryCache())

describe('job history timeline state', () => {
  it('keeps display labels compact and accessible without exposing provenance', () => {
    const node: HistoryNode = {
      can_view_historical: true,
      connect_previous: false,
      date: '2026-11-06',
      summary: '岗位职责',
      tags: ['reopened', 'body_responsibilities'],
      type: 'reopened' as const,
    }

    expect(historyNodeTagLabel('body_responsibilities')).toBe('岗位职责')
    expect(historyNodeAccessibleLabel(node)).toBe('2026-11-06 · 重新上线 · 重新上线、岗位职责')
    expect(historyNodeAccessibleLabel(node)).not.toMatch(/run|parser|version|uuid/i)
  })

  it('caches history for ten minutes and sizes long timelines for horizontal pan', () => {
    const response = { nodes: [] }
    writeHistoryCache('jd-1', response, 100)
    expect(readHistoryCache('jd-1', 100)).toBe(response)
    expect(readHistoryCache('jd-1', 100 + 10 * 60 * 1000 - 1)).toBe(response)
    expect(readHistoryCache('jd-1', 100 + 10 * 60 * 1000)).toBeUndefined()
    expect(historyTimelineMinWidth(8, 1)).toBe('1184px')
  })

  it('keeps lifecycle statuses visually distinct in the history chart', () => {
    expect(historyNodeChartLabel('modified')).toBe('修改')
    expect(historyNodeChartSymbols.confirmed_closed).toBe('rect')
    expect(historyNodeChartColors.latest).toBe('#22C55E')
    expect(new Set(Object.values(historyNodeChartColors)).size).toBeGreaterThan(1)
  })
})
