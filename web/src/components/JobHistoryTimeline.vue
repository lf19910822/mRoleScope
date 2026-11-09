<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import {
  getJobHistory,
  getJobHistoryDetail,
  type HistoryNode,
  type JobDetailResponse,
} from '@/api/jd-api'
import JDDetailReadingLayout from '@/components/JDDetailReadingLayout.vue'
import {
  historyNodeAccessibleLabel,
  historyNodeTagLabel,
  historyNodeTypeLabel,
  historyTimelineMinWidth,
  readHistoryCache,
  writeHistoryCache,
} from '@/lib/job-history-timeline'

const props = defineProps<{ jobId: string }>()

const history = ref<{ nodes: HistoryNode[] }>()
const error = ref('')
const isLoading = ref(false)
const zoom = ref(1)
const selectedNode = ref<HistoryNode>()
const selectedDetail = ref<JobDetailResponse>()
const detailError = ref('')
const isDetailLoading = ref(false)
const nodeButtons = new Map<string, HTMLButtonElement>()
let historyRequest = 0
let detailRequest = 0

function setNodeButton(date: string, element: Element | null): void {
  if (element instanceof HTMLButtonElement) nodeButtons.set(date, element)
  else nodeButtons.delete(date)
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
  const date = selectedNode.value?.date
  selectedNode.value = undefined
  selectedDetail.value = undefined
  detailError.value = ''
  if (date) nextTick(() => nodeButtons.get(date)?.focus())
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
  zoom.value = 1
}

