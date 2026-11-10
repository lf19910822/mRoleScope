<script setup lang="ts">
import { use } from 'echarts/core'
import type { EChartsOption } from 'echarts'
import { LineChart } from 'echarts/charts'
import {
  DataZoomComponent,
  GridComponent,
  LegendComponent,
  TooltipComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import VChart, { type Exposed as VChartExposed } from 'vue-echarts'

import {
  getJobHistory,
  getJobHistoryDetail,
  type HistoryNode,
  type HistoryNodeType,
  type JobDetailResponse,
} from '@/api/jd-api'
import JDDetailReadingLayout from '@/components/JDDetailReadingLayout.vue'
import {
  historyNodeChartColors,
  historyNodeChartLabel,
  historyNodeChartSymbols,
  historyNodeTagLabel,
  historyNodeTypeLabel,
  readHistoryCache,
  writeHistoryCache,
} from '@/lib/job-history-timeline'

use([
  CanvasRenderer,
  LineChart,
  DataZoomComponent,
  GridComponent,
  LegendComponent,
  TooltipComponent,
])

const props = defineProps<{ jobId: string }>()

const history = ref<{ nodes: HistoryNode[] }>()
const error = ref('')
const isLoading = ref(false)
const selectedNode = ref<HistoryNode>()
const selectedDetail = ref<JobDetailResponse>()
const detailError = ref('')
const isDetailLoading = ref(false)
const historyChart = ref<VChartExposed | null>(null)
const chartRegion = ref<HTMLElement>()
let historyRequest = 0
let detailRequest = 0
let bodyScrollLocked = false
let bodyOverflowBeforeLock = ''
let bodyPaddingRightBeforeLock = ''

function setBodyScrollLock(locked: boolean): void {
  if (locked && !bodyScrollLocked) {
    bodyScrollLocked = true
    bodyOverflowBeforeLock = document.body.style.overflow
    bodyPaddingRightBeforeLock = document.body.style.paddingRight

    const scrollbarWidth = window.innerWidth - document.documentElement.clientWidth
    document.body.style.overflow = 'hidden'
    document.body.style.paddingRight =
      scrollbarWidth > 0 ? `${scrollbarWidth}px` : bodyPaddingRightBeforeLock
    return
  }

  if (!locked && bodyScrollLocked) {
    bodyScrollLocked = false
    document.body.style.overflow = bodyOverflowBeforeLock
    document.body.style.paddingRight = bodyPaddingRightBeforeLock
  }
}

const historyNodeTypes: HistoryNodeType[] = [
  'discovered',
  'modified',
  'confirmed_closed',
  'reopened',
  'latest',
]

function formatHistoryAxisDate(value: string | number): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value).slice(5).replace('-', '/')
  return `${date.getMonth() + 1}/${date.getDate()}`
}

async function loadHistory(force = false): Promise<void> {
  const request = ++historyRequest
  const cached = force ? undefined : readHistoryCache(props.jobId)
  if (cached) {
    history.value = cached
    error.value = ''
    return
  }
  isLoading.value = true
  error.value = ''
  try {
    const response = await getJobHistory(props.jobId)
    if (request !== historyRequest) return
    const safeResponse = {
      nodes: Array.isArray(response?.nodes) ? response.nodes : [],
    }
    writeHistoryCache(props.jobId, safeResponse)
    history.value = safeResponse
  } catch (reason) {
    if (request !== historyRequest) return
    error.value = reason instanceof Error ? reason.message : '岗位历史暂不可用'
  } finally {
    if (request === historyRequest) isLoading.value = false
  }
}

