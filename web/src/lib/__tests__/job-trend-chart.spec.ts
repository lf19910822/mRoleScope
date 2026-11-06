import { describe, expect, it } from 'vitest'

import {
  createTrendChartOption,
  trendCurveColor,
  trendCurveLegendLabel,
  trendCurveObservationSummary,
  trendPalette,
} from '../job-trend-chart'
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

  it('enables bounded in-chart zoom and pan for all-history and custom ranges', () => {
    const option = createTrendChartOption([row()], 'all')

    expect(option.dataZoom).toHaveLength(2)
    expect(option.dataZoom[0]).toMatchObject({ type: 'inside', xAxisIndex: 0 })
    expect(option.dataZoom[1]).toMatchObject({ type: 'inside', yAxisIndex: 0 })
    expect(option.yAxis.min).toBe(0)

    expect(createTrendChartOption([row()], 'custom').dataZoom).toHaveLength(2)
  })

  it('restores a curve’s legend visibility without removing its data', () => {
    const hidden = row()
    hidden.visible = false

    const option = createTrendChartOption([hidden], 'week')

    expect(option.series).toHaveLength(1)
    expect(option.legend.selected).toEqual({ '技术关键词 · Go': false })
  })

  it('keeps an applied curve’s observation colour and compact legend label when a sibling is a draft', () => {
    const draft = row()
    draft.id = 'trend-1'
    draft.series = undefined
    const applied = row()
    applied.id = 'trend-2'

    const option = createTrendChartOption([draft, applied], 'week')

    expect(trendCurveColor(applied)).toBe(trendPalette[1])
    expect(option.series[0]?.itemStyle.color).toBe(trendPalette[1])
    expect(trendCurveLegendLabel(applied)).toBe('技术 Go')
    expect(option.legend.formatter?.('技术关键词 · Go')).toBe('技术 Go')
  })

  it('provides eight distinct high-contrast colors for the eight curve slots', () => {
    expect(trendPalette).toHaveLength(8)
    expect(new Set(trendPalette).size).toBe(8)
    expect(trendPalette).toEqual([
      '#0072B2',
      '#E69F00',
      '#009E73',
      '#D55E00',
      '#CC79A7',
      '#56B4E9',
      '#B79F00',
      '#332288',
    ])
  })

  it('summarizes the latest numeric observation across a gap without treating zero as missing', () => {
    const observed = row()
    observed.series!.points = [
      { count: 3, date: '2026-11-03' },
      { count: null, date: '2026-11-04' },
      { count: 0, date: '2026-11-05' },
    ]
    const unavailable = row()
    unavailable.series!.points = [
      { count: null, date: '2026-11-03' },
      { count: null, date: '2026-11-04' },
    ]

    expect(trendCurveObservationSummary(observed)).toEqual({
      count: 0,
      date: '2026-11-05',
      delta: -3,
    })
    expect(trendCurveObservationSummary(unavailable)).toBeUndefined()
  })

  it('reserves a separate legend band when eight curves share the chart', () => {
    const option = createTrendChartOption(
      Array.from({ length: 8 }, (_, index) => {
        const curve = row()
        curve.id = `trend-${index + 1}`
        curve.condition.value = `technology-${index + 1}`
        curve.series = {
          ...curve.series!,
          curve: {
            ...curve.series!.curve,
            label: `Technology ${index + 1}`,
            value: `technology-${index + 1}`,
          },
        }
        return curve
      }),
      'all',
    )

    expect(option.grid.bottom).toBeGreaterThanOrEqual(72)
    expect(option.legend).toMatchObject({ bottom: 8, left: 46, right: 24, type: 'scroll' })
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