function onWheel(event: WheelEvent): void {
  if (!event.ctrlKey && !event.metaKey) return
  event.preventDefault()
  zoom.value = Math.min(2.25, Math.max(0.8, zoom.value + (event.deltaY < 0 ? 0.1 : -0.1)))
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

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => {
  historyRequest += 1
  detailRequest += 1
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <section class="job-history" aria-labelledby="job-history-heading">
    <header class="job-history__header">
      <div>
        <p class="job-history__eyebrow">岗位历史</p>
        <h2 id="job-history-heading">观察时间线</h2>
        <p class="job-history__hint">按每日最终成功观察还原，缺失日期不生成虚假事件。</p>
      </div>
      <div class="job-history__tools">
        <button class="job-history__refresh" type="button" @click="loadHistory(true)">
          刷新历史
        </button>
        <button class="job-history__zoom" type="button" @click="resetZoom">复位视图</button>
      </div>
    </header>

    <div v-if="isLoading && !history" class="job-history__state" role="status">
      正在读取岗位历史…
    </div>
    <div v-else-if="error && !history" class="job-history__state job-history__state--error" role="alert">
      <span>{{ error }}</span>
      <button type="button" @click="loadHistory(true)">重试</button>
    </div>
    <div v-else-if="history && history.nodes.length === 0" class="job-history__state">
      暂无可展示的岗位历史。
    </div>
    <template v-else-if="history">
      <div
        class="job-history__scroll"
        tabindex="0"
        aria-label="岗位历史时间线，可横向滚动"
        @wheel="onWheel"
      >
        <div
          class="job-history__track"
          :style="{ minWidth: historyTimelineMinWidth(history.nodes.length, zoom) }"
        >
          <div
            v-for="(node, index) in history.nodes"
            :key="node.date"
            class="job-history__event"
            :class="`job-history__event--${node.type}`"
          >
            <span
              v-if="index > 0"
              class="job-history__connector"
              :class="{ 'job-history__connector--disconnected': !node.connect_previous }"
              aria-hidden="true"
            />
            <button
              :ref="(element) => setNodeButton(node.date, element as Element | null)"
              class="job-history__node"
              :class="{ 'job-history__node--disabled': !node.can_view_historical }"
              type="button"
              :disabled="!node.can_view_historical"
              :aria-label="historyNodeAccessibleLabel(node)"
              @click="openNode(node)"
            >
              <span class="job-history__node-shape" aria-hidden="true" />
              <span class="job-history__node-date">{{ node.date }}</span>
              <strong>{{ historyNodeTypeLabel(node.type) }}</strong>
              <span v-if="node.summary" class="job-history__node-summary">{{ node.summary }}</span>
              <span v-if="!node.can_view_historical" class="job-history__node-unavailable">暂不可查看</span>
              <span v-if="node.tags.length" class="job-history__tags" aria-label="变动标签">
                <span v-for="tag in node.tags" :key="tag">{{ historyNodeTagLabel(tag) }}</span>
              </span>
            </button>
          </div>
        </div>
      </div>
      <p v-if="error" class="job-history__cached-error" role="status">
        历史刷新失败，当前显示缓存内容。<button type="button" @click="loadHistory(true)">重试</button>
      </p>
      <p class="job-history__gesture">拖动时间线查看较早记录 · Ctrl/⌘ + 滚轮缩放 · {{ zoom.toFixed(1) }}×</p>
    </template>

    <div v-if="selectedNode" class="job-history__detail" role="dialog" aria-modal="false" aria-labelledby="job-history-detail-heading">
      <header class="job-history__detail-header">
        <div>
          <p class="job-history__eyebrow">{{ selectedNode.date }}</p>
          <h3 id="job-history-detail-heading">{{ historyNodeTypeLabel(selectedNode.type) }}</h3>
        </div>
        <button class="job-history__detail-close" type="button" aria-label="关闭岗位历史详情" @click="closeDetail">关闭</button>
      </header>
      <div v-if="isDetailLoading" class="job-history__state" role="status">正在读取节点详情…</div>
      <div v-else-if="detailError" class="job-history__state job-history__state--error" role="alert">
        <span>{{ detailError }}</span>
        <button type="button" @click="retryDetail">重试</button>
      </div>
      <JDDetailReadingLayout v-else-if="selectedDetail" :job="selectedDetail" @retry-comparison="retryDetail" />
    </div>
  </section>
</template>

<style scoped>
.job-history {
  --history-ink: #152b36;
  --history-muted: #5c7680;
  --history-rule: #c5dde5;
  --history-paper: rgba(255, 255, 255, 0.78);
  --history-blue: #007bab;
  --history-green: #008b72;
  --history-red: #ad4654;
  --history-orange: #c7782d;
  --history-lilac: #6c63a8;
  background: var(--history-paper);
  border: 1px solid var(--history-rule);
  border-radius: 18px;
  margin-top: 28px;
  overflow: hidden;
  padding: 20px 22px 24px;
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
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.14em;
  margin: 0 0 3px;
  text-transform: uppercase;
}

.job-history h2,
.job-history h3 {
  color: var(--history-ink);
  margin: 0;
}

.job-history h2 {
  font-size: 1.16rem;
}

.job-history h3 {
  font-size: 1.02rem;
}

.job-history__hint,
.job-history__gesture,
.job-history__cached-error {
  color: var(--history-muted);
  font-size: 0.78rem;
  margin: 5px 0 0;
}

.job-history__tools {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.job-history__tools button,
.job-history__state button,
.job-history__cached-error button,
.job-history__detail-close {
  background: transparent;
  border: 1px solid var(--history-rule);
  border-radius: 999px;
  color: var(--history-blue);
  cursor: pointer;
  font-size: 0.76rem;
  font-weight: 750;
  padding: 6px 10px;
}

.job-history__scroll {
  margin: 24px -4px 0;
  overflow-x: auto;
  padding: 8px 4px 12px;
  scrollbar-color: #96bdc9 transparent;
}

.job-history__track {
  align-items: flex-start;
  display: flex;
  min-height: 152px;
  padding: 16px 8px 0;
  position: relative;
}

.job-history__track::before {
  background: var(--history-rule);
  content: '';
  height: 2px;
  left: 22px;
  position: absolute;
  right: 22px;
  top: 39px;
}

.job-history__event {
  flex: 1 0 148px;
  min-width: 148px;
  position: relative;
  text-align: center;
}

.job-history__connector {
  background: currentColor;
  height: 3px;
  left: -50%;
  opacity: 0.35;
  position: absolute;
  right: 50%;
  top: 38px;
}

.job-history__connector--disconnected {
  background: repeating-linear-gradient(90deg, currentColor 0 5px, transparent 5px 10px);
  opacity: 0.55;
}

.job-history__node {
  align-items: center;
  background: transparent;
  border: 0;
  color: var(--history-ink);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 3px;
  margin: 0 auto;
  max-width: 145px;
  padding: 0 6px;
  position: relative;
  text-align: center;
  z-index: 1;
}

.job-history__node:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.job-history__node-shape {
  background: currentColor;
  border: 4px solid var(--history-paper);
  border-radius: 50%;
  box-shadow: 0 0 0 2px currentColor;
  display: block;
  height: 18px;
  margin-bottom: 10px;
  width: 18px;
}

.job-history__event--modified .job-history__node-shape,
.job-history__event--reopened .job-history__node-shape {
  border-radius: 5px;
  transform: rotate(45deg);
}

.job-history__event--confirmed_closed .job-history__node-shape {
  border-radius: 4px;
}

.job-history__event--discovered,
.job-history__event--latest {
  color: var(--history-blue);
}

.job-history__event--modified {
  color: var(--history-orange);
}

.job-history__event--confirmed_closed {
  color: var(--history-red);
}

.job-history__event--reopened {
  color: var(--history-green);
}

.job-history__node-date {
  color: var(--history-muted);
  font-size: 0.7rem;
  font-variant-numeric: tabular-nums;
}

.job-history__node strong {
  font-size: 0.82rem;
}

.job-history__node-summary {
  color: var(--history-muted);
  font-size: 0.7rem;
  line-height: 1.35;
}

.job-history__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 3px;
  justify-content: center;
  margin-top: 3px;
}

.job-history__tags span {
  background: color-mix(in srgb, currentColor 12%, transparent);
  border-radius: 999px;
  color: currentColor;
  font-size: 0.62rem;
  padding: 2px 5px;
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

.job-history__detail {
  animation: job-history-detail-in 220ms ease-out;
  border-top: 1px solid var(--history-rule);
  margin: 20px -22px -24px;
  padding: 20px 22px 24px;
}

.job-history__detail .detail-reading-layout {
  margin-top: 16px;
}

@keyframes job-history-detail-in {
  from {
    opacity: 0;
    transform: translateY(16px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 700px) {
  .job-history {
    border-radius: 14px;
    padding: 16px 14px 20px;
  }

  .job-history__header {
    display: block;
  }

  .job-history__tools {
    margin-top: 12px;
  }

  .job-history__detail {
    margin: 18px -14px -20px;
    padding: 16px 14px 20px;
  }
}

:global(.app-shell[data-color-scheme='dark']) .job-history {
  --history-ink: #e9f5f6;
  --history-muted: #a9c2c8;
  --history-rule: #31535d;
  --history-paper: rgba(18, 43, 52, 0.88);
}
</style>
