<script setup lang="ts">
import { use } from 'echarts/core'
import type { EChartsOption } from 'echarts'
import 'echarts/theme/dark'
import { LineChart } from 'echarts/charts'
import {
  DataZoomComponent,
  GridComponent,
  LegendComponent,
  TooltipComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import {
  NButton,
  NEmpty,
  NPagination,
  NPopover,
  NSelect,
  NSwitch,
  type SelectOption,
  useOsTheme,
} from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import VChart, { type Exposed as VChartExposed } from 'vue-echarts'

import {
  getFilterOptions,
  getJobDetail,
  getJobHistoryDetail,
  getJobTrendDrilldown,
  getJobTrendEvents,
  type FilterOptionsResponse,
  type JobDetailResponse,
  type TrendDrilldownItem,
  type TrendDrilldownStatus,
} from '@/api/jd-api'
import FilterDatePicker from '@/components/FilterDatePicker.vue'
import JDDetailReadingLayout from '@/components/JDDetailReadingLayout.vue'
import JobDetailSkeleton from '@/components/JobDetailSkeleton.vue'
import TechnologyKeywordSelect from '@/components/TechnologyKeywordSelect.vue'
import { createTrendChartOption, trendCurveColor, trendCurveLabel } from '@/lib/job-trend-chart'
import { useJobTrendDashboard } from '@/lib/job-trend-dashboard'
import { createTrendDrilldownState } from '@/lib/job-trend-drilldown-state'
import {
  createTrendEventState,
  formatTrendEventComparison,
  formatTrendEventRate,
  supportsTrendEvents,
  trendEventTypeLabel,
} from '@/lib/job-trend-event-state'
import type {
  TrendComparison,
  TrendCurveRow,
  TrendDimension,
  TrendEventType,
  TrendRange,
} from '@/lib/job-trend-dashboard-state'

use([
  CanvasRenderer,
  LineChart,
  DataZoomComponent,
  GridComponent,
  LegendComponent,
  TooltipComponent,
])

const dashboard = useJobTrendDashboard()
const drilldown = createTrendDrilldownState({ readTrendDrilldown: getJobTrendDrilldown })
const eventStats = createTrendEventState({ readTrendEvents: getJobTrendEvents })
const eventPopoverThemeOverrides = { boxShadow: 'none' }
const trendChart = ref<VChartExposed | null>(null)
const mobileConfigurationOpen = ref(false)
const configurationExpanded = ref(defaultConfigurationExpanded())
const configurationManuallyToggled = ref(false)
const customRangeOpen = ref(false)
const customRangeError = ref('')
const customStartDate = ref(dashboard.customRange?.startDate ?? '')
const customEndDate = ref(dashboard.customRange?.endDate ?? '')
const filterOptionsError = ref('')
const curveFeedback = ref('')
const highlightIncompleteCurves = ref(false)
const focusedCurveId = ref('')
const filterOptions = ref<FilterOptionsResponse>({
  activity_states: [],
  education_levels: [],
  experience_years: [],
  job_families: [],
  locations: [],
  main_job_families: [],
  other_job_families: [],
  projects: [],
  technology_keywords: [],
  urgent_options: [],
})
const osTheme = useOsTheme()
const drilldownAnchor = ref({ x: 96, y: 180 })
const detail = ref<JobDetailResponse>()
const detailError = ref('')
const detailLoading = ref(false)
const detailMode = ref<'historical' | 'latest'>('latest')
const historicalDetailStatus = ref<TrendDrilldownStatus>()
const historicalDetailDate = ref('')
const mobileSheetExpanded = ref(false)
const mobileSheetOffset = ref(0)
let detailRequest = 0
let comparisonRequest = 0
let sheetPointerID: number | undefined
let sheetPointerStartY = 0

const rangeOptions: Array<{ label: string; value: TrendRange }> = [
  { label: '本周', value: 'week' },
  { label: '本月', value: 'month' },
  { label: '本季度', value: 'quarter' },
  { label: '本年', value: 'year' },
  { label: '全部历史', value: 'all' },
]

const dimensionOptions: Array<{ label: string; value: TrendDimension }> = [
  { label: '技术关键词', value: 'technology' },
  { label: '岗位族', value: 'job_family' },
  { label: '学历', value: 'education' },
  { label: '项目', value: 'project' },
  { label: '工作地点', value: 'location' },
  { label: '工作年限', value: 'experience' },
]

const comparisonOptions: Array<{ label: string; value: TrendComparison }> = [
  { label: '等于', value: 'eq' },
  { label: '大于', value: 'gt' },
  { label: '大于等于', value: 'gte' },
  { label: '小于', value: 'lt' },
  { label: '小于等于', value: 'lte' },
]

const hasRenderedSeries = computed(() => dashboard.rows.some((row) => row.series !== undefined))
const isRefreshing = computed(() => dashboard.rows.some((row) => row.status === 'loading'))
const supportsChartNavigation = computed(
  () => dashboard.range === 'all' || dashboard.range === 'custom',
)
const trendCoverage = computed(
  () => dashboard.rows.find((row) => row.series?.coverage)?.series?.coverage,
)
const chartPeriodText = computed(() => {
  const dates = dashboard.rows.flatMap((row) => row.series?.points.map((point) => point.date) ?? [])
  if (dates.length === 0) return ''
  const firstDate = dates.reduce((earliest, date) => (date < earliest ? date : earliest))
  const lastDate = dates.reduce((latest, date) => (date > latest ? date : latest))
  return `${firstDate.replace(/-/g, '.')} 至 ${lastDate.replace(/-/g, '.')}`
})
const chartOption = computed(() => createTrendChartOption(dashboard.rows, dashboard.range))
const echartsOption = computed(() => chartOption.value as unknown as EChartsOption)
const chartTheme = computed(() => (osTheme.value === 'dark' ? 'dark' : undefined))
const curveLimitReached = computed(() => dashboard.rows.length >= 8)
const eventCurveRows = computed(() =>
  dashboard.rows.filter((row) => row.appliedCondition !== undefined),
)
const eventCurveOptions = computed<SelectOption[]>(() =>
  eventCurveRows.value.slice(0, 8).map((row) => ({
    label: trendCurveLabel(row),
    value: row.id,
  })),
)
const hasAppliedCurve = computed(() =>
  dashboard.rows.some((row) => row.appliedCondition !== undefined),
)
const selectedEventRow = computed(() =>
  eventCurveRows.value.find((row) => row.id === eventStats.selectedCurveId),
)
const selectedEventCurveLabel = computed(() =>
  selectedEventRow.value ? trendCurveLabel(selectedEventRow.value) : '',
)
const eventStatsRows = computed(() => eventStats.response?.rows ?? [])

function preferredDrilldownHeight(maximumHeight: number): number {
  const rowCount = drilldown.response?.items.length ?? drilldown.node?.count ?? 1
  const visibleRowCount = Math.min(5, Math.max(1, rowCount))
  const hasPagination =
    drilldown.response !== undefined &&
    drilldown.response.page.total > drilldown.response.page.page_size
  const baseContentHeight = drilldown.node?.event !== undefined ? 176 : 128
  const compactContentHeight = baseContentHeight + visibleRowCount * 80 + (hasPagination ? 36 : 0)
  const minimumHeight = drilldown.node?.event !== undefined ? 256 : 208
  return Math.min(maximumHeight, Math.max(minimumHeight, compactContentHeight))
}

const drilldownPlacement = computed(() => {
  if (window.innerWidth <= 600) {
    const maximumHeight = Math.min(620, Math.max(280, Math.round(window.innerHeight * 0.72)))
    const height = preferredDrilldownHeight(maximumHeight)
    return {
      opensAbove: false,
      style: {
        '--drilldown-height': `${height}px`,
        '--sheet-offset': `${mobileSheetOffset.value}px`,
      },
    }
  }

  const expandedWorkspaceRight =
    window.innerWidth >= 1440 ? 420 : window.innerWidth >= 1180 ? 368 : 0
  const workspaceRight = configurationExpanded.value ? expandedWorkspaceRight : 0
  const pageRight = window.innerWidth - workspaceRight
  const width = Math.min(380, Math.max(160, pageRight - 36))
  if (drilldown.node?.event !== undefined) {
    const left = Math.max(18, Math.min(drilldownAnchor.value.x - width, pageRight - width - 18))
    const maximumHeight = Math.min(580, Math.max(340, Math.round(window.innerHeight * 0.6)))
    const height = preferredDrilldownHeight(maximumHeight)
    const arrowReach = 9
    const canOpenAbove = drilldownAnchor.value.y - height - arrowReach >= 18
    const canOpenBelow = drilldownAnchor.value.y + height + arrowReach <= window.innerHeight - 18
    const opensAbove = canOpenAbove || !canOpenBelow
    const top = opensAbove
      ? Math.max(18, drilldownAnchor.value.y - height - arrowReach)
      : Math.min(
          drilldownAnchor.value.y + arrowReach,
          window.innerHeight - height - 18,
        )
    const arrowOffset = Math.max(18, Math.min(drilldownAnchor.value.x - left - 8, width - 34))

    return {
      opensAbove,
      style: {
        '--drilldown-arrow-offset': `${arrowOffset}px`,
        '--drilldown-height': `${height}px`,
        '--drilldown-width': `${width}px`,
        '--sheet-offset': `${mobileSheetOffset.value}px`,
        left: `${left}px`,
        top: `${top}px`,
      },
    }
  }

  const maximumHeight = Math.min(580, Math.max(340, Math.round(window.innerHeight * 0.6)))
  const height = preferredDrilldownHeight(maximumHeight)
  const arrowReach = 9
  const canOpenAbove = drilldownAnchor.value.y - height - arrowReach >= 18
  const opensAbove =
    drilldownAnchor.value.y + height + arrowReach > window.innerHeight - 18 && canOpenAbove
  const left = Math.max(18, Math.min(drilldownAnchor.value.x - 34, pageRight - width - 18))
  const top = opensAbove
    ? drilldownAnchor.value.y - height - arrowReach
    : Math.max(18, Math.min(drilldownAnchor.value.y + arrowReach, window.innerHeight - height - 18))
  const arrowOffset = Math.max(18, Math.min(drilldownAnchor.value.x - left - 8, width - 34))

  return {
    opensAbove,
    style: {
      '--drilldown-arrow-offset': `${arrowOffset}px`,
      '--drilldown-height': `${height}px`,
      '--drilldown-width': `${width}px`,
      '--sheet-offset': `${mobileSheetOffset.value}px`,
      left: `${left}px`,
      top: `${top}px`,
    },
  }
})
const drilldownStyle = computed(() => drilldownPlacement.value.style)

function defaultConfigurationExpanded(): boolean {
  return typeof window === 'undefined' || window.innerWidth >= 1440
}

function isComparableDimension(dimension: TrendDimension): boolean {
  return dimension === 'education' || dimension === 'experience'
}

function isCurveComplete(row: TrendCurveRow): boolean {
  return (
    row.condition.value.trim() !== '' &&
    (!isComparableDimension(row.condition.dimension) || row.condition.operator !== '')
  )
}

function clearCompletedBatchPrompt(): void {
  if (!dashboard.rows.every(isCurveComplete)) return
  highlightIncompleteCurves.value = false
  focusedCurveId.value = ''
  curveFeedback.value = ''
}

function curveValueOptions(row: TrendCurveRow): SelectOption[] {
  let options: SelectOption[]
  switch (row.condition.dimension) {
    case 'education':
      options = filterOptions.value.education_levels
      break
    case 'experience':
      options = filterOptions.value.experience_years.map((option) => ({
        label: `${option.label} 年`,
        value: option.value,
      }))
      break
    case 'job_family':
      options = filterOptions.value.job_families
      break
    case 'location':
      options = filterOptions.value.locations
      break
    case 'project':
      options = filterOptions.value.projects
      break
    case 'technology':
      options = filterOptions.value.technology_keywords.map((option) => ({
        label: option.label,
        value: option.value,
      }))
      break
  }
  if (row.condition.value && !options.some((option) => option.value === row.condition.value)) {
    return [{ label: `当前不可用：${row.condition.value}`, value: row.condition.value }, ...options]
  }
  return options
}

function addCurve(): void {
  const curve = dashboard.addCurve({
    dimension: 'technology',
    includeSecondary: false,
    operator: '',
    value: '',
  })
  if (curve === null) {
    curveFeedback.value = '最多添加 8 条曲线。'
  } else {
    curveFeedback.value = ''
  }
}

function startFirstCurve(): void {
  addCurve()
  if (dashboard.rows.length > 0) openConfiguration()
}

function focusCurveCard(row: TrendCurveRow | undefined): void {
  if (row === undefined) return
  focusedCurveId.value = row.id
  void nextTick().then(() => {
    const target = document.querySelector<HTMLElement>(`[data-testid="curve-card-${row.id}"]`)
    if (target === null) return
    target.scrollIntoView?.({ behavior: 'smooth', block: 'center' })
    target.focus({ preventScroll: true })
  })
}

function openConfiguration(): void {
  if (window.innerWidth > 760) {
    configurationManuallyToggled.value = true
    configurationExpanded.value = true
    focusCurveCard(dashboard.rows[0])
    return
  }
  mobileConfigurationOpen.value = true
  void nextTick(() => {
    document.querySelector<HTMLElement>('[data-testid="close-trend-configuration"]')?.focus()
  })
}

function closeConfiguration(): void {
  if (window.innerWidth > 760) {
    configurationManuallyToggled.value = true
    configurationExpanded.value = false
    void nextTick(() => {
      const marker = focusedCurveId.value
        ? document.querySelector<HTMLElement>(
            `[data-testid="curve-marker-${focusedCurveId.value}"]`,
          )
        : document.querySelector<HTMLElement>('[data-testid="curve-marker-rail"] button')
      marker?.focus()
    })
    return
  }
  mobileConfigurationOpen.value = false
  void nextTick(async () => {
    await nextTick()
    document.querySelector<HTMLElement>('[data-testid="open-trend-configuration"]')?.focus()
  })
}

function changeCurveDimension(row: TrendCurveRow, value: string | number | null): void {
  if (typeof value !== 'string') return
  dashboard.changeDimension(row.id, value as TrendDimension)
  clearCompletedBatchPrompt()
}

function changeCurveValue(row: TrendCurveRow, value: string | number | null): void {
  if (typeof value !== 'string') return
  const updated = dashboard.updateCurve(row.id, { ...row.condition, value })
  if (!updated) row.error = '这条曲线已存在。'
  else clearCompletedBatchPrompt()
}

function changeCurveTechnology(row: TrendCurveRow, values: string[]): void {
  const value = values[values.length - 1] ?? ''
  const updated = dashboard.updateCurve(row.id, { ...row.condition, value })
  if (!updated) row.error = '这条曲线已存在。'
  else clearCompletedBatchPrompt()
}

function technologySelection(row: TrendCurveRow): string[] {
  return row.condition.value ? [row.condition.value] : []
}

function changeCurveOperator(row: TrendCurveRow, value: string | number | null): void {
  if (typeof value !== 'string') return
  const updated = dashboard.updateCurve(row.id, {
    ...row.condition,
    operator: value as TrendComparison,
  })
  if (!updated) row.error = '这条曲线已存在。'
  else clearCompletedBatchPrompt()
}

function changeIncludeSecondary(row: TrendCurveRow, includeSecondary: boolean): void {
  const updated = dashboard.updateCurve(row.id, { ...row.condition, includeSecondary })
  if (!updated) row.error = '这条曲线已存在。'
  else clearCompletedBatchPrompt()
}

function applyCurve(row: TrendCurveRow): void {
  if (!isCurveComplete(row)) {
    row.error = '请先选择完整条件。'
    return
  }
  row.error = ''
  void dashboard.applyCurve(row.id)
}

function focusFirstIncompleteCurve(row: TrendCurveRow): void {
  focusCurveCard(row)
}

function applyAllCurves(): void {
  const incompleteRows = dashboard.rows.filter((row) => !isCurveComplete(row))
  highlightIncompleteCurves.value = incompleteRows.length > 0

  for (const row of incompleteRows) row.error = '请完成条件后再应用。'
  for (const row of dashboard.rows) {
    if (!isCurveComplete(row)) continue
    row.error = ''
    void dashboard.applyCurve(row.id)
  }

  if (incompleteRows.length > 0) {
    curveFeedback.value = `还有 ${incompleteRows.length} 张曲线卡待配置。`
    const firstIncomplete = incompleteRows[0]
    if (firstIncomplete !== undefined) focusFirstIncompleteCurve(firstIncomplete)
    return
  }

  focusedCurveId.value = ''
  curveFeedback.value = ''
}

function clearAllCurves(): void {
  for (const row of [...dashboard.rows]) dashboard.removeCurve(row.id)
  highlightIncompleteCurves.value = false
  focusedCurveId.value = ''
  curveFeedback.value = ''
}

async function refreshAll(): Promise<void> {
  await dashboard.refresh()
  await eventStats.load(dashboard.rows, dashboard.range, dashboard.customRange, true)
}

function selectRange(range: TrendRange): void {
  customRangeOpen.value = false
  void dashboard.setRange(range)
}

function applyCustomRange(): void {
  if (!customStartDate.value || !customEndDate.value) {
    customRangeError.value = '请选择起止日期。'
    return
  }
  if (customStartDate.value > customEndDate.value) {
    customRangeError.value = '开始日期不能晚于结束日期。'
    return
  }
  customRangeError.value = ''
  customRangeOpen.value = false
  void dashboard.setCustomRange(customStartDate.value, customEndDate.value)
}

function resetTrendView(): void {
  for (const dataZoomIndex of [0, 1]) {
    trendChart.value?.dispatchAction({
      dataZoomIndex,
      end: 100,
      start: 0,
      type: 'dataZoom',
    })
  }
}

function removeCurve(row: TrendCurveRow): void {
  dashboard.removeCurve(row.id)
  if (focusedCurveId.value === row.id) focusedCurveId.value = ''
  if (dashboard.rows.length === 0) closeConfiguration()
}

function chartLegendChanged(event: unknown): void {
  if (typeof event !== 'object' || event === null) return
  const candidate = event as { name?: unknown; selected?: Record<string, unknown> }
  if (typeof candidate.name !== 'string' || candidate.selected === undefined) return
  const row = dashboard.rows.find((curve) => trendCurveLabel(curve) === candidate.name)
  const visible = candidate.selected[candidate.name]
  if (row !== undefined && typeof visible === 'boolean')
    dashboard.setCurveVisibility(row.id, visible)
}

function chartPointClicked(event: unknown): void {
  if (typeof event !== 'object' || event === null) return
  const candidate = event as {
    data?: unknown
    dataIndex?: unknown
    event?: { event?: { clientX?: unknown; clientY?: unknown } }
    seriesName?: unknown
  }
  if (typeof candidate.seriesName !== 'string') return
  const row = dashboard.rows.find((curve) => trendCurveLabel(curve) === candidate.seriesName)
  if (row !== undefined && !Array.isArray(candidate.data)) {
    void eventStats.selectCurve(row.id, dashboard.rows)
    return
  }
  if (!Array.isArray(candidate.data) || candidate.data.length < 2) return
  const [date, count] = candidate.data
  if (typeof date !== 'string' || (typeof count !== 'number' && count !== null)) return
  const condition = row?.appliedCondition
  if (condition === undefined) return
  const sourceEvent = candidate.event?.event
  if (typeof sourceEvent?.clientX === 'number' && typeof sourceEvent.clientY === 'number') {
    drilldownAnchor.value = { x: sourceEvent.clientX, y: sourceEvent.clientY }
  }
  void drilldown.open(condition, date, count)
}

function selectEventCurve(row: TrendCurveRow): void {
  void eventStats.selectCurve(row.id, dashboard.rows)
}

function openCurveMarker(row: TrendCurveRow): void {
  void eventStats.selectCurve(row.id, dashboard.rows)
  configurationManuallyToggled.value = true
  configurationExpanded.value = true
  focusCurveCard(row)
}

function selectEventCurveById(value: string | number | null): void {
  if (typeof value !== 'string') return
  void eventStats.selectCurve(value, dashboard.rows)
}

function eventDrilldown(
  eventType: 'entered' | 'modified' | 'left' | 'reopened',
  period: 'current' | 'previous' | 'year_over_year',
  count: number | null,
  sourceEvent?: MouseEvent,
): void {
  const response = eventStats.response
  const condition = selectedEventRow.value?.appliedCondition
  if (
    response === undefined ||
    response.status !== 'available' ||
    condition === undefined ||
    count === null
  )
    return
  if (typeof sourceEvent?.clientX === 'number' && typeof sourceEvent.clientY === 'number') {
    drilldownAnchor.value = { x: sourceEvent.clientX, y: sourceEvent.clientY }
  }
  const window = response[period]
  void drilldown.openEvent(condition, eventType, window.start, window.end, count)
}

function eventRowLabel(type: 'entered' | 'modified' | 'left' | 'reopened'): string {
  return trendEventTypeLabel(type)
}

function eventDefinition(type: TrendEventType): string {
  switch (type) {
    case 'entered':
      return '首次被观察到进入该曲线，或修改后从不匹配变为匹配。'
    case 'modified':
      return '修改前后都匹配该曲线，且来源元数据或正文版本发生变化。'
    case 'left':
      return '修改后移出该曲线，或岗位被确认下线。'
    case 'reopened':
      return '岗位确认下线后恢复有效，并重新匹配该曲线。'
  }
}

function eventComparisonText(value: { count: number | null; rate: number | null }): string {
  return formatTrendEventComparison(value)
}

function eventPeriodText(period: { start: string; end: string } | undefined): string {
  return period === undefined ? '' : `${period.start} 至 ${period.end}`
}

function changeDrilldownTitle(event: Event): void {
  const target = event.target
  if (target instanceof HTMLInputElement) drilldown.setTitle(target.value)
}

function openDrilldownDetail(item: TrendDrilldownItem, mode: 'historical' | 'latest'): void {
  const nodeDate = item.event_date ?? drilldown.node?.date
  if (mode === 'historical' && nodeDate === undefined) return
  const request = ++detailRequest
  detail.value = undefined
  detailError.value = ''
  detailLoading.value = true
  detailMode.value = mode
  historicalDetailStatus.value = mode === 'historical' ? item.status : undefined
  historicalDetailDate.value = mode === 'historical' ? nodeDate! : ''
  const requestDetail =
    mode === 'historical' && drilldown.node?.event === 'modified'
      ? getJobHistoryDetail(item.id, nodeDate!, 'modified')
      : getJobDetail(item.id, mode === 'historical' ? { date: nodeDate!, mode } : { mode })
  void requestDetail
    .then((response) => {
      if (request !== detailRequest) return
      detail.value = response
    })
    .catch(() => {
      if (request !== detailRequest) return
      detailError.value = mode === 'historical' ? '历史 JD 暂不可用。' : '岗位详情暂不可用。'
    })
    .finally(() => {
      if (request === detailRequest) detailLoading.value = false
    })
}

function closeDetail(): void {
  detailRequest += 1
  comparisonRequest += 1
  detail.value = undefined
  detailError.value = ''
  detailLoading.value = false
  historicalDetailStatus.value = undefined
  historicalDetailDate.value = ''
}

function retryComparison(): void {
  const historyDate =
    detail.value?.history_date || historicalDetailDate.value || drilldown.node?.date
  if (detailMode.value !== 'historical' || detail.value === undefined || historyDate === undefined)
    return
  const currentDetail = detail.value
  const request = ++comparisonRequest
  const requestDetail =
    drilldown.node?.event === 'modified'
      ? getJobHistoryDetail(currentDetail.id, historyDate, 'modified')
      : getJobDetail(currentDetail.id, { date: historyDate, mode: 'historical' })
  void requestDetail
    .then((response) => {
      if (request === comparisonRequest && detail.value === currentDetail) detail.value = response
    })
    .catch(() => {
      // Keep the already loaded historical JD and its non-blocking retry affordance.
    })
}

function closeDrilldown(): void {
  closeDetail()
  drilldown.close()
  mobileSheetExpanded.value = false
  mobileSheetOffset.value = 0
}

function startSheetDrag(event: PointerEvent): void {
  if (window.innerWidth > 600) return
  sheetPointerID = event.pointerId
  sheetPointerStartY = event.clientY
  mobileSheetOffset.value = 0
  ;(event.currentTarget as HTMLElement | null)?.setPointerCapture?.(event.pointerId)
}

function moveSheetDrag(event: PointerEvent): void {
  if (sheetPointerID !== event.pointerId) return
  mobileSheetOffset.value = Math.max(0, event.clientY - sheetPointerStartY)
}

function finishSheetDrag(event: PointerEvent): void {
  if (sheetPointerID !== event.pointerId) return
  const delta = event.clientY - sheetPointerStartY
  sheetPointerID = undefined
  mobileSheetOffset.value = 0
  if (delta > 96) {
    closeDrilldown()
    return
  }
  if (delta < -42) mobileSheetExpanded.value = true
  else if (delta > 42) mobileSheetExpanded.value = false
}

function handleEscape(event: KeyboardEvent): void {
  if (event.key !== 'Escape') return
  if (detail.value !== undefined || detailLoading.value || detailError.value) closeDetail()
  else if (drilldown.isOpen) closeDrilldown()
  else if (mobileConfigurationOpen.value || configurationExpanded.value) closeConfiguration()
}

function handleViewportResize(): void {
  if (!configurationManuallyToggled.value) {
    configurationExpanded.value = defaultConfigurationExpanded()
  }
}

watch(
  () => drilldown.isOpen,
  (isOpen) => {
    if (window.innerWidth > 600) return
    document.body.style.overflow = isOpen ? 'hidden' : ''
  },
)

onMounted(async () => {
  window.addEventListener('keydown', handleEscape)
  window.addEventListener('resize', handleViewportResize)
  void dashboard.activate()
  try {
    filterOptions.value = await getFilterOptions()
  } catch {
    filterOptionsError.value = '无法读取可选条件，请稍后重试。'
  }
})

watch(
  () => [dashboard.rows, dashboard.range, dashboard.customRange] as const,
  () => {
    void eventStats.load(dashboard.rows, dashboard.range, dashboard.customRange)
  },
  { deep: true, immediate: true },
)

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleEscape)
  window.removeEventListener('resize', handleViewportResize)
  document.body.style.overflow = ''
})
</script>