async function openNode(node: HistoryNode): Promise<void> {
  if (!node.can_view_historical) return
  selectedNode.value = node
  selectedDetail.value = undefined
  detailError.value = ''
  const request = ++detailRequest
  isDetailLoading.value = true
  try {
    const response = await getJobHistoryDetail(props.jobId, node.date, node.type)
    if (request !== detailRequest) return
    selectedDetail.value = response
    await nextTick()
    document.querySelector<HTMLElement>('.job-history__detail-close')?.focus()
  } catch (reason) {
    if (request !== detailRequest) return
    detailError.value = reason instanceof Error ? reason.message : '岗位历史详情暂不可用'
  } finally {
    if (request === detailRequest) isDetailLoading.value = false
  }
}

function closeDetail(): void {
  selectedNode.value = undefined
  selectedDetail.value = undefined
  detailError.value = ''
  void nextTick(() => chartRegion.value?.focus())
}

function retryDetail(): void {
  if (selectedNode.value) void openNode(selectedNode.value)
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && selectedNode.value) {
    event.preventDefault()
    closeDetail()
  }
}

function resetZoom(): void {
  historyChart.value?.dispatchAction({
    dataZoomIndex: 0,
    end: 100,
    start: 0,
    type: 'dataZoom',
  })
}

function visibleTags(node: HistoryNode): HistoryNode['tags'] {
  return node.tags.filter((tag) => tag !== node.type)
}

const historyChartOption = computed(() => {
  const nodes = history.value?.nodes ?? []
  const dates = nodes.map((node) => [node.date, 0.5] as [string, number])
  const statusTypes = historyNodeTypes.filter((type) => nodes.some((node) => node.type === type))

  return {
    animationDuration: 300,
    animationDurationUpdate: 240,
    dataZoom: [
      {
        filterMode: 'none',
        moveOnMouseMove: true,
        moveOnMouseWheel: true,
        type: 'inside',
        xAxisIndex: 0,
        zoomOnMouseWheel: true,
      },
    ],
    grid: { bottom: 58, left: 18, right: 18, top: 22 },
    legend: {
      bottom: 5,
      data: statusTypes.map(historyNodeChartLabel),
      left: 18,
      right: 18,
      type: 'scroll',
    },
    series: [
      {
        connectNulls: false,
        data: dates,
        itemStyle: { color: '#c8dce3' },
        lineStyle: { color: '#c8dce3', width: 2 },
        name: '观察轨道',
        showSymbol: false,
        silent: true,
        smooth: 0.18,
        type: 'line',
      },
      ...statusTypes.map((type) => ({
        connectNulls: false,
        data: nodes
          .filter((node) => node.type === type)
          .map((node) => [node.date, 0.5] as [string, number]),
        emphasis: {
          itemStyle: { borderColor: '#ffffff', borderWidth: 3 },
          scale: 1.45,
        },
        itemStyle: { color: historyNodeChartColors[type] },
        lineStyle: { color: historyNodeChartColors[type], width: 0, opacity: 0 },
        name: historyNodeChartLabel(type),
        showSymbol: true,
        symbol: historyNodeChartSymbols[type],
        symbolSize: type === 'latest' ? 16 : 13,
        type: 'line',
      })),
    ],
    tooltip: {
      formatter: (params: unknown) => {
        const item = Array.isArray(params) ? params[0] : params
        if (typeof item !== 'object' || item === null) return ''
        const value = (item as { value?: unknown }).value
        const date = Array.isArray(value) && typeof value[0] === 'string' ? value[0] : ''
        const node = nodes.find((candidate) => candidate.date === date)
        return node === undefined ? date : `${date}<br/>${historyNodeChartLabel(node.type)}`
      },
      trigger: 'item',
    },
    xAxis: {
      axisLabel: {
        formatter: formatHistoryAxisDate,
        hideOverlap: true,
      },
      boundaryGap: false,
      type: 'time',
    },
    yAxis: {
      max: 1,
      min: 0,
      show: false,
      type: 'value',
    },
  } as unknown as EChartsOption
})

