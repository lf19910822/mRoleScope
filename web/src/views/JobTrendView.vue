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
import { NButton, NEmpty, NSelect, NSwitch, type SelectOption, useOsTheme } from 'naive-ui'
import { computed, onMounted, ref } from 'vue'
import VChart from 'vue-echarts'

import { getFilterOptions, type FilterOptionsResponse } from '@/api/jd-api'
import { createTrendChartOption, trendCurveLabel } from '@/lib/job-trend-chart'
import { useJobTrendDashboard } from '@/lib/job-trend-dashboard'
import type {
  TrendComparison,
  TrendCurveRow,
  TrendDimension,
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
const mobileConfigurationOpen = ref(false)
const filterOptionsError = ref('')
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
const pointFeedback = ref('')
const osTheme = useOsTheme()
let pointFeedbackTimer: ReturnType<typeof setTimeout> | undefined

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
const chartOption = computed(() => createTrendChartOption(dashboard.rows, dashboard.range))
const echartsOption = computed(() => chartOption.value as unknown as EChartsOption)
const chartTheme = computed(() => (osTheme.value === 'dark' ? 'dark' : undefined))
const curveLimitReached = computed(() => dashboard.rows.length >= 8)

function isComparableDimension(dimension: TrendDimension): boolean {
  return dimension === 'education' || dimension === 'experience'
}

function isCurveComplete(row: TrendCurveRow): boolean {
  return (
    row.condition.value.trim() !== '' &&
    (!isComparableDimension(row.condition.dimension) || row.condition.operator !== '')
  )
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
    pointFeedback.value = curveLimitReached.value ? '最多添加 8 条曲线。' : '请先完成当前这条曲线。'
  }
}

function changeCurveDimension(row: TrendCurveRow, value: string | number | null): void {
  if (typeof value !== 'string') return
  dashboard.changeDimension(row.id, value as TrendDimension)
}

function changeCurveValue(row: TrendCurveRow, value: string | number | null): void {
  if (typeof value !== 'string') return
  const updated = dashboard.updateCurve(row.id, { ...row.condition, value })
  if (!updated) row.error = '这条曲线已存在。'
}

function changeCurveOperator(row: TrendCurveRow, value: string | number | null): void {
  if (typeof value !== 'string') return
  const updated = dashboard.updateCurve(row.id, {
    ...row.condition,
    operator: value as TrendComparison,
  })
  if (!updated) row.error = '这条曲线已存在。'
}

function changeIncludeSecondary(row: TrendCurveRow, includeSecondary: boolean): void {
  const updated = dashboard.updateCurve(row.id, { ...row.condition, includeSecondary })
  if (!updated) row.error = '这条曲线已存在。'
}

function applyCurve(row: TrendCurveRow): void {
  if (!isCurveComplete(row)) {
    row.error = '请先选择完整条件。'
    return
  }
  row.error = ''
  void dashboard.applyCurve(row.id)
}

function refreshAll(): void {
  void dashboard.refresh()
}

function selectRange(range: TrendRange): void {
  void dashboard.setRange(range)
}