<template>
  <main
    :class="[
      'job-trend-view',
      { 'job-trend-view--configuration-expanded': configurationExpanded },
    ]"
  >
    <section class="job-trend-view__chart-card" aria-labelledby="trend-dashboard-title">
      <header class="job-trend-view__header">
        <div>
          <p class="job-trend-view__eyebrow">岗位趋势</p>
          <h1 id="trend-dashboard-title">岗位需求变化趋势</h1>
          <p v-if="chartPeriodText">当前周期：{{ chartPeriodText }}</p>
        </div>
        <div class="job-trend-view__header-actions">
          <NButton
            v-if="supportsChartNavigation && hasRenderedSeries"
            data-testid="reset-trend-view"
            tertiary
            size="small"
            @click="resetTrendView"
          >
            复位视图
          </NButton>
          <NButton v-if="hasRenderedSeries" tertiary size="small" @click="refreshAll">
            刷新数据
          </NButton>
        </div>
      </header>

      <div class="job-trend-view__range-stage">
        <nav class="job-trend-view__ranges" aria-label="时间范围">
          <NButton
            v-for="option in rangeOptions"
            :key="option.value"
            size="small"
            :type="dashboard.range === option.value ? 'primary' : 'default'"
            @click="selectRange(option.value)"
          >
            {{ option.label }}
          </NButton>
          <NButton
            data-testid="open-custom-trend-range"
            size="small"
            :type="dashboard.range === 'custom' || customRangeOpen ? 'primary' : 'default'"
            @click="customRangeOpen = !customRangeOpen"
          >
            自定义
          </NButton>
        </nav>
        <div
          v-if="customRangeOpen"
          class="job-trend-view__custom-range"
          role="group"
          aria-label="自定义日期范围"
        >
          <label>
            <span>起始日期</span>
            <FilterDatePicker
              v-model="customStartDate"
              aria-label="选择趋势起始日期"
              placeholder="起始日期"
            />
          </label>
          <span class="job-trend-view__custom-range-separator" aria-hidden="true">至</span>
          <label>
            <span>结束日期</span>
            <FilterDatePicker
              v-model="customEndDate"
              aria-label="选择趋势结束日期"
              placeholder="结束日期"
            />
          </label>
          <NButton
            data-testid="apply-custom-trend-range"
            size="small"
            type="primary"
            @click="applyCustomRange"
          >
            应用
          </NButton>
          <p
            v-if="trendCoverage?.first_date && trendCoverage?.last_date"
            class="job-trend-view__coverage"
          >
            系统数据覆盖：{{ trendCoverage.first_date }} 至 {{ trendCoverage.last_date }}
          </p>
          <p v-if="customRangeError" role="alert">{{ customRangeError }}</p>
        </div>
        <p
          v-if="supportsChartNavigation && hasRenderedSeries && !customRangeOpen"
          class="job-trend-view__all-history-gestures"
          data-testid="all-history-gestures"
        >
          滚轮缩放时间 · Shift+滚轮缩放岗位数 · 拖拽平移
        </p>
      </div>

      <div
        class="job-trend-view__chart-region"
        data-testid="trend-chart-region"
        :aria-busy="isRefreshing"
      >
        <VChart
          v-if="hasRenderedSeries"
          ref="trendChart"
          class="job-trend-view__chart"
          :option="echartsOption"
          :theme="chartTheme"
          :loading="isRefreshing"
          :loading-options="{ color: '#007cab', maskColor: 'rgba(255,255,255,0.54)', text: '' }"
          :autoresize="{ throttle: 120 }"
          @click="chartPointClicked"
          @legendselectchanged="chartLegendChanged"
        />
        <section
          v-else
          class="job-trend-view__empty"
          data-testid="trend-empty-state"
          aria-live="polite"
        >
          <NEmpty description="先设定一个观察条件，再比较岗位需求变化。">
            <template #extra>
              <NButton data-testid="start-first-curve" type="primary" @click="startFirstCurve"
                >开始配置首条曲线</NButton
              >
            </template>
          </NEmpty>
        </section>
        <div
          v-if="isRefreshing && !hasRenderedSeries"
          class="job-trend-view__loading"
          aria-live="polite"
        >
          正在更新曲线…
        </div>
      </div>

      <section
        v-if="hasAppliedCurve"
        class="job-trend-view__event-card"
        aria-labelledby="trend-event-title"
      >
        <header class="job-trend-view__event-header">
          <div>
            <p class="job-trend-view__event-eyebrow">周期事件</p>
            <h2 id="trend-event-title">{{ selectedEventCurveLabel || '岗位事件统计' }}</h2>
            <p v-if="eventStats.response?.as_of">截至 {{ eventStats.response.as_of }}</p>
          </div>
          <div
            class="job-trend-view__event-selector"
            data-testid="event-curve-selector"
            aria-label="事件统计曲线选择器"
          >
            <span>统计曲线</span>
            <NSelect
              :value="eventStats.selectedCurveId || null"
              :options="eventCurveOptions"
              size="small"
              @update:value="selectEventCurveById"
            />
          </div>
        </header>
        <p v-if="dashboard.range === 'all'" class="job-trend-view__event-empty">
          请选择具体时间范围。
        </p>
        <p
          v-else-if="selectedEventRow && !supportsTrendEvents(selectedEventRow)"
          class="job-trend-view__event-development"
        >
          开发中
        </p>
        <template v-else-if="eventStats.status === 'loading'">
          <div class="job-trend-view__event-skeleton" aria-label="正在计算事件统计">
            <span v-for="index in 4" :key="index" />
          </div>
        </template>
        <p
          v-else-if="eventStats.status === 'error'"
          class="job-trend-view__event-error"
          role="alert"
        >
          {{ eventStats.error }} <button type="button" @click="eventStats.retry">重试</button>
        </p>
        <p
          v-else-if="eventStats.status === 'idle' || eventStats.response === undefined"
          class="job-trend-view__event-empty"
        >
          请选择已应用曲线。
        </p>
        <p
          v-else-if="eventStats.response.status === 'unavailable'"
          class="job-trend-view__event-empty"
        >
          暂无可比数据
        </p>
        <div v-else class="job-trend-view__event-table-wrap">
          <div class="job-trend-view__event-table-shell">
            <table class="job-trend-view__event-table">
              <thead>
                <tr>
                  <th scope="col">事件</th>
                  <th scope="col">
                    当前周期
                    <NPopover
                      content-class="job-trend-view__event-popover"
                      placement="top"
                      raw
                      :show-arrow="true"
                      :theme-overrides="eventPopoverThemeOverrides"
                      trigger="hover"
                    >
                      <template #trigger>
                        <button
                          type="button"
                          class="job-trend-view__event-help"
                          aria-label="当前周期日期范围"
                          data-testid="event-period-current-help"
                          :data-tooltip="`当前周期：${eventPeriodText(eventStats.response.current)}`"
                        >
                          i
                        </button>
                      </template>
                      <div class="job-trend-view__event-popover-content">
                        <strong>当前周期</strong>
                        <span>{{ eventPeriodText(eventStats.response.current) }}</span>
                      </div>
                    </NPopover>
                  </th>
                  <th scope="col">
                    同比
                    <NPopover
                      content-class="job-trend-view__event-popover"
                      placement="top"
                      raw
                      :show-arrow="true"
                      :theme-overrides="eventPopoverThemeOverrides"
                      trigger="hover"
                    >
                      <template #trigger>
                        <button
                          type="button"
                          class="job-trend-view__event-help"
                          aria-label="同比周期日期范围"
                          data-testid="event-period-year-over-year-help"
                          :data-tooltip="`同比周期：${eventPeriodText(eventStats.response.year_over_year)}`"
                        >
                          i
                        </button>
                      </template>
                      <div class="job-trend-view__event-popover-content">
                        <strong>同比周期</strong>
                        <span>{{ eventPeriodText(eventStats.response.year_over_year) }}</span>
                      </div>
                    </NPopover>
                  </th>
                  <th scope="col">
                    环比
                    <NPopover
                      content-class="job-trend-view__event-popover"
                      placement="top"
                      raw
                      :show-arrow="true"
                      :theme-overrides="eventPopoverThemeOverrides"
                      trigger="hover"
                    >
                      <template #trigger>
                        <button
                          type="button"
                          class="job-trend-view__event-help"
                          aria-label="环比周期日期范围"
                          data-testid="event-period-previous-help"
                          :data-tooltip="`环比周期：${eventPeriodText(eventStats.response.previous)}`"
                        >
                          i
                        </button>
                      </template>
                      <div class="job-trend-view__event-popover-content">
                        <strong>环比周期</strong>
                        <span>{{ eventPeriodText(eventStats.response.previous) }}</span>
                      </div>
                    </NPopover>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="row in eventStatsRows"
                  :key="row.type"
                  :class="`job-trend-view__event-row--${row.type}`"
                >
                  <th scope="row">
                    {{ eventRowLabel(row.type) }}
                    <NPopover
                      content-class="job-trend-view__event-popover"
                      placement="top-start"
                      raw
                      :show-arrow="true"
                      :theme-overrides="eventPopoverThemeOverrides"
                      trigger="hover"
                    >
                      <template #trigger>
                        <span
                          class="job-trend-view__event-help"
                          :aria-label="`${eventRowLabel(row.type)}统计口径`"
                          :data-testid="`event-definition-${row.type}`"
                          :data-tooltip="eventDefinition(row.type)"
                          role="img"
                          tabindex="0"
                        >
                          i
                        </span>
                      </template>
                      <div class="job-trend-view__event-popover-content">
                        <strong>{{ eventRowLabel(row.type) }}</strong>
                        <span>{{ eventDefinition(row.type) }}</span>
                      </div>
                    </NPopover>
                  </th>
                  <td data-label="当前周期">
                    <div class="job-trend-view__event-metric">
                      <button
                        class="job-trend-view__event-count job-trend-view__event-count--current"
                        data-testid="event-count-current"
                        type="button"
                        :disabled="eventStats.response?.status !== 'available'"
                        @click="eventDrilldown(row.type, 'current', row.current, $event)"
                      >
                        {{ row.current }}
                      </button>
                    </div>
                  </td>
                  <td data-label="同比">
                    <template v-if="row.year_over_year.count === null">
                      <span class="job-trend-view__event-unavailable">
                        {{ eventComparisonText(row.year_over_year) }}
                      </span>
                    </template>
                    <template v-else>
                      <div class="job-trend-view__event-metric">
                        <button
                          class="job-trend-view__event-count job-trend-view__event-count--comparison"
                          data-testid="event-count-year-over-year"
                          type="button"
                          @click="
                            eventDrilldown(
                              row.type,
                              'year_over_year',
                              row.year_over_year.count,
                              $event,
                            )
                          "
                        >
                          {{ row.year_over_year.count }}
                        </button>
                        <span
                          class="job-trend-view__event-rate"
                          data-testid="event-rate-year-over-year"
                          :class="{
                            'job-trend-view__event-rate--empty': row.year_over_year.rate === null,
                          }"
                        >
                          {{ formatTrendEventRate(row.year_over_year.rate) }}
                        </span>
                      </div>
                    </template>
                  </td>
                  <td data-label="环比">
                    <template v-if="row.previous.count === null">
                      <span class="job-trend-view__event-unavailable">
                        {{ eventComparisonText(row.previous) }}
                      </span>
                    </template>
                    <template v-else>
                      <div class="job-trend-view__event-metric">
                        <button
                          class="job-trend-view__event-count job-trend-view__event-count--comparison"
                          data-testid="event-count-previous"
                          type="button"
                          @click="eventDrilldown(row.type, 'previous', row.previous.count, $event)"
                        >
                          {{ row.previous.count }}
                        </button>
                        <span
                          class="job-trend-view__event-rate"
                          data-testid="event-rate-previous"
                          :class="{
                            'job-trend-view__event-rate--empty': row.previous.rate === null,
                          }"
                        >
                          {{ formatTrendEventRate(row.previous.rate) }}
                        </span>
                      </div>
                    </template>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>
    </section>

    <div
      v-if="drilldown.isOpen"
      class="job-trend-view__drilldown-backdrop"
      aria-hidden="true"
      @click="closeDrilldown"
    />
    <section
      v-if="drilldown.isOpen"
      :class="[
        'job-trend-view__drilldown',
        {
          'job-trend-view__drilldown--above': drilldownPlacement.opensAbove,
          'job-trend-view__drilldown--expanded': mobileSheetExpanded,
          'job-trend-view__drilldown--event': drilldown.node?.event !== undefined,
        },
      ]"
      :style="drilldownStyle"
      aria-label="趋势节点岗位"
      aria-modal="true"
      role="dialog"
    >
      <header
        class="job-trend-view__drilldown-header"
        @pointerdown="startSheetDrag"
        @pointermove="moveSheetDrag"
        @pointerup="finishSheetDrag"
        @pointercancel="finishSheetDrag"
      >
        <div>
          <p data-testid="event-drilldown-title">
            {{
              drilldown.node?.event
                ? `${eventRowLabel(drilldown.node.event)} · ${selectedEventCurveLabel}`
                : `节点日期 ${drilldown.node?.date ?? ''}`
            }}
          </p>
          <strong v-if="drilldown.node?.event" class="job-trend-view__drilldown-event-summary">
            <span data-testid="event-drilldown-period"
              >{{ drilldown.node?.startDate }} 至 {{ drilldown.node?.endDate }}</span
            >
            <span data-testid="event-drilldown-count"
              >周期内去重
              {{ drilldown.response?.matched_total ?? drilldown.node?.count ?? 0 }} 个岗位</span
            >
          </strong>
          <strong v-else>
            匹配 {{ drilldown.response?.matched_total ?? drilldown.node?.count ?? 0 }} 个岗位
          </strong>
        </div>
        <button type="button" aria-label="关闭节点岗位" @click="closeDrilldown">×</button>
      </header>
      <input
        class="job-trend-view__drilldown-search"
        :value="drilldown.title"
        aria-label="按岗位名称搜索"
        placeholder="搜索岗位名称"
        type="search"
        @input="changeDrilldownTitle"
      />
      <p
        v-if="drilldown.response?.count_matches_expected === false"
        class="job-trend-view__drilldown-mismatch"
        role="status"
      >
        数据已更新，请重新打开节点
        <NButton size="tiny" tertiary @click="drilldown.retry">重试</NButton>
      </p>
      <div v-if="drilldown.isLoading" class="job-trend-view__drilldown-skeleton" aria-live="polite">
        <span v-for="index in 4" :key="index" />
      </div>
      <p v-else-if="drilldown.error" class="job-trend-view__drilldown-error" role="alert">
        {{ drilldown.error }}
        <NButton size="tiny" tertiary @click="drilldown.retry">重试</NButton>
      </p>
      <p v-else-if="drilldown.response?.items.length === 0" class="job-trend-view__drilldown-empty">
        {{
          drilldown.node?.event
            ? '该周期没有符合条件的岗位'
            : drilldown.node?.count === 0 && !drilldown.title
              ? '该日没有匹配岗位'
              : '没有匹配的岗位'
        }}
      </p>
      <ol
        v-else
        :class="[
          'job-trend-view__drilldown-list',
          { 'job-trend-view__drilldown-list--event': drilldown.node?.event !== undefined },
        ]"
        :data-testid="drilldown.node?.event ? 'event-drilldown-list' : undefined"
      >
        <li v-for="item in drilldown.response?.items" :key="item.id">
          <strong>{{ item.title }}</strong>
          <span
            :class="`job-trend-view__drilldown-status job-trend-view__drilldown-status--${item.status}`"
          >
            {{
              item.status === 'current'
                ? '仍有效'
                : item.status === 'changed'
                  ? '有改动'
                  : '确认下线'
            }}
          </span>
          <small
            v-if="drilldown.response?.mode === 'event'"
            class="job-trend-view__drilldown-event-meta"
          >
            {{ item.event_date }}<template v-if="item.reason"> · {{ item.reason }}</template>
          </small>
          <div>
            <NButton
              v-if="item.can_view_historical"
              size="tiny"
              @click="openDrilldownDetail(item, 'historical')"
              >查看历史 JD</NButton
            >
            <NButton
              v-if="item.can_view_latest && item.status !== 'confirmed_closed'"
              size="tiny"
              tertiary
              @click="openDrilldownDetail(item, 'latest')"
              >查看最新{{ item.can_view_historical ? ' JD' : '' }}</NButton
            >
          </div>
        </li>
      </ol>
      <footer
        v-if="
          drilldown.response && drilldown.response.page.total > drilldown.response.page.page_size
        "
        class="job-trend-view__drilldown-pagination"
      >
        <NPagination
          :item-count="drilldown.response.page.total"
          :page="drilldown.page"
          :page-size="drilldown.response.page.page_size"
          :page-slot="5"
          size="small"
          @update:page="drilldown.setPage"
        />
      </footer>
    </section>

    <Transition name="job-trend-detail">
      <section
        v-if="detail !== undefined || detailLoading || detailError"
        class="job-trend-view__detail-drawer"
        aria-label="岗位详情"
        aria-modal="true"
        role="dialog"
      >
        <header>
          <p>
            {{
              detailMode === 'historical'
                ? `历史 JD · ${detail?.history_date || historicalDetailDate || drilldown.node?.date || drilldown.node?.startDate || ''}`
                : '最新 JD'
            }}
          </p>
          <button type="button" aria-label="关闭岗位详情" @click="closeDetail">×</button>
        </header>
        <div class="job-trend-view__detail-content">
          <JobDetailSkeleton v-if="detailLoading" />
          <p v-else-if="detailError" class="job-trend-view__detail-error" role="alert">
            {{ detailError }}
          </p>
          <JDDetailReadingLayout
            v-else-if="detail"
            :historical-context="
              detailMode === 'historical' && historicalDetailStatus
                ? { currentStatus: historicalDetailStatus }
                : undefined
            "
            :job="detail"
            :show-observation-history="false"
            @retry-comparison="retryComparison"
          />
        </div>
      </section>
    </Transition>

    <div
      v-if="!mobileConfigurationOpen"
      class="job-trend-view__mobile-panel job-trend-view__configuration-trigger"
    >
      <NButton
        data-testid="open-trend-configuration"
        :aria-expanded="mobileConfigurationOpen"
        aria-controls="trend-configuration-rail"
        type="primary"
        @click="openConfiguration"
      >
        配置曲线<span v-if="dashboard.rows.length">（{{ dashboard.rows.length }}）</span>
      </NButton>
    </div>

    <div
      v-if="mobileConfigurationOpen"
      class="job-trend-view__configuration-backdrop"
      aria-hidden="true"
      data-testid="trend-configuration-backdrop"
      @click="closeConfiguration"
    />

    <aside
      :class="[
        'job-trend-view__rail job-trend-view__rail--drawer',
        {
          'job-trend-view__rail--desktop-expanded': configurationExpanded,
          'job-trend-view__rail--desktop-collapsed': !configurationExpanded,
        },
        { 'job-trend-view__rail--open': mobileConfigurationOpen },
        { 'job-trend-view__rail--mobile-open': mobileConfigurationOpen },
        { 'job-trend-view__rail--drawer-open': mobileConfigurationOpen },
      ]"
      data-testid="trend-configuration-rail"
      aria-label="曲线配置"
    >
      <div
        class="job-trend-view__curve-marker-rail"
        :class="{
          'job-trend-view__curve-marker-rail--hidden':
            configurationExpanded || mobileConfigurationOpen,
        }"
        aria-label="已配置曲线"
        data-testid="curve-marker-rail"
      >
        <div
          v-for="(row, index) in dashboard.rows"
          :key="row.id"
          class="job-trend-view__curve-marker-slot"
          :data-testid="`curve-marker-slot-${row.id}`"
        >
          <span
            v-if="index > 0"
            class="job-trend-view__curve-marker-divider"
            aria-hidden="true"
          ></span>
          <button
            :aria-label="`展开曲线：${trendCurveLabel(row) || '未设置曲线'}`"
            :data-testid="`curve-marker-${row.id}`"
            class="job-trend-view__curve-marker"
            :style="{ '--curve-color': trendCurveColor(row) }"
            type="button"
            @click="openCurveMarker(row)"
          >
            <span class="job-trend-view__curve-marker-core" aria-hidden="true"></span>
            <span class="job-trend-view__curve-marker-note" aria-hidden="true">
              {{ trendCurveLabel(row) || '未设置曲线' }}
            </span>
          </button>
        </div>
        <div v-if="dashboard.rows.length === 0" class="job-trend-view__curve-marker-slot">
          <button
            class="job-trend-view__curve-marker job-trend-view__curve-marker--empty"
            type="button"
            aria-label="打开曲线配置"
            @click="openConfiguration"
          >
            <span aria-hidden="true">+</span>
          </button>
        </div>
      </div>

      <div
        class="job-trend-view__rail-content"
        :class="{
          'job-trend-view__rail-content--hidden':
            !configurationExpanded && !mobileConfigurationOpen,
        }"
      >
        <header class="job-trend-view__rail-header">
          <div>
            <p>曲线配置</p>
            <small>每条曲线只统计一个条件</small>
          </div>
          <button
            class="job-trend-view__rail-close"
            data-testid="close-trend-configuration"
            type="button"
            aria-label="关闭曲线配置"
            @click="closeConfiguration"
          >
            ×
          </button>
        </header>

        <p v-if="filterOptionsError" class="job-trend-view__options-error" role="alert">
          {{ filterOptionsError }}
        </p>
        <p v-if="curveFeedback" class="job-trend-view__curve-feedback" role="status">
          {{ curveFeedback }}
        </p>

        <ol v-if="dashboard.rows.length" class="job-trend-view__curve-list">
        <li
          v-for="(row, index) in dashboard.rows"
          :key="row.id"
          :class="[
            'job-trend-view__curve-row',
            {
              'job-trend-view__curve-row--incomplete':
                highlightIncompleteCurves && !isCurveComplete(row),
              'job-trend-view__curve-row--focused': focusedCurveId === row.id,
            },
          ]"
          :data-testid="`curve-card-${row.id}`"
          :style="{ '--curve-color': trendCurveColor(row) }"
          tabindex="-1"
          @click="selectEventCurve(row)"
        >
          <header>
            <span class="job-trend-view__curve-index">{{
              String(index + 1).padStart(2, '0')
            }}</span>
            <div class="job-trend-view__curve-identity">
              <strong>{{ trendCurveLabel(row) || '未设置曲线' }}</strong>
            </div>
            <span
              v-if="highlightIncompleteCurves && !isCurveComplete(row)"
              class="job-trend-view__curve-pending"
            >
              待配置
            </span>
            <button type="button" aria-label="删除曲线" @click.stop="removeCurve(row)">×</button>
          </header>

          <label class="job-trend-view__curve-field">
            <span>统计维度</span>
            <NSelect
              :value="row.condition.dimension"
              :options="dimensionOptions"
              size="small"
              @update:value="changeCurveDimension(row, $event)"
            />
          </label>

          <div
            v-if="isComparableDimension(row.condition.dimension)"
            class="job-trend-view__curve-pair"
          >
            <label class="job-trend-view__curve-field">
              <span>比较方式</span>
              <NSelect
                :value="row.condition.operator || null"
                :options="comparisonOptions"
                placeholder="选择"
                size="small"
                @update:value="changeCurveOperator(row, $event)"
              />
            </label>
            <label class="job-trend-view__curve-field">
              <span>{{ row.condition.dimension === 'education' ? '学历门槛' : '最低年限' }}</span>
              <NSelect
                :value="row.condition.value || null"
                :options="curveValueOptions(row)"
                filterable
                placeholder="选择"
                size="small"
                @update:value="changeCurveValue(row, $event)"
              />
            </label>
          </div>

          <label
            v-else-if="row.condition.dimension === 'technology'"
            class="job-trend-view__curve-field"
          >
            <span>统计条件</span>
            <div class="job-trend-view__technology-select">
              <TechnologyKeywordSelect
                :model-value="technologySelection(row)"
                :options="filterOptions.technology_keywords"
                placeholder="选择技术关键词"
                @update:model-value="changeCurveTechnology(row, $event)"
              />
            </div>
          </label>

          <label v-else class="job-trend-view__curve-field">
            <span>统计条件</span>
            <NSelect
              :value="row.condition.value || null"
              :options="curveValueOptions(row)"
              filterable
              placeholder="选择"
              size="small"
              @update:value="changeCurveValue(row, $event)"
            />
          </label>

          <label
            v-if="row.condition.dimension === 'job_family'"
            class="job-trend-view__secondary-switch"
          >
            <span>包含副岗位族</span>
            <NSwitch
              :value="row.condition.includeSecondary"
              size="small"
              @update:value="changeIncludeSecondary(row, $event)"
            />
          </label>

          <div class="job-trend-view__curve-actions">
            <span
              :class="['job-trend-view__curve-state', `job-trend-view__curve-state--${row.status}`]"
            >
              {{
                row.status === 'loading'
                  ? '更新中'
                  : row.status === 'error'
                    ? '未更新'
                    : row.status === 'idle' && row.condition.value
                      ? '待应用'
                      : ''
              }}
            </span>
            <NButton
              size="small"
              type="primary"
              :disabled="!isCurveComplete(row) || row.status === 'loading'"
              @click="applyCurve(row)"
              >应用</NButton
            >
          </div>
          <p v-if="row.error" class="job-trend-view__curve-error" role="alert">{{ row.error }}</p>
        </li>
        </ol>

        <div class="job-trend-view__rail-footer" data-testid="curve-rail-footer">
          <span>{{ dashboard.rows.length }}/8 条</span>
          <div class="job-trend-view__rail-footer-actions">
            <NButton
              data-testid="clear-all-curves"
              class="job-trend-view__clear-all"
              size="tiny"
              quaternary
              @click="clearAllCurves"
            >
              清除全部
            </NButton>
            <NButton
              data-testid="apply-all-curves"
              size="small"
              type="primary"
              :disabled="isRefreshing"
              @click="applyAllCurves"
            >
              应用全部
            </NButton>
            <NButton size="small" :disabled="curveLimitReached" @click="addCurve">
              + 添加曲线
            </NButton>
          </div>
        </div>
      </div>
    </aside>
  </main>