function onChartPointClicked(event: unknown): void {
  if (typeof event !== 'object' || event === null) return
  const candidate = event as { data?: unknown; seriesName?: unknown }
  const data = candidate.data
  if (!Array.isArray(data) || typeof data[0] !== 'string') return
  if (typeof candidate.seriesName !== 'string' || candidate.seriesName === '观察轨道') return
  const node = history.value?.nodes.find((item) => item.date === data[0])
  if (node === undefined || !node.can_view_historical) return
  void openNode(node)
}

watch(
  () => props.jobId,
  () => {
    selectedNode.value = undefined
    selectedDetail.value = undefined
    void loadHistory()
  },
  { immediate: true },
)

watch(selectedNode, (node) => {
  setBodyScrollLock(node !== undefined)
})

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => {
  historyRequest += 1
  detailRequest += 1
  setBodyScrollLock(false)
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <section
    id="job-detail-reading-history"
    class="job-history"
    aria-labelledby="job-history-heading"
  >
    <header class="job-history__header">
      <div>
        <p class="job-history__eyebrow">岗位历史</p>
        <h2 id="job-history-heading">观察时间线</h2>
        <p class="job-history__hint">按每日最终成功观察还原，缺失日期不生成虚假事件。</p>
      </div>
      <div class="job-history__tools">
        <span v-if="history" class="job-history__count">{{ history.nodes.length }} 个观测点</span>
        <button class="job-history__refresh" type="button" @click="loadHistory(true)">
          刷新历史
        </button>
        <button class="job-history__zoom" type="button" @click="resetZoom">复位视图</button>
      </div>
    </header>

    <div v-if="isLoading && !history" class="job-history__state" role="status">
      正在读取岗位历史…
    </div>
    <div
      v-else-if="error && !history"
      class="job-history__state job-history__state--error"
      role="alert"
    >
      <span>{{ error }}</span>
      <button type="button" @click="loadHistory(true)">重试</button>
    </div>
    <div v-else-if="history && history.nodes.length === 0" class="job-history__state">
      暂无可展示的岗位历史。
    </div>
    <template v-else-if="history">
      <div
        ref="chartRegion"
        class="job-history__chart-region"
        tabindex="0"
        aria-label="岗位历史时间线，可缩放和平移"
        :aria-busy="isLoading"
      >
        <VChart
          ref="historyChart"
          class="job-history__chart"
          :option="historyChartOption"
          :autoresize="{ throttle: 120 }"
          @click="onChartPointClicked"
        />
      </div>
      <p v-if="error" class="job-history__cached-error" role="status">
        历史刷新失败，当前显示缓存内容。<button type="button" @click="loadHistory(true)">
          重试
        </button>
      </p>
      <p class="job-history__gesture">滚轮缩放时间 · 拖拽平移 · 点击节点查看当日岗位</p>
    </template>

    <Teleport to="body">
      <div v-if="selectedNode" class="job-history__detail-backdrop" @click="closeDetail" />
      <Transition name="job-history-detail">
        <section
          v-if="selectedNode"
          class="job-history__detail-drawer"
          role="dialog"
          aria-modal="true"
          aria-labelledby="job-history-detail-heading"
        >
          <header class="job-history__detail-header">
            <div>
              <p class="job-history__eyebrow">岗位历史 · {{ selectedNode.date }}</p>
              <h3 id="job-history-detail-heading">{{ historyNodeTypeLabel(selectedNode.type) }}</h3>
              <div
                v-if="visibleTags(selectedNode).length"
                class="job-history__detail-tags"
                aria-label="本次观察变动"
              >
                <span v-for="tag in visibleTags(selectedNode)" :key="tag">{{
                  historyNodeTagLabel(tag)
                }}</span>
              </div>
            </div>
            <button
              class="job-history__detail-close"
              type="button"
              aria-label="关闭岗位历史详情"
              @click="closeDetail"
            >
              ×
            </button>
          </header>
          <div class="job-history__detail-content">
            <div v-if="isDetailLoading" class="job-history__state" role="status">
              正在读取节点详情…
            </div>
            <div
              v-else-if="detailError"
              class="job-history__state job-history__state--error"
              role="alert"
            >
              <span>{{ detailError }}</span>
              <button type="button" @click="retryDetail">重试</button>
            </div>
            <JDDetailReadingLayout
              v-else-if="selectedDetail"
              :job="selectedDetail"
              @retry-comparison="retryDetail"
            />
          </div>
        </section>
      </Transition>
    </Teleport>
  </section>
</template>

<style scoped>
.job-history {
  --history-ink: #14313d;
  --history-muted: #687981;
  --history-subtle: #71828a;
  --history-rule: #dce8ed;
  --history-rule-soft: #e8f0f3;
  --history-paper: rgba(255, 255, 255, 0.96);
  --history-drawer: #ffffff;
  --history-node-hover: #f5fafb;
  --history-blue: #0072b2;
  --history-green: #009e73;
  --history-red: #d55e00;
  --history-orange: #e69f00;
  --history-lilac: #332288;
  background: var(--history-paper);
  border: 1px solid var(--history-rule);
  border-top: 3px solid var(--history-blue);
  border-radius: 12px;
  box-shadow: 0 20px 44px -38px rgba(17, 73, 96, 0.56);
  margin-top: 24px;
  overflow: hidden;
  padding: 17px 20px 20px;
  scroll-margin-top: 24px;
}

.job-history__header,
.job-history__detail-header {
  align-items: flex-start;
  display: flex;
  gap: 18px;
  justify-content: space-between;
}

.job-history__eyebrow {
  color: var(--history-blue);
  font-size: 0.68rem;
  font-weight: 800;
  letter-spacing: 0.12em;
  margin: 0 0 3px;
  text-transform: uppercase;
}

.job-history h2,
.job-history h3 {
  color: var(--history-ink);
  margin: 0;
}

.job-history h2 {
  font-size: 1.12rem;
  letter-spacing: -0.01em;
}

.job-history h3 {
  font-size: 1rem;
}

.job-history__hint,
.job-history__gesture,
.job-history__cached-error {
  color: var(--history-muted);
  font-size: 0.74rem;
  line-height: 1.45;
  margin: 5px 0 0;
}

.job-history__tools {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
  padding-top: 1px;
}

.job-history__count {
  color: var(--history-subtle);
  font-size: 0.72rem;
  font-variant-numeric: tabular-nums;
  margin-right: 2px;
  white-space: nowrap;
}

.job-history__tools button,
.job-history__state button,
.job-history__cached-error button,
.job-history__detail-close {
  background: transparent;
  border: 1px solid var(--history-rule);
  border-radius: 7px;
  color: var(--history-subtle);
  cursor: pointer;
  font-size: 0.72rem;
  font-weight: 750;
  min-height: 30px;
  padding: 5px 9px;
  transition:
    background 160ms ease,
    border-color 160ms ease,
    color 160ms ease;
}

.job-history__tools button:hover,
.job-history__state button:hover,
.job-history__cached-error button:hover,
.job-history__detail-close:hover {
  background: var(--history-node-hover);
  border-color: color-mix(in srgb, var(--history-blue), var(--history-rule) 55%);
  color: var(--history-blue);
}

.job-history button:focus-visible,
.job-history__chart-region:focus-visible {
  outline: 3px solid color-mix(in srgb, var(--history-blue), transparent 70%);
  outline-offset: 2px;
}

.job-history__chart-region {
  border-bottom: 1px solid var(--history-rule-soft);
  border-top: 1px solid var(--history-rule-soft);
  margin: 20px -4px 0;
  min-height: 250px;
  padding: 7px 4px 0;
  position: relative;
}

.job-history__chart {
  height: 242px;
  width: 100%;
}

.job-history__state {
  align-items: center;
  color: var(--history-muted);
  display: flex;
  gap: 10px;
  justify-content: center;
  min-height: 92px;
}

.job-history__state--error {
  color: #a13f4b;
}

.job-history__cached-error {
  align-items: center;
  display: flex;
  gap: 8px;
}

.job-history__detail-backdrop {
  background: rgba(17, 42, 52, 0.2);
  inset: 0;
  position: fixed;
  z-index: 40;
}

.job-history__detail-drawer {
  --history-drawer: #ffffff;
  background: var(--history-drawer);
  border: 1px solid
    color-mix(in srgb, var(--history-rule, #dce8ed), var(--history-blue, #0072b2) 22%);
  border-radius: 18px;
  box-shadow: 0 -22px 60px -28px rgba(5, 38, 50, 0.62);
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  height: min(78dvh, 780px);
  inset: auto 24px 24px 260px;
  min-height: 0;
  overflow: hidden;
  padding: 20px 22px;
  position: fixed;
  z-index: 41;
}

.job-history__detail-header {
  align-items: center;
  border-bottom: 1px solid var(--history-rule-soft);
  flex: 0 0 auto;
  margin-bottom: 16px;
  padding-bottom: 12px;
}

.job-history__detail-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 8px;
}

.job-history__detail-tags span {
  background: var(--history-node-hover);
  border: 1px solid var(--history-rule);
  border-radius: 5px;
  color: var(--history-muted);
  font-size: 0.66rem;
  font-weight: 750;
  padding: 3px 6px;
}

.job-history__detail-close {
  border: 0;
  font-size: 1.35rem;
  line-height: 1;
  min-height: 30px;
  padding: 2px 4px;
}

.job-history__detail-content {
  flex: 1 1 auto;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  padding-right: 6px;
  scrollbar-gutter: stable;
  scrollbar-width: thin;
}

.job-history__detail-content::-webkit-scrollbar {
  width: 8px;
}

.job-history__detail-content::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--history-blue), transparent 48%);
  border: 2px solid transparent;
  border-radius: 999px;
  background-clip: padding-box;
}