function removeCurve(row: TrendCurveRow): void {
  dashboard.removeCurve(row.id)
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

function chartPointClicked(event: { data?: unknown }): void {
  const data = event.data
  if (!Array.isArray(data) || typeof data[0] !== 'string') return
  pointFeedback.value = `已聚焦 ${data[0]} 的数据点。`
  if (pointFeedbackTimer !== undefined) clearTimeout(pointFeedbackTimer)
  pointFeedbackTimer = setTimeout(() => {
    pointFeedback.value = ''
  }, 1600)
}

onMounted(async () => {
  void dashboard.activate()
  try {
    filterOptions.value = await getFilterOptions()
  } catch {
    filterOptionsError.value = '无法读取可选条件，请稍后重试。'
  }
})
</script>

<template>
  <main class="job-trend-view">
    <section class="job-trend-view__chart-card" aria-labelledby="trend-dashboard-title">
      <header class="job-trend-view__header">
        <div>
          <p class="job-trend-view__eyebrow">岗位趋势</p>
          <h1 id="trend-dashboard-title">岗位需求变化趋势</h1>
          <p>每个点是当日快照中匹配的独立岗位数。</p>
        </div>
        <div class="job-trend-view__header-actions">
          <NButton
            tertiary
            size="small"
            :disabled="dashboard.rows.length === 0"
            @click="refreshAll"
          >
            刷新数据
          </NButton>
        </div>
      </header>

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
      </nav>

      <div
        class="job-trend-view__chart-region"
        data-testid="trend-chart-region"
        :aria-busy="isRefreshing"
      >
        <VChart
          v-if="hasRenderedSeries"
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
          <NEmpty description="添加一条曲线，开始查看岗位趋势。">
            <template #extra>
              <NButton type="primary" @click="addCurve">添加曲线</NButton>
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
      <p v-if="pointFeedback" class="job-trend-view__feedback" role="status">{{ pointFeedback }}</p>
    </section>

    <div class="job-trend-view__mobile-panel">
      <NButton
        data-testid="open-trend-configuration"
        type="primary"
        @click="mobileConfigurationOpen = true"
      >
        配置曲线<span v-if="dashboard.rows.length">（{{ dashboard.rows.length }}）</span>
      </NButton>
    </div>

    <aside
      :class="[
        'job-trend-view__rail',
        { 'job-trend-view__rail--mobile-open': mobileConfigurationOpen },
      ]"
      data-testid="trend-configuration-rail"
      aria-label="曲线配置"
    >
      <header class="job-trend-view__rail-header">
        <div>
          <p>曲线配置</p>
          <small>每条曲线只统计一个条件</small>
        </div>
        <button
          class="job-trend-view__rail-close"
          type="button"
          aria-label="关闭曲线配置"
          @click="mobileConfigurationOpen = false"
        >
          ×
        </button>
      </header>

      <p v-if="filterOptionsError" class="job-trend-view__options-error" role="alert">
        {{ filterOptionsError }}
      </p>

      <ol v-if="dashboard.rows.length" class="job-trend-view__curve-list">
        <li v-for="(row, index) in dashboard.rows" :key="row.id" class="job-trend-view__curve-row">
          <header>
            <span class="job-trend-view__curve-index">{{
              String(index + 1).padStart(2, '0')
            }}</span>
            <strong>{{ trendCurveLabel(row) || '未设置曲线' }}</strong>
            <button type="button" aria-label="删除曲线" @click="removeCurve(row)">×</button>
          </header>

          <label>
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
            <label>
              <span>比较方式</span>
              <NSelect
                :value="row.condition.operator || null"
                :options="comparisonOptions"
                placeholder="选择"
                size="small"
                @update:value="changeCurveOperator(row, $event)"
              />
            </label>
            <label>
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

          <label v-else>
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

      <div class="job-trend-view__rail-footer">
        <span>{{ dashboard.rows.length }}/8 条</span>
        <NButton size="small" :disabled="curveLimitReached" @click="addCurve">+ 添加曲线</NButton>
      </div>
    </aside>
  </main>
</template>