</template>

<style scoped>
.job-trend-view {
  --trend-card-surface: rgba(255, 255, 255, 0.96);
  --trend-curve-border: #e1ebef;
  --trend-curve-surface: #ffffff;
  --trend-ink: #14313d;
  --trend-muted: #687981;
  --trend-rail-surface: #f7fbfc;
  --trend-rule: #dce8ed;
  --trend-rule-soft: #e6eef1;
  --trend-strong: #27424d;
  --trend-subtle: #71828a;
  --trend-surface-shadow: 0 20px 44px -38px rgba(17, 73, 96, 0.56);
  --trend-rail-shadow: -18px 0 42px -38px rgba(11, 61, 82, 0.54);
  --trend-workspace-left: 32px;
  --trend-workspace-right: 0px;
  align-items: start;
  display: grid;
  gap: 16px;
  grid-template-columns: minmax(0, 1fr) 48px;
  min-height: 100vh;
  padding: 28px 32px 32px;
  transition: grid-template-columns 260ms cubic-bezier(0.22, 0.9, 0.25, 1);
}
.job-trend-view--configuration-expanded {
  grid-template-columns: minmax(0, 1fr) 320px;
}
.job-trend-view__chart-card {
  background: var(--trend-card-surface);
  border: 1px solid var(--trend-rule);
  border-radius: 20px;
  box-shadow: var(--trend-surface-shadow);
  min-height: calc(100dvh - 60px);
  overflow: hidden;
}
.job-trend-view__header {
  align-items: flex-start;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  padding: 30px 34px 18px;
}
.job-trend-view__eyebrow {
  color: #008287;
  font-size: 0.72rem;
  font-weight: 760;
  letter-spacing: 0.12em;
  margin: 0 0 6px;
}
.job-trend-view__header h1 {
  color: var(--trend-ink);
  font-size: clamp(1.35rem, 2.2vw, 1.82rem);
  letter-spacing: -0.045em;
  line-height: 1.2;
  margin: 0;
}
.job-trend-view__header p:not(.job-trend-view__eyebrow) {
  color: var(--trend-muted);
  font-size: 0.82rem;
  margin: 8px 0 0;
}
.job-trend-view__range-stage {
  border-bottom: 1px solid var(--trend-rule-soft);
  position: relative;
  z-index: 10;
}
.job-trend-view__ranges {
  display: flex;
  gap: 8px;
  padding: 0 34px 18px;
}
.job-trend-view__custom-range {
  align-items: end;
  background: color-mix(in srgb, var(--trend-rail-surface), transparent 12%);
  border: 1px solid var(--trend-rule-soft);
  border-radius: 12px;
  display: grid;
  gap: 8px;
  grid-template-columns: minmax(132px, 176px) auto minmax(132px, 176px) auto;
  left: 34px;
  padding: 9px;
  position: absolute;
  top: calc(100% + 8px);
  width: min(540px, calc(100% - 68px));
  z-index: 2;
}
.job-trend-view__custom-range label {
  color: var(--trend-muted);
  display: grid;
  font-size: 0.68rem;
  font-weight: 700;
  gap: 4px;
  min-width: 0;
}
.job-trend-view__custom-range-separator {
  color: var(--trend-subtle);
  font-size: 0.72rem;
  padding-bottom: 11px;
}
.job-trend-view__custom-range p {
  color: #a8424d;
  font-size: 0.72rem;
  grid-column: 1 / -1;
  margin: 0;
}
.job-trend-view__custom-range .job-trend-view__coverage {
  color: var(--trend-subtle);
  grid-column: 1 / -1;
  margin: 0;
}
.job-trend-view__all-history-gestures {
  background: color-mix(in srgb, var(--trend-card-surface), transparent 5%);
  border: 1px solid color-mix(in srgb, var(--trend-rule), transparent 24%);
  border-radius: 999px;
  color: var(--trend-subtle);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.67rem;
  letter-spacing: -0.01em;
  margin: 0;
  padding: 5px 8px;
  position: absolute;
  right: 34px;
  top: calc(100% + 10px);
  z-index: 1;
}
.job-trend-view__chart-region {
  min-height: 540px;
  padding: 10px 16px 20px;
  position: relative;
}
.job-trend-view__event-card {
  border-top: 1px solid var(--trend-rule-soft);
  padding: 28px 34px 34px;
}
.job-trend-view__event-header {
  align-items: center;
  display: flex;
  gap: 16px;
  justify-content: space-between;
}
.job-trend-view__event-eyebrow {
  color: #008287;
  font-size: 0.68rem;
  font-weight: 760;
  letter-spacing: 0.12em;
  margin: 0 0 4px;
}
.job-trend-view__event-header h2 {
  color: var(--trend-ink);
  font-size: 1.18rem;
  letter-spacing: -0.03em;
  margin: 0;
}
.job-trend-view__event-header p:last-child {
  color: var(--trend-muted);
  font-size: 0.75rem;
  margin: 6px 0 0;
}
.job-trend-view__event-selector {
  align-items: center;
  color: var(--trend-muted);
  display: flex;
  flex-wrap: wrap;
  font-size: 0.68rem;
  font-weight: 720;
  gap: 6px 8px;
  justify-content: flex-end;
}
.job-trend-view__event-selector :deep(.n-base-selection) {
  min-width: min(280px, 42vw);
}
.job-trend-view__event-selector :deep(.n-base-selection-label) {
  font-size: 0.78rem;
}
.job-trend-view__event-table th:has(.job-trend-view__event-help) {
  white-space: nowrap;
}
.job-trend-view__event-help {
  align-items: center;
  background: color-mix(in srgb, #007f88, transparent 90%);
  border: 1px solid color-mix(in srgb, #007f88, transparent 68%);
  border-radius: 50%;
  color: #00737b;
  cursor: help;
  display: inline-flex;
  font-family: ui-sans-serif, system-ui, sans-serif;
  font-size: 0.61rem;
  font-style: normal;
  font-weight: 850;
  height: 17px;
  justify-content: center;
  line-height: 1;
  margin-left: 4px;
  padding: 0;
  transition:
    background-color 150ms ease,
    border-color 150ms ease,
    color 150ms ease,
    transform 150ms ease;
  vertical-align: 1px;
  width: 17px;
}
.job-trend-view__event-help:hover,
.job-trend-view__event-help:focus-visible {
  background: #007f88;
  border-color: #007f88;
  color: #fff;
  transform: translateY(-1px);
}
.job-trend-view__event-help:focus-visible,
.job-trend-view__event-table td button:focus-visible {
  outline: 3px solid rgba(0, 124, 171, 0.24);
  outline-offset: 2px;
}
.job-trend-view__event-empty,
.job-trend-view__event-development,
.job-trend-view__event-error {
  color: var(--trend-muted);
  font-size: 0.78rem;
  margin: 16px 0 0;
}
.job-trend-view__event-development {
  font-weight: 700;
}
.job-trend-view__event-error button {
  background: none;
  border: 0;
  color: #007cab;
  cursor: pointer;
  font-weight: 700;
  padding: 0 0 0 8px;
}
.job-trend-view__event-table-wrap {
  overflow-x: auto;
}
.job-trend-view__event-table-shell {
  background: linear-gradient(135deg, #fbfeff 0%, #f6fbfc 100%);
  border: 1px solid var(--trend-rule);
  border-radius: 16px;
  box-shadow: 0 14px 32px -30px rgba(17, 73, 96, 0.52);
  overflow: hidden;
  width: min(100%, 640px);
}
.job-trend-view__event-table {
  border-collapse: separate;
  border-spacing: 0;
  min-width: 560px;
  table-layout: fixed;
  width: 100%;
}
.job-trend-view__event-table th,
.job-trend-view__event-table td {
  border-bottom: 1px solid var(--trend-rule-soft);
  color: var(--trend-strong);
  font-size: 0.8rem;
  padding: 15px 18px;
  text-align: right;
}
.job-trend-view__event-table th:first-child,
.job-trend-view__event-table td:first-child {
  width: 24%;
}
.job-trend-view__event-table th:not(:first-child),
.job-trend-view__event-table td:not(:first-child) {
  text-align: center;
  width: 25.333%;
}
.job-trend-view__event-table thead th {
  background: rgba(232, 243, 246, 0.64);
  color: #60757d;
  font-size: 0.68rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  padding-bottom: 12px;
  padding-top: 12px;
  text-transform: uppercase;
}
.job-trend-view__event-table thead th:first-child {
  text-align: left;
}
.job-trend-view__event-table thead th:not(:first-child) {
  border-left: 1px solid rgba(220, 232, 237, 0.7);
}
.job-trend-view__event-table tbody th {
  background: rgba(255, 255, 255, 0.38);
  position: relative;
  font-weight: 700;
  padding-left: 28px;
  text-align: left;
}
.job-trend-view__event-table tbody th::before {
  background: var(--trend-subtle);
  border-radius: 999px;
  content: '';
  height: 7px;
  left: 17px;
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
}
.job-trend-view__event-row--entered th::before,
.job-trend-view__event-row--reopened th::before {
  background: #008a5a;
}
.job-trend-view__event-row--modified th::before {
  background: #d19a00;
}
.job-trend-view__event-row--left th::before {
  background: #b44953;
}
.job-trend-view__event-table tbody tr:nth-child(even) th,
.job-trend-view__event-table tbody tr:nth-child(even) td {
  background: rgba(244, 250, 251, 0.62);
}
.job-trend-view__event-table tbody tr:last-child th,
.job-trend-view__event-table tbody tr:last-child td {
  border-bottom: 0;
}
.job-trend-view__event-metric {
  align-items: baseline;
  display: inline-flex;
  gap: 7px;
  justify-content: center;
  min-width: 74px;
}
.job-trend-view__event-count {
  font-variant-numeric: tabular-nums;
  font-weight: 760;
}
.job-trend-view__event-count--current {
  color: var(--trend-ink) !important;
  font-size: 1rem !important;
}
.job-trend-view__event-count--comparison {
  color: #007cab !important;
}
.job-trend-view__event-rate {
  color: var(--trend-subtle);
  font-size: 0.72rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.job-trend-view__event-rate--empty {
  color: var(--trend-muted);
}
.job-trend-view__event-unavailable {
  color: var(--trend-subtle);
  font-size: 0.74rem;
}
.job-trend-view__event-table td button {
  background: none;
  border: 0;
  color: inherit;
  cursor: pointer;
  font: inherit;
  padding: 0;
  text-decoration: underline;
  text-decoration-color: color-mix(in srgb, currentColor, transparent 66%);
  text-underline-offset: 3px;
}
.job-trend-view__event-table td button:disabled {
  color: var(--trend-subtle);
  cursor: not-allowed;
  text-decoration: none;
}

:global(.job-trend-view__event-popover) {
  background: #edf7f9 !important;
  border: 1px solid #c6dfe5 !important;
  border-radius: 11px !important;
  box-shadow: 0 14px 30px -20px rgba(17, 73, 96, 0.5) !important;
  color: #244853 !important;
  max-width: min(280px, calc(100vw - 32px));
  padding: 11px 13px !important;
}
:global(.job-trend-view__event-popover .n-popover-arrow) {
  background: #edf7f9 !important;
  border-color: #c6dfe5 !important;
}
.job-trend-view__event-popover-content {
  display: grid;
  gap: 3px;
  line-height: 1.45;
}
.job-trend-view__event-popover-content strong {
  color: #163943;
  font-size: 0.74rem;
  font-weight: 800;
}
.job-trend-view__event-popover-content span {
  color: #52717a;
  font-size: 0.72rem;
}
.job-trend-view__event-skeleton {
  display: grid;
  gap: 8px;
  margin-top: 16px;
}
.job-trend-view__event-skeleton span {
  background: linear-gradient(90deg, #edf4f6 25%, #f8fbfc 50%, #edf4f6 75%);
  background-size: 200% 100%;
  border-radius: 6px;
  height: 34px;
}
.job-trend-view__chart {
  height: 540px;
  width: 100%;
}
.job-trend-view__empty {
  align-items: center;
  color: var(--trend-muted);
  display: flex;
  height: 510px;
  justify-content: center;
}
.job-trend-view__loading {
  background: rgba(248, 253, 254, 0.72);
  color: #007cab;
  font-size: 0.8rem;
  font-weight: 700;
  inset: 0;
  padding-top: 42px;
  position: absolute;
  text-align: center;
}
.job-trend-view__drilldown-backdrop {
  background: rgba(17, 42, 52, 0.12);
  inset: 0 var(--trend-workspace-right) 0 0;
  position: fixed;
  z-index: 28;
}
.job-trend-view__drilldown {
  background: var(--trend-card-surface);
  border: 1px solid color-mix(in srgb, var(--trend-rule), #007cab 22%);
  border-radius: 18px;
  box-sizing: border-box;
  box-shadow: 0 24px 64px -28px rgba(6, 43, 57, 0.52);
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: var(--drilldown-height, min(60dvh, 580px));
  max-height: calc(100dvh - 36px);
  min-height: 0;
  overflow: hidden;
  padding: 14px;
  position: fixed;
  width: var(--drilldown-width, min(390px, calc(100vw - 430px)));
  z-index: 29;
}
.job-trend-view__drilldown::before {
  border-bottom: 9px solid color-mix(in srgb, var(--trend-card-surface), #007cab 8%);
  border-left: 8px solid transparent;
  border-right: 8px solid transparent;
  content: '';
  left: var(--drilldown-arrow-offset);
  position: absolute;
  top: -9px;
}
.job-trend-view__drilldown--above::before {
  border-bottom: 0;
  border-top: 9px solid color-mix(in srgb, var(--trend-card-surface), #007cab 8%);
  bottom: -9px;
  top: auto;
}
.job-trend-view__drilldown--event {
  --drilldown-event-success-bg: color-mix(in srgb, #008a5a, transparent 86%);
  --drilldown-event-success-ink: #126b54;
  --drilldown-event-warning-bg: color-mix(in srgb, #d19a00, transparent 84%);
  --drilldown-event-warning-ink: #805500;
  --drilldown-event-closed-bg: color-mix(in srgb, #b44953, transparent 85%);
  --drilldown-event-closed-ink: #993d4c;
}
.job-trend-view__drilldown-header {
  align-items: center;
  display: flex;
  flex: 0 0 auto;
  justify-content: space-between;
}
.job-trend-view__drilldown-header p {
  color: var(--trend-subtle);
  font-size: 0.68rem;
  margin: 0 0 3px;
}
.job-trend-view__drilldown-header strong {
  color: var(--trend-ink);
  font-size: 0.92rem;
}
.job-trend-view__drilldown--event .job-trend-view__drilldown-header > div {
  min-width: 0;
}
.job-trend-view__drilldown-event-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 3px 10px;
  line-height: 1.4;
}
.job-trend-view__drilldown-event-summary span {
  min-width: 0;
}
.job-trend-view__drilldown-event-summary span:first-child {
  overflow-wrap: anywhere;
}
.job-trend-view__drilldown-header button,
.job-trend-view__detail-drawer > header button {
  background: transparent;
  border: 0;
  color: var(--trend-muted);
  cursor: pointer;
  font-size: 1.35rem;
  line-height: 1;
}
.job-trend-view__drilldown-search {
  background: color-mix(in srgb, var(--trend-curve-surface), transparent 4%);
  border: 1px solid var(--trend-rule);
  border-radius: 9px;
  color: var(--trend-ink);
  flex: 0 0 auto;
  font: inherit;
  font-size: 0.8rem;
  outline: none;
  padding: 7px 9px;
}
.job-trend-view__drilldown-search:focus {
  border-color: #007cab;
  box-shadow: 0 0 0 3px rgba(0, 124, 171, 0.12);
}
.job-trend-view__drilldown-skeleton {
  align-content: start;
  display: grid;
  flex: 1 1 auto;
  gap: 9px;
  min-height: 0;
  overflow: hidden;
}
.job-trend-view__drilldown-skeleton span {
  animation: job-trend-drilldown-pulse 1.15s ease-in-out infinite alternate;
  background: linear-gradient(
    100deg,
    var(--trend-rule-soft),
    color-mix(in srgb, var(--trend-rule-soft), #fff 48%),
    var(--trend-rule-soft)
  );
  border-radius: 8px;
  height: 56px;
}
.job-trend-view__drilldown-skeleton span:nth-child(2) {
  animation-delay: 100ms;
}
.job-trend-view__drilldown-skeleton span:nth-child(3) {
  animation-delay: 200ms;
}
.job-trend-view__drilldown-skeleton span:nth-child(4) {
  animation-delay: 300ms;
}
@keyframes job-trend-drilldown-pulse {
  to {
    opacity: 0.54;
  }
}
.job-trend-view__drilldown-list {
  display: grid;
  flex: 1 1 auto;
  gap: 0;
  list-style: none;
  margin: 0;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 0 6px 0 0;
  scrollbar-color: color-mix(in srgb, #007cab, transparent 38%) transparent;
  scrollbar-gutter: stable;
  scrollbar-width: thin;
}
.job-trend-view__drilldown-list::-webkit-scrollbar {
  width: 7px;
}
.job-trend-view__drilldown-list::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, #007cab, transparent 38%);
  border: 2px solid transparent;
  border-radius: 999px;
  background-clip: padding-box;
}
.job-trend-view__drilldown-list li {
  align-items: center;
  border-bottom: 1px solid var(--trend-rule-soft);
  display: grid;
  gap: 3px 7px;
  grid-template-areas:
    'title status'
    'meta meta'
    'actions actions';
  grid-template-columns: minmax(0, 1fr) auto;
  padding: 7px 0;
}
.job-trend-view__drilldown-list li > strong {
  color: var(--trend-strong);
  font-size: 0.8rem;
  grid-area: title;
  line-height: 1.35;
  overflow-wrap: anywhere;
}
.job-trend-view__drilldown-event-meta {
  color: var(--trend-subtle);
  font-size: 0.68rem;
  grid-area: meta;
}
.job-trend-view__drilldown-list li > div {
  display: flex;
  gap: 4px;
  grid-area: actions;
}
.job-trend-view__drilldown-list li > div :deep(.n-button) {
  min-height: 23px;
  padding-left: 7px;
  padding-right: 7px;
}
.job-trend-view__drilldown-list--event li {
  padding: 10px 0;
}
.job-trend-view__drilldown-list--event li > strong {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
}
.job-trend-view__drilldown-list--event li > div {
  flex-wrap: wrap;
  gap: 6px;
  padding-top: 2px;
}
.job-trend-view__drilldown--event .job-trend-view__drilldown-status--current {
  background: var(--drilldown-event-success-bg);
  color: var(--drilldown-event-success-ink);
}
.job-trend-view__drilldown--event .job-trend-view__drilldown-status--changed {
  background: var(--drilldown-event-warning-bg);
  color: var(--drilldown-event-warning-ink);
}
.job-trend-view__drilldown--event .job-trend-view__drilldown-status--confirmed_closed {
  background: var(--drilldown-event-closed-bg);
  color: var(--drilldown-event-closed-ink);
}
.job-trend-view__drilldown-list--event li > div :deep(.n-button) {
  min-height: 30px;
}
.job-trend-view__drilldown-status {
  border-radius: 99px;
  font-size: 0.62rem;
  font-weight: 760;
  grid-area: status;
  padding: 3px 6px;
  white-space: nowrap;
}
.job-trend-view__drilldown-status--current {
  background: #d8f1e7;
  color: #126b54;
}
.job-trend-view__drilldown-status--changed {
  background: #fff0c8;
  color: #8a5a00;
}
.job-trend-view__drilldown-status--confirmed_closed {
  background: #f6dfe1;
  color: #993d4c;
}
.job-trend-view__drilldown-empty,
.job-trend-view__drilldown-error {
  align-self: center;
  color: var(--trend-muted);
  font-size: 0.8rem;
  margin: 0;
  text-align: center;
}
.job-trend-view__drilldown-error {
  color: #a8424d;
}
.job-trend-view__drilldown-mismatch {
  background: color-mix(in srgb, #e69f00, transparent 87%);
  border-radius: 8px;
  color: #805500;
  flex: 0 0 auto;
  font-size: 0.7rem;
  margin: -3px 0 0;
  padding: 6px 8px;
}
.job-trend-view__drilldown-pagination {
  border-top: 1px solid var(--trend-rule-soft);
  display: flex;
  flex: 0 0 auto;
  justify-content: center;
  padding-top: 8px;
}
.job-trend-view__detail-drawer {
  background: var(--trend-card-surface);
  border: 1px solid color-mix(in srgb, var(--trend-rule), #007cab 22%);
  border-radius: 22px;
  box-shadow: 0 -22px 60px -28px rgba(5, 38, 50, 0.62);
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  height: min(78dvh, 780px);
  inset: auto var(--trend-workspace-right) 32px var(--trend-workspace-left);
  min-height: 0;
  overflow: hidden;
  padding: 22px;
  position: fixed;
  width: auto;
  z-index: 32;
}
.job-trend-view__detail-content {
  flex: 1 1 auto;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  padding-right: 6px;
  scrollbar-gutter: stable;
  scrollbar-width: thin;
}
.job-trend-view__detail-content::-webkit-scrollbar {
  width: 8px;
}
.job-trend-view__detail-content::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, #007cab, transparent 48%);
  border: 2px solid transparent;
  border-radius: 999px;
  background-clip: padding-box;
}
.job-trend-detail-enter-active,
.job-trend-detail-leave-active {
  transition:
    opacity 180ms ease,
    transform 320ms cubic-bezier(0.22, 0.9, 0.25, 1);
}
.job-trend-detail-enter-from,
.job-trend-detail-leave-to {
  opacity: 0;
  transform: translateY(calc(100% + 28px));
}
.job-trend-view__detail-drawer > header {
  align-items: center;
  border-bottom: 1px solid var(--trend-rule-soft);
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
  padding-bottom: 12px;
}
.job-trend-view__detail-drawer > header p {
  color: var(--trend-ink);
  font-size: 0.9rem;
  font-weight: 760;
  margin: 0;
}
.job-trend-view__detail-error {
  color: #a8424d;
}
.job-trend-view__curve-feedback {
  color: #00716f;
  font-size: 0.78rem;
  line-height: 1.45;
  margin: 0 2px 12px;
}
.job-trend-view__rail {
  background: var(--trend-rail-surface);
  border: 1px solid var(--trend-rule);
  border-radius: 18px;
  box-shadow: var(--trend-rail-shadow);
  box-sizing: border-box;
  display: grid;
  grid-column: 2;
  height: calc(100dvh - 28px);
  min-width: 0;
  overflow: hidden;
  padding: 20px 16px 14px;
  position: sticky;
  top: 0;
  transition:
    background-color 180ms ease,
    padding 260ms cubic-bezier(0.22, 0.9, 0.25, 1),
    opacity 180ms ease,
    transform 260ms cubic-bezier(0.22, 0.9, 0.25, 1),
    visibility 0s linear;
  transform: translateX(0);
  visibility: visible;
  width: 100%;
}
.job-trend-view__rail--open {
  display: grid;
  transform: translateX(0);
}
.job-trend-view__rail--desktop-collapsed {
  padding: 10px 6px;
}
.job-trend-view__rail-content,
.job-trend-view__curve-marker-rail {
  grid-area: 1 / 1;
  min-height: 0;
  min-width: 0;
  transition:
    opacity 180ms ease,
    transform 260ms cubic-bezier(0.22, 0.9, 0.25, 1),
    visibility 0s linear;
}
.job-trend-view__rail-content {
  display: flex;
  flex-direction: column;
  opacity: 1;
  transform: translateX(0);
  visibility: visible;
}
.job-trend-view__rail-content--hidden {
  opacity: 0;
  pointer-events: none;
  transform: translateX(12px);
  visibility: hidden;
  transition-delay: 0s, 0s, 260ms;
}
.job-trend-view__curve-marker-rail {
  align-content: start;
  display: grid;
  gap: 10px;
  grid-auto-rows: 32px;
  justify-items: center;
  opacity: 1;
  overflow: visible;
  transform: translateX(0);
  visibility: visible;
}
.job-trend-view__curve-marker-rail--hidden {
  opacity: 0;
  pointer-events: none;
  transform: translateX(-12px);
  visibility: hidden;
  transition-delay: 0s, 0s, 260ms;
}
.job-trend-view__curve-marker-slot {
  align-items: center;
  display: flex;
  height: 32px;
  justify-content: center;
  min-width: 32px;
  position: relative;
  width: 100%;
}
.job-trend-view__curve-marker-divider {
  background: color-mix(in srgb, var(--trend-rule), transparent 16%);
  height: 1px;
  left: 0;
  position: absolute;
  right: 0;
  top: -5px;
}
.job-trend-view__curve-marker {
  align-items: center;
  background: color-mix(in srgb, var(--curve-color), var(--trend-rail-surface) 82%);
  border: 1px solid color-mix(in srgb, var(--curve-color), var(--trend-rule) 48%);
  border-radius: 9px;
  cursor: pointer;
  display: inline-flex;
  height: 32px;
  justify-content: center;
  padding: 0;
  position: relative;
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
  width: 32px;
}
.job-trend-view__curve-marker-note {
  background: color-mix(in srgb, var(--curve-color), #ffffff 86%);
  border: 1px solid color-mix(in srgb, var(--curve-color), #ffffff 58%);
  border-right: 2px solid var(--curve-color);
  border-radius: 8px 3px 3px 8px;
  box-shadow: 0 8px 18px -14px rgba(18, 54, 68, 0.78);
  color: color-mix(in srgb, var(--curve-color), var(--trend-ink) 34%);
  display: block;
  font-size: 0.68rem;
  font-weight: 760;
  height: 28px;
  line-height: 28px;
  max-width: 180px;
  min-width: 112px;
  opacity: 0;
  overflow: hidden;
  padding: 0 9px;
  pointer-events: auto;
  position: absolute;
  right: calc(100% + 8px);
  text-align: left;
  text-overflow: ellipsis;
  top: 50%;
  transform: translateX(8px) translateY(-50%);
  transition:
    opacity 120ms ease,
    transform 160ms ease,
    visibility 0s linear 160ms;
  transition-delay: 0ms, 0ms, 160ms;
  visibility: hidden;
  white-space: nowrap;
  z-index: 2;
}
.job-trend-view__curve-marker-slot:hover {
  z-index: 3;
}
.job-trend-view__curve-marker-slot:hover .job-trend-view__curve-marker-note {
  opacity: 1;
  transform: translateX(0) translateY(-50%);
  transition-delay: 200ms, 200ms, 0s;
  visibility: visible;
}
.job-trend-view__curve-marker-core {
  background: var(--curve-color, #008287);
  border-radius: 999px;
  display: block;
  height: 16px;
  width: 5px;
}
.job-trend-view__curve-marker:hover,
.job-trend-view__curve-marker:focus-visible {
  border-color: var(--curve-color);
  box-shadow: 0 6px 16px -10px var(--curve-color);
  outline: none;
  transform: translateY(-1px);
}
.job-trend-view__curve-marker--empty span {
  background: transparent;
  color: var(--trend-subtle);
  font-size: 1.15rem;
  height: auto;
  line-height: 1;
  width: auto;
}
.job-trend-view__rail-header {
  align-items: flex-start;
  display: flex;
  justify-content: space-between;
  padding: 0 4px 12px;
}
.job-trend-view__rail-header p {
  color: var(--trend-ink);
  font-size: 1rem;
  font-weight: 740;
  letter-spacing: -0.02em;
  margin: 0;
}
.job-trend-view__rail-header small {
  color: var(--trend-subtle);
  display: block;
  font-size: 0.72rem;
  margin-top: 4px;
}
.job-trend-view__rail-close {
  align-items: center;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 9px;
  color: var(--trend-muted);
  cursor: pointer;
  display: inline-flex;
  font-size: 1.55rem;
  justify-content: center;
  line-height: 1;
  min-height: 32px;
  padding: 0 6px;
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    color 160ms ease;
}
.job-trend-view__rail-close:hover,
.job-trend-view__rail-close:focus-visible {
  background: color-mix(in srgb, var(--trend-rule), transparent 42%);
  border-color: var(--trend-rule);
  color: var(--trend-ink);
  outline: none;
}
.job-trend-view__options-error,
.job-trend-view__curve-error {
  color: #a8424d;
  font-size: 0.74rem;
  line-height: 1.45;
  margin: 0 2px 12px;
}
.job-trend-view__curve-list {
  align-content: start;
  display: grid;
  flex: 1 1 auto;
  gap: 10px;
  list-style: none;
  margin: 0;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 0 0 10px;
  scrollbar-gutter: stable;
  scrollbar-width: thin;
}
.job-trend-view__curve-list::-webkit-scrollbar {
  width: 10px;
}
.job-trend-view__curve-list::-webkit-scrollbar-track {
  background: transparent;
}
.job-trend-view__curve-list::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--trend-subtle), transparent 42%);
  border: 3px solid transparent;
  border-radius: 999px;
  background-clip: padding-box;
}
.job-trend-view__curve-row {
  background: var(--trend-curve-surface);
  border: 1px solid var(--trend-curve-border);
  border-top: 4px solid var(--curve-color);
  border-radius: 12px;
  box-shadow: 0 5px 16px -14px rgba(18, 67, 86, 0.72);
  display: grid;
  gap: 7px;
  padding: 11px;
  scroll-margin: 18px;
  transition:
    box-shadow 180ms ease,
    outline-color 180ms ease;
}
.job-trend-view__curve-row--incomplete {
  background: color-mix(in srgb, var(--curve-color), var(--trend-curve-surface) 90%);
  border-color: color-mix(in srgb, var(--curve-color), var(--trend-curve-surface) 52%);
  border-top-color: var(--curve-color);
  border-top-width: 5px;
  box-shadow: 0 8px 22px -18px color-mix(in srgb, var(--curve-color), transparent 35%);
}
.job-trend-view__curve-row--focused {
  outline: 2px solid color-mix(in srgb, var(--curve-color), transparent 30%);
  outline-offset: 2px;
}
.job-trend-view__curve-row > header {
  align-items: center;
  display: flex;
  gap: 6px;
  min-width: 0;
}
.job-trend-view__curve-identity {
  display: grid;
  flex: 1;
  gap: 2px;
  min-width: 0;
}
.job-trend-view__curve-identity strong {
  color: var(--trend-strong);
  font-size: 0.84rem;
  font-weight: 800;
  letter-spacing: -0.015em;
  line-height: 1.3;
  min-width: 0;
  overflow-wrap: anywhere;
}
.job-trend-view__curve-pending {
  background: color-mix(in srgb, var(--curve-color), var(--trend-curve-surface) 82%);
  border-radius: 999px;
  color: color-mix(in srgb, var(--curve-color), var(--trend-strong) 32%);
  flex: none;
  font-size: 0.62rem;
  font-weight: 800;
  line-height: 1;
  padding: 5px 7px;
}
.job-trend-view__curve-row > header button {
  background: transparent;
  border: 0;
  color: var(--trend-subtle);
  cursor: pointer;
  font-size: 1.1rem;
  line-height: 1;
  padding: 2px;
}
.job-trend-view__curve-index {
  color: var(--curve-color);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.66rem;
  font-weight: 760;
}
.job-trend-view__curve-field {
  color: var(--trend-muted);
  display: grid;
  font-size: 0.64rem;
  font-weight: 700;
  gap: 4px;
}
.job-trend-view__curve-field :deep(.n-base-selection) {
  min-height: 30px;
}
.job-trend-view__curve-field :deep(.n-base-selection-label) {
  font-size: 0.78rem;
}
.job-trend-view__technology-select {
  min-width: 0;
  width: 100%;
}
.job-trend-view__technology-select :deep(.technology-keyword-select__trigger) {
  border-radius: 8px;
  font-size: 0.78rem;
  min-height: 30px;
  padding: 0 10px;
}
.job-trend-view__curve-pair {
  display: grid;
  gap: 7px;
  grid-template-columns: 1fr 1fr;
}
.job-trend-view__secondary-switch {
  align-items: center;
  color: var(--trend-muted);
  display: flex !important;
  font-size: 0.64rem !important;
  min-height: 29px;
  justify-content: space-between;
}
.job-trend-view__curve-actions,
.job-trend-view__rail-footer {
  align-items: center;
  display: flex;
  justify-content: space-between;
}
.job-trend-view__curve-actions {
  border-top: 1px solid var(--trend-rule-soft);
  margin-top: 0;
  padding-top: 8px;
}
.job-trend-view__curve-state {
  color: var(--trend-subtle);
  font-size: 0.71rem;
}
.job-trend-view__curve-state--loading {
  color: #0072b2;
}
.job-trend-view__curve-state--error {
  color: #a8424d;
}
.job-trend-view__rail-footer {
  border-top: 1px solid var(--trend-rule-soft);
  color: var(--trend-subtle);
  flex: 0 0 auto;
  font-size: 0.72rem;
  margin-top: auto;
  padding: 10px 4px 0;
}
.job-trend-view__rail-footer-actions {
  align-items: center;
  display: flex;
  gap: 7px;
}
.job-trend-view__mobile-panel {
  display: none;
}
.job-trend-view__configuration-trigger,
.job-trend-view__configuration-backdrop {
  display: none;
}

@media (min-width: 761px) {
  .job-trend-view {
    grid-template-columns: minmax(0, 1fr) 48px;
  }
  .job-trend-view--configuration-expanded {
    grid-template-columns: minmax(0, 1fr) 320px;
  }
  .job-trend-view__rail--drawer {
    max-width: min(368px, 92vw);
    overflow: hidden;
    position: sticky;
    width: 100%;
  }
  .job-trend-view__rail--desktop-collapsed {
    overflow: visible;
  }
  .job-trend-view__rail--drawer .job-trend-view__curve-list {
    overflow-y: auto;
  }
  .job-trend-view__rail--drawer .job-trend-view__rail-footer {
    flex: 0 0 auto;
  }
  .job-trend-view__configuration-backdrop,
  .job-trend-view__configuration-trigger {
    display: none;
  }
}
:global(.app-shell[data-color-scheme='dark'] .job-trend-view) {
  --trend-card-surface: rgba(22, 47, 58, 0.96);
  --trend-curve-border: #294752;
  --trend-curve-surface: #193640;
  --trend-ink: #edf7f8;
  --trend-muted: #a8c3ca;
  --trend-rail-surface: #102b35;
  --trend-rule: #2b4c58;
  --trend-rule-soft: #294752;
  --trend-strong: #edf7f8;
  --trend-subtle: #b7ccd2;
  --trend-surface-shadow: 0 20px 44px -38px rgba(0, 0, 0, 0.86);
  --trend-rail-shadow: -18px 0 42px -38px rgba(0, 0, 0, 0.86);
}
:global(.app-shell[data-color-scheme='dark'] .job-trend-view__drilldown--event) {
  --drilldown-event-success-bg: color-mix(in srgb, #56c596, transparent 78%);
  --drilldown-event-success-ink: #b5efd6;
  --drilldown-event-warning-bg: color-mix(in srgb, #e3ae37, transparent 76%);
  --drilldown-event-warning-ink: #ffe6a5;
  --drilldown-event-closed-bg: color-mix(in srgb, #c65e66, transparent 76%);
  --drilldown-event-closed-ink: #ffc1c6;
}
@media (min-width: 1440px) {
  .job-trend-view {
    --trend-workspace-right: 0px;
    padding: 28px 32px 32px;
  }
}
@media (min-width: 1180px) and (max-width: 1439px) {
  .job-trend-view {
    --trend-workspace-right: 0px;
    padding: 24px 28px 28px;
  }
  .job-trend-view__rail--desktop-expanded {
    padding: 18px 14px 12px;
  }
  .job-trend-view__header {
    padding: 26px 28px 16px;
  }
  .job-trend-view__ranges {
    gap: 7px;
    padding: 0 28px 16px;
  }
  .job-trend-view__chart-region {
    min-height: 500px;
    padding: 8px 12px 16px;
  }
  .job-trend-view__chart,
  .job-trend-view__empty {
    height: 500px;
  }
  .job-trend-view__event-card {
    padding: 24px 28px 28px;
  }
}
@media (min-width: 761px) and (max-width: 1179px) {
  .job-trend-view {
    --trend-workspace-right: 0px;
    padding: 20px 32px 28px 32px;
    padding-right: 32px;
  }
  .job-trend-view__chart-card {
    min-height: calc(100dvh - 48px);
  }
  .job-trend-view__header {
    padding: 24px 26px 14px;
  }
  .job-trend-view__ranges {
    gap: 6px;
    padding: 0 26px 14px;
  }
  .job-trend-view__chart-region {
    min-height: 460px;
    padding: 8px 10px 14px;
  }
  .job-trend-view__chart,
  .job-trend-view__empty {
    height: 460px;
  }
  .job-trend-view__event-card {
    padding: 22px 26px 26px;
  }
}
@media (max-width: 760px) {
  .job-trend-view {
    --trend-workspace-right: 0px;
    display: block;
    grid-template-columns: none;
  }
  .job-trend-view {
    min-height: auto;
    padding: 64px 16px 24px;
    padding-bottom: calc(88px + env(safe-area-inset-bottom));
  }
  .job-trend-view__chart-card {
    min-height: 0;
  }
  .job-trend-view__header {
    padding: 24px 22px 16px;
  }
  .job-trend-view__header-actions {
    margin-top: 2px;
  }
  .job-trend-view__ranges {
    gap: 6px;
    overflow-x: auto;
    padding: 0 22px 14px;
    scrollbar-width: none;
  }
  .job-trend-view__ranges::-webkit-scrollbar {
    display: none;
  }
  .job-trend-view__custom-range {
    gap: 6px;
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
    left: 22px;
    width: calc(100% - 44px);
  }
  .job-trend-view__custom-range > :deep(.n-button) {
    grid-column: 1 / -1;
  }
  .job-trend-view__all-history-gestures {
    display: none;
  }
  .job-trend-view__chart-region {
    min-height: 430px;
    padding: 4px 4px 12px;
  }
  .job-trend-view__chart,
  .job-trend-view__empty {
    height: 430px;
  }
  .job-trend-view__event-card {
    padding: 20px 16px 24px;
  }
  .job-trend-view__event-header {
    align-items: flex-start;
    flex-wrap: wrap;
  }
  .job-trend-view__event-header h2 {
    font-size: 1rem;
  }
  .job-trend-view__event-selector {
    margin-left: auto;
  }
  .job-trend-view__mobile-panel {
    bottom: calc(18px + env(safe-area-inset-bottom));
    display: block;
    left: 50%;
    position: fixed;
    transform: translateX(-50%);
    z-index: 11;
  }
  .job-trend-view__configuration-trigger {
    bottom: calc(18px + env(safe-area-inset-bottom));
    display: block;
    left: 50%;
    position: fixed;
    transform: translateX(-50%);
    z-index: 11;
  }
  .job-trend-view__configuration-backdrop {
    background: rgba(17, 42, 52, 0.16);
    display: block;
    inset: 0;
    position: fixed;
    z-index: 19;
  }
  .job-trend-view__rail {
    border: 1px solid var(--trend-rule);
    border-radius: 18px 18px 0 0;
    bottom: 0;
    display: grid;
    left: 0;
    max-height: min(82dvh, 720px);
    opacity: 0;
    padding: 18px 14px 12px;
    pointer-events: none;
    position: fixed;
    right: 0;
    top: auto;
    transform: translateY(18px);
    visibility: hidden;
    width: auto;
    z-index: 20;
  }
  .job-trend-view__curve-marker-rail {
    display: none;
  }
  .job-trend-view__rail--mobile-open {
    display: grid;
    opacity: 1;
    pointer-events: auto;
    transform: translateY(0);
    visibility: visible;
  }
  .job-trend-view__rail-close {
    display: block;
  }
  .job-trend-view__curve-list {
    max-height: 56dvh;
  }
  .job-trend-view__detail-drawer {
    height: auto;
    inset: 0;
    width: 100%;
  }
  .job-trend-detail-enter-from {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .job-trend-view {
    transition-duration: 140ms !important;
  }

  .job-trend-view__rail {
    transition:
      background-color 140ms ease,
      padding 140ms ease,
      opacity 140ms ease,
      visibility 0s linear 140ms !important;
    transform: none !important;
  }

  .job-trend-view__rail--mobile-open {
    transition-delay: 0s, 0s, 0s, 0s !important;
  }

  .job-trend-view__rail-content,
  .job-trend-view__curve-marker-rail,
  .job-trend-view__rail-content--hidden,
  .job-trend-view__curve-marker-rail--hidden {
    transition: opacity 140ms ease, visibility 0s linear 0s !important;
    transform: none !important;
  }

  .job-trend-view__rail-content--hidden,
  .job-trend-view__curve-marker-rail--hidden {
    transition-delay: 0s, 140ms !important;
  }

  .job-trend-view__curve-marker,
  .job-trend-view__curve-marker-note,
  .job-trend-view__rail-close {
    transition-duration: 140ms !important;
  }
  .job-trend-view__curve-marker-note {
    transform: translateY(-50%) !important;
  }
}
@media (max-width: 600px) {
  .job-trend-view__drilldown-backdrop {
    background: rgba(8, 33, 43, 0.36);
    right: 0;
  }
  .job-trend-view__drilldown {
    border-radius: 20px 20px 0 0;
    bottom: 0;
    height: var(--drilldown-height);
    left: 0 !important;
    max-height: min(84dvh, 720px);
    min-height: 0;
    padding: 18px 16px calc(16px + env(safe-area-inset-bottom));
    top: auto !important;
    transform: translateY(var(--sheet-offset));
    transition:
      height 180ms ease,
      transform 150ms ease;
    width: 100%;
  }
  .job-trend-view__drilldown--expanded {
    height: min(84dvh, 720px);
  }
  .job-trend-view__drilldown::before {
    background: var(--trend-rule);
    border: 0;
    border-radius: 999px;
    height: 4px;
    left: calc(50% - 21px);
    top: 8px;
    width: 42px;
  }
  .job-trend-view__drilldown-header {
    padding-top: 6px;
  }
  .job-trend-view__drilldown--event .job-trend-view__drilldown-header {
    align-items: flex-start;
  }
  .job-trend-view__drilldown--event .job-trend-view__drilldown-event-summary {
    display: grid;
    gap: 2px;
  }
  .job-trend-view__event-table-wrap {
    overflow: visible;
  }
  .job-trend-view__event-table-shell {
    border-radius: 14px;
  }
  .job-trend-view__event-table {
    border-collapse: separate;
    border-spacing: 0 8px;
    min-width: 0;
  }
  .job-trend-view__event-table thead {
    height: 1px;
    overflow: hidden;
    position: absolute;
    width: 1px;
    clip: rect(1px, 1px, 1px, 1px);
  }
  .job-trend-view__event-table tbody tr {
    background: var(--trend-rail-surface);
    border: 1px solid var(--trend-rule-soft);
    border-radius: 12px;
    display: grid;
    gap: 0;
    padding: 8px 10px;
  }
  .job-trend-view__event-table tbody tr:first-child th,
  .job-trend-view__event-table tbody tr:first-child td {
    padding-top: 8px;
  }
  .job-trend-view__event-table tbody tr:last-child th,
  .job-trend-view__event-table tbody tr:last-child td {
    border-bottom: 0;
  }
  .job-trend-view__event-table tbody th,
  .job-trend-view__event-table tbody td {
    align-items: center;
    border-bottom: 1px solid var(--trend-rule-soft);
    display: flex;
    justify-content: space-between;
    min-height: 34px;
    padding: 7px 0;
    text-align: right;
  }
  .job-trend-view__event-table tbody th {
    justify-content: flex-start;
    padding-left: 12px;
  }
  .job-trend-view__event-table tbody th::before {
    left: 0;
  }
  .job-trend-view__event-table tbody td {
    background: transparent !important;
  }
  .job-trend-view__event-table tbody td::before {
    color: var(--trend-subtle);
    content: attr(data-label);
    font-size: 0.68rem;
    font-weight: 700;
    text-align: left;
  }
  .job-trend-view__event-table tbody tr > :last-child {
    border-bottom: 0;
  }
  .job-trend-view__event-table .job-trend-view__event-count--current {
    font-size: 0.96rem !important;
  }
  .job-trend-view__event-metric {
    min-width: 0;
  }
  .job-trend-view__detail-drawer {
    border-radius: 0;
    padding: 18px 14px calc(20px + env(safe-area-inset-bottom));
  }
}
</style>