.job-history-detail-enter-active,
.job-history-detail-leave-active {
  transition:
    opacity 180ms ease,
    transform 320ms cubic-bezier(0.22, 0.9, 0.25, 1);
}

.job-history-detail-enter-from,
.job-history-detail-leave-to {
  opacity: 0;
  transform: translateY(calc(100% + 28px));
}

@media (max-width: 700px) {
  .job-history {
    border-radius: 10px;
    margin-top: 18px;
    padding: 15px 14px 17px;
  }

  .job-history__header {
    display: block;
  }

  .job-history__tools {
    justify-content: flex-start;
    margin-top: 11px;
  }

  .job-history__count {
    margin-right: auto;
  }

  .job-history__chart-region {
    margin-top: 16px;
    min-height: 220px;
  }

  .job-history__chart {
    height: 212px;
  }

  .job-history__detail-header {
    align-items: flex-start;
  }

  .job-history__detail-drawer {
    border-radius: 18px 18px 0 0;
    bottom: 0;
    height: min(86dvh, 760px);
    inset-inline: 0;
    padding: 18px 14px calc(18px + env(safe-area-inset-bottom));
  }
}

:global(.app-shell[data-color-scheme='dark']) .job-history {
  --history-ink: #edf7f8;
  --history-muted: #a8c3ca;
  --history-subtle: #b7ccd2;
  --history-rule: #2b4c58;
  --history-rule-soft: #294752;
  --history-paper: rgba(22, 47, 58, 0.96);
  --history-drawer: #162f3a;
  --history-node-hover: #1d414c;
  --history-blue: #56b4e9;
  --history-green: #009e73;
  --history-red: #d55e00;
  --history-orange: #e69f00;
  --history-lilac: #cc79a7;
  box-shadow: 0 20px 44px -38px rgba(0, 0, 0, 0.86);
}

:global(body:has(.app-shell[data-color-scheme='dark']) .job-history__detail-drawer) {
  --history-drawer: #162f3a;
}
</style>