<style scoped>
.job-trend-view {
  min-height: 100vh;
  padding: 28px 420px 32px 32px;
}
.job-trend-view__chart-card {
  background: rgba(255, 255, 255, 0.96);
  border: 1px solid #dce8ed;
  border-radius: 20px;
  box-shadow: 0 20px 44px -38px rgba(17, 73, 96, 0.56);
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
  color: #14313d;
  font-size: clamp(1.35rem, 2.2vw, 1.82rem);
  letter-spacing: -0.045em;
  line-height: 1.2;
  margin: 0;
}
.job-trend-view__header p:not(.job-trend-view__eyebrow) {
  color: #687981;
  font-size: 0.82rem;
  margin: 8px 0 0;
}
.job-trend-view__ranges {
  border-bottom: 1px solid #e6eef1;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 0 34px 18px;
}
.job-trend-view__chart-region {
  min-height: 540px;
  padding: 10px 16px 20px;
  position: relative;
}
.job-trend-view__chart {
  height: 540px;
  width: 100%;
}
.job-trend-view__empty {
  align-items: center;
  color: #62767f;
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
.job-trend-view__feedback {
  color: #00716f;
  font-size: 0.78rem;
  margin: 0 34px 12px;
}
.job-trend-view__rail {
  background: rgba(255, 255, 255, 0.98);
  border-left: 1px solid #dce8ed;
  bottom: 0;
  box-shadow: -18px 0 42px -38px rgba(11, 61, 82, 0.54);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 26px 22px 18px;
  position: fixed;
  right: 0;
  top: 0;
  width: 368px;
  z-index: 8;
}
.job-trend-view__rail-header {
  align-items: flex-start;
  display: flex;
  justify-content: space-between;
  padding: 0 4px 17px;
}
.job-trend-view__rail-header p {
  color: #18333f;
  font-size: 1rem;
  font-weight: 740;
  letter-spacing: -0.02em;
  margin: 0;
}
.job-trend-view__rail-header small {
  color: #7b8b92;
  display: block;
  font-size: 0.72rem;
  margin-top: 4px;
}
.job-trend-view__rail-close {
  background: transparent;
  border: 0;
  color: #52717f;
  cursor: pointer;
  display: none;
  font-size: 1.55rem;
  line-height: 1;
}
.job-trend-view__options-error,
.job-trend-view__curve-error {
  color: #a8424d;
  font-size: 0.74rem;
  line-height: 1.45;
  margin: 0 2px 12px;
}
.job-trend-view__curve-list {
  display: grid;
  gap: 12px;
  list-style: none;
  margin: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 0 2px 12px;
}
.job-trend-view__curve-row {
  background: #f8fcfd;
  border: 1px solid #e1ebef;
  border-radius: 13px;
  display: grid;
  gap: 10px;
  padding: 13px;
}
.job-trend-view__curve-row > header {
  align-items: center;
  display: flex;
  gap: 8px;
  min-width: 0;
}
.job-trend-view__curve-row > header strong {
  color: #27424d;
  flex: 1;
  font-size: 0.76rem;
  line-height: 1.35;
  min-width: 0;
}
.job-trend-view__curve-row > header button {
  background: transparent;
  border: 0;
  color: #84959c;
  cursor: pointer;
  font-size: 1.2rem;
  line-height: 1;
  padding: 2px;
}
.job-trend-view__curve-index {
  color: #008287;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.68rem;
  font-weight: 760;
}
.job-trend-view__curve-row label {
  color: #687981;
  display: grid;
  font-size: 0.69rem;
  font-weight: 700;
  gap: 5px;
}
.job-trend-view__curve-pair {
  display: grid;
  gap: 9px;
  grid-template-columns: 1fr 1fr;
}
.job-trend-view__secondary-switch {
  align-items: center;
  display: flex !important;
  justify-content: space-between;
}
.job-trend-view__curve-actions,
.job-trend-view__rail-footer {
  align-items: center;
  display: flex;
  justify-content: space-between;
}
.job-trend-view__curve-actions {
  border-top: 1px solid #e6eef1;
  margin-top: 1px;
  padding-top: 10px;
}
.job-trend-view__curve-state {
  color: #778a92;
  font-size: 0.71rem;
}
.job-trend-view__curve-state--loading {
  color: #007cab;
}
.job-trend-view__curve-state--error {
  color: #a8424d;
}
.job-trend-view__rail-footer {
  border-top: 1px solid #e2ecf0;
  color: #71828a;
  font-size: 0.72rem;
  margin-top: auto;
  padding: 14px 4px 0;
}
.job-trend-view__mobile-panel {
  display: none;
}
:global(.app-shell[data-color-scheme='dark'] .job-trend-view__chart-card) {
  background: rgba(22, 47, 58, 0.96);
  border-color: #2b4c58;
  box-shadow: 0 20px 44px -38px rgba(0, 0, 0, 0.86);
}
:global(.app-shell[data-color-scheme='dark'] .job-trend-view__header h1),
:global(.app-shell[data-color-scheme='dark'] .job-trend-view__rail-header p),
:global(.app-shell[data-color-scheme='dark'] .job-trend-view__curve-row > header strong) {
  color: #edf7f8;
}
:global(
  .app-shell[data-color-scheme='dark'] .job-trend-view__header p:not(.job-trend-view__eyebrow)
),
:global(.app-shell[data-color-scheme='dark'] .job-trend-view__empty) {
  color: #a8c3ca;
}
:global(.app-shell[data-color-scheme='dark'] .job-trend-view__ranges),
:global(.app-shell[data-color-scheme='dark'] .job-trend-view__curve-actions) {
  border-color: #294752;
}
:global(.app-shell[data-color-scheme='dark'] .job-trend-view__rail) {
  background: rgba(16, 37, 46, 0.98);
  border-color: #294752;
  box-shadow: -18px 0 42px -38px rgba(0, 0, 0, 0.86);
}
:global(.app-shell[data-color-scheme='dark'] .job-trend-view__curve-row) {
  background: #18313b;
  border-color: #294752;
}
@media (max-width: 1120px) {
  .job-trend-view {
    padding-right: 392px;
  }
  .job-trend-view__rail {
    width: 340px;
  }
}
@media (max-width: 760px) {
  .job-trend-view {
    min-height: auto;
    padding: 64px 16px 24px;
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
    padding: 0 22px 14px;
  }
  .job-trend-view__chart-region {
    min-height: 430px;
    padding: 4px 4px 12px;
  }
  .job-trend-view__chart,
  .job-trend-view__empty {
    height: 430px;
  }
  .job-trend-view__mobile-panel {
    bottom: 18px;
    display: block;
    left: 50%;
    position: fixed;
    transform: translateX(-50%);
    z-index: 11;
  }
  .job-trend-view__rail {
    border: 1px solid #dce8ed;
    border-radius: 18px 18px 0 0;
    bottom: 0;
    display: none;
    left: 0;
    max-height: min(82dvh, 720px);
    padding: 22px 16px 14px;
    right: 0;
    top: auto;
    width: auto;
    z-index: 20;
  }
  .job-trend-view__rail--mobile-open {
    display: flex;
  }
  .job-trend-view__rail-close {
    display: block;
  }
  .job-trend-view__curve-list {
    max-height: 56dvh;
  }
}
</style>
