import { describe, expect, it } from 'vitest'

import { createTrendChartOption } from '../job-trend-chart'
import type { TrendCurveRow } from '../job-trend-dashboard-state'

function row(): TrendCurveRow {
  return {
    condition: {
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    },
    error: '',
    id: 'trend-1',
    series: {
      curve: { dimension: 'technology', label: 'Go', value: 'go' },
      freshness: 'safe-marker',
      points: [
        { count: 0, date: '2026-11-03' },
        { count: null, date: '2026-11-04' },
        { count: 3, date: '2026-11-05' },
      ],
    },
    status: 'ready',
  }
}

describe('job trend chart option', () => {
  it('preserves zeroes and missing-history gaps while keeping the Y baseline at zero', () => {
    const option = createTrendChartOption([row()], 'week')
    const series = option.series[0]

    expect(series?.data).toEqual([
      ['2026-11-03', 0],
      ['2026-11-04', null],
      ['2026-11-05', 3],
    ])
    expect(series?.connectNulls).toBe(false)
    expect(option.yAxis.min).toBe(0)
    expect(option.dataZoom).toEqual([])
  })

  it('enables bounded in-chart zoom and pan only for the all-history range', () => {
    const option = createTrendChartOption([row()], 'all')

    expect(option.dataZoom).toHaveLength(2)
    expect(option.dataZoom[0]).toMatchObject({ type: 'inside', xAxisIndex: 0 })
    expect(option.dataZoom[1]).toMatchObject({ type: 'inside', yAxisIndex: 0 })
    expect(option.yAxis.min).toBe(0)
  })

  it('restores a curve’s legend visibility without removing its data', () => {
    const hidden = row()
    hidden.visible = false

    const option = createTrendChartOption([hidden], 'week')

    expect(option.series).toHaveLength(1)
    expect(option.legend.selected).toEqual({ '技术关键词 · Go': false })
  })

  it('keeps a rendered line labelled with its last applied condition while its next condition is still a draft', () => {
    const pending = row()
    pending.appliedCondition = {
      dimension: 'education',
      includeSecondary: false,
      operator: 'gte',
      value: 'bachelor',
    }
    pending.condition = {
      dimension: 'location',
      includeSecondary: false,
      operator: '',
      value: '上海',
    }
    pending.series = {
      curve: {
        dimension: 'education',
        label: '本科',
        operator: 'gte',
        value: 'bachelor',
      },
      freshness: 'safe-marker',
      points: [{ count: 2, date: '2026-11-03' }],
    }

    const option = createTrendChartOption([pending], 'week')

    expect(option.legend.data).toEqual(['学历 ≥ 本科'])
  })
})
