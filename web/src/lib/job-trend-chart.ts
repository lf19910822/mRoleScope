import type { TrendCurveRow, TrendRange } from './job-trend-dashboard-state'

export interface TrendChartSeries {
  connectNulls: false
  data: Array<[string, number | null]>
  emphasis: { focus: 'series' }
  itemStyle: { color: string }
  lineStyle: { color: string; width: number }
  name: string
  showSymbol: false
  smooth: number
  symbol: 'circle'
  type: 'line'
}

export interface TrendChartOption {
  animationDuration: number
  animationDurationUpdate: number
  dataZoom: Array<{
    filterMode: 'none'
    moveOnMouseMove: boolean | 'shift'
    moveOnMouseWheel: boolean | 'shift'
    type: 'inside'
    xAxisIndex?: number
    yAxisIndex?: number
    zoomOnMouseWheel: boolean | 'shift'
  }>
  grid: { bottom: number; left: number; right: number; top: number }
  legend: { data: string[]; selected: Record<string, boolean>; type: 'scroll' }
  series: TrendChartSeries[]
  tooltip: { axisPointer: { type: 'line' }; trigger: 'axis' }
  xAxis: { axisLabel: { hideOverlap: true }; boundaryGap: false; type: 'time' }
  yAxis: {
    axisLabel: { formatter: '{value}' }
    min: 0
    minInterval: 1
    name: '岗位数'
    type: 'value'
  }
}

const trendPalette = [
  '#007cab',
  '#0b9a8c',
  '#9360c9',
  '#dc7d32',
  '#3c83d1',
  '#c6546c',
  '#648c38',
  '#b060a8',
]

export function trendCurveLabel(row: TrendCurveRow): string {
  const representedCondition =
    row.series !== undefined ? (row.appliedCondition ?? row.condition) : row.condition
  const label = row.series?.curve.label ?? representedCondition.value
  switch (representedCondition.dimension) {
    case 'education':
      return `学历 ${trendOperatorLabel(representedCondition.operator)} ${label}`
    case 'experience':
      return `最低工作年限 ${trendOperatorLabel(representedCondition.operator)} ${label} 年`
    case 'job_family':
      return `岗位族 · ${label}${representedCondition.includeSecondary ? '（含副岗位族）' : '（仅主岗位族）'}`
    case 'location':
      return `地点 · ${label}`
    case 'project':
      return `项目 · ${label}`
    case 'technology':
      return `技术关键词 · ${label}`
  }
}

function trendOperatorLabel(operator: TrendCurveRow['condition']['operator']): string {
  switch (operator) {
    case 'eq':
      return '='
    case 'gt':
      return '>'
    case 'gte':
      return '≥'
    case 'lt':
      return '<'
    case 'lte':
      return '≤'
    default:
      return ''
  }
}

function allHistoryDataZoom(): TrendChartOption['dataZoom'] {
  return [
    {
      filterMode: 'none',
      moveOnMouseMove: true,
      moveOnMouseWheel: false,
      type: 'inside',
      xAxisIndex: 0,
      zoomOnMouseWheel: true,
    },
    {
      filterMode: 'none',
      moveOnMouseMove: 'shift',
      moveOnMouseWheel: false,
      type: 'inside',
      yAxisIndex: 0,
      zoomOnMouseWheel: 'shift',
    },
  ]
}

export function createTrendChartOption(rows: TrendCurveRow[], range: TrendRange): TrendChartOption {
  const renderedRows = rows.filter(
    (row): row is TrendCurveRow & { series: NonNullable<TrendCurveRow['series']> } =>
      row.series !== undefined,
  )
  const series = renderedRows.map((row, index) => {
    const color = trendPalette[index % trendPalette.length] ?? trendPalette[0] ?? '#007cab'
    return {
      connectNulls: false as const,
      data: row.series.points.map((point) => [point.date, point.count] as [string, number | null]),
      emphasis: { focus: 'series' as const },
      itemStyle: { color },
      lineStyle: { color, width: 2.5 },
      name: trendCurveLabel(row),
      showSymbol: false as const,
      smooth: 0.24,
      symbol: 'circle' as const,
      type: 'line' as const,
    }
  })
  return {
    animationDuration: 300,
    animationDurationUpdate: 260,
    dataZoom: range === 'all' ? allHistoryDataZoom() : [],
    grid: { bottom: 36, left: 46, right: 24, top: 46 },
    legend: {
      data: series.map((line) => line.name),
      selected: Object.fromEntries(
        renderedRows.map((row) => [trendCurveLabel(row), row.visible !== false]),
      ),
      type: 'scroll',
    },
    series,
    tooltip: { axisPointer: { type: 'line' }, trigger: 'axis' },
    xAxis: { axisLabel: { hideOverlap: true }, boundaryGap: false, type: 'time' },
    yAxis: {
      axisLabel: { formatter: '{value}' },
      min: 0,
      minInterval: 1,
      name: '岗位数',
      type: 'value',
    },
  }
}

export { trendPalette }
