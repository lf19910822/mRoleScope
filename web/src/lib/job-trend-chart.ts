import type { TrendCurveRow, TrendRange } from './job-trend-dashboard-state'

export interface TrendChartSeries {
  connectNulls: false
  data: Array<[string, number | null]>
  emphasis: {
    focus: 'series'
    itemStyle: { borderColor: string; borderWidth: number }
    scale: number
  }
  itemStyle: { color: string }
  lineStyle: { color: string; width: number }
  name: string
  showSymbol: boolean
  smooth: number
  symbol: 'circle'
  symbolSize: number
  type: 'line'
}

export interface TrendCurveObservationSummary {
  count: number
  date: string
  delta?: number
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
  legend: {
    bottom: number
    data: string[]
    formatter?: (name: string) => string
    left: number
    right: number
    selected: Record<string, boolean>
    type: 'scroll'
  }
  series: TrendChartSeries[]
  tooltip: { axisPointer: { snap: boolean; type: 'line' }; trigger: 'axis' }
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
  '#0072B2',
  '#E69F00',
  '#009E73',
  '#D55E00',
  '#CC79A7',
  '#56B4E9',
  '#B79F00',
  '#332288',
]

function representedCondition(row: TrendCurveRow) {
  return row.series !== undefined ? (row.appliedCondition ?? row.condition) : row.condition
}

function representedLabel(row: TrendCurveRow): string {
  return row.series?.curve.label ?? representedCondition(row).value
}

export function trendCurveColor(row: TrendCurveRow): string {
  const match = /^trend-(\d+)$/u.exec(row.id)
  if (match?.[1] !== undefined) {
    const ordinal = Number(match[1])
    if (Number.isSafeInteger(ordinal) && ordinal > 0)
      return trendPalette[(ordinal - 1) % trendPalette.length] ?? trendPalette[0]!
  }

  const hash = Array.from(row.id).reduce((total, character) => total + character.codePointAt(0)!, 0)
  return trendPalette[hash % trendPalette.length] ?? trendPalette[0]!
}

export function trendCurveLabel(row: TrendCurveRow): string {
  const condition = representedCondition(row)
  const label = representedLabel(row)
  switch (condition.dimension) {
    case 'education':
      return `学历 ${trendOperatorLabel(condition.operator)} ${label}`
    case 'experience':
      return `最低工作年限 ${trendOperatorLabel(condition.operator)} ${label} 年`
    case 'job_family':
      return `岗位族 · ${label}${condition.includeSecondary ? '（含副岗位族）' : '（仅主岗位族）'}`
    case 'location':
      return `地点 · ${label}`
    case 'project':
      return `项目 · ${label}`
    case 'technology':
      return `技术关键词 · ${label}`
  }
}

export function trendCurveLegendLabel(row: TrendCurveRow): string {
  const condition = representedCondition(row)
  const label = representedLabel(row)
  switch (condition.dimension) {
    case 'education':
      return `学历 ${trendOperatorLabel(condition.operator)} ${label}`
    case 'experience':
      return `年限 ${trendOperatorLabel(condition.operator)} ${label}年`
    case 'job_family':
      return `岗位 ${label}${condition.includeSecondary ? ' +副' : ''}`
    case 'location':
      return `地点 ${label}`
    case 'project':
      return `项目 ${label}`
    case 'technology':
      return `技术 ${label}`
  }
}

export function trendCurveObservationSummary(
  row: TrendCurveRow,
): TrendCurveObservationSummary | undefined {
  const points = row.series?.points ?? []
  let latestIndex = -1
  for (let index = points.length - 1; index >= 0; index -= 1) {
    if (points[index]?.count !== null) {
      latestIndex = index
      break
    }
  }
  if (latestIndex < 0) return undefined

  const latest = points[latestIndex]!
  let priorIndex = -1
  for (let index = latestIndex - 1; index >= 0; index -= 1) {
    if (points[index]?.count !== null) {
      priorIndex = index
      break
    }
  }
  const prior = priorIndex < 0 ? undefined : points[priorIndex]

  return {
    count: latest.count!,
    date: latest.date,
    ...(prior === undefined ? {} : { delta: latest.count! - prior.count! }),
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
  const series = renderedRows.map((row) => {
    const color = trendCurveColor(row)
    return {
      connectNulls: false as const,
      data: row.series.points.map((point) => [point.date, point.count] as [string, number | null]),
      emphasis: {
        focus: 'series' as const,
        itemStyle: { borderColor: '#ffffff', borderWidth: 2 },
        scale: 1.5,
      },
      itemStyle: { color },
      lineStyle: { color, width: 2.5 },
      name: trendCurveLabel(row),
      showSymbol: true,
      smooth: 0.24,
      symbol: 'circle' as const,
      symbolSize: 8,
      type: 'line' as const,
    }
  })
  const compactLegendLabels = new Map(
    renderedRows.map((row) => [trendCurveLabel(row), trendCurveLegendLabel(row)]),
  )
  return {
    animationDuration: 300,
    animationDurationUpdate: 260,
    dataZoom: range === 'all' || range === 'custom' ? allHistoryDataZoom() : [],
    grid: { bottom: 78, left: 46, right: 24, top: 46 },
    legend: {
      bottom: 8,
      data: series.map((line) => line.name),
      formatter: (name) => compactLegendLabels.get(name) ?? name,
      left: 46,
      right: 24,
      selected: Object.fromEntries(
        renderedRows.map((row) => [trendCurveLabel(row), row.visible !== false]),
      ),
      type: 'scroll',
    },
    series,
    tooltip: { axisPointer: { snap: true, type: 'line' }, trigger: 'axis' },
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
