import { reactive } from 'vue'

import {
  streamAITrendDraft,
  type AITrendDraft,
  type AITrendDraftCurve,
  type AITrendStreamEvent,
} from '@/api/jd-api'
import type { TrendCondition } from '@/lib/job-trend-dashboard-state'

export interface AITrendOperation {
  tool: string
  params?: Record<string, unknown>
  status: 'running' | 'completed' | 'failed'
}

export interface AITrendDraftState {
  draft?: AITrendDraft
  durationLabel: string
  error: string
  operations: AITrendOperation[]
  operationsExpanded: boolean
  query: string
  status: 'idle' | 'running' | 'success' | 'partial' | 'failure' | 'stopped'
  stop(): void
  submit(query?: string): Promise<void>
  applySelected(apply: (condition: TrendCondition) => void): void
}

const mockAITrendEnabled = import.meta.env.DEV && import.meta.env.VITE_AI_TREND_MOCK !== 'false'

function formatElapsed(milliseconds: number): string {
  const seconds = Math.max(0, Math.floor(milliseconds / 1000))
  if (seconds < 60) return `${seconds}秒`
  const minutes = Math.floor(seconds / 60)
  const remainder = seconds % 60
  if (minutes < 60) return `${minutes}分${remainder}秒`
  return `${Math.floor(minutes / 60)}小时${minutes % 60}分`
}

function waitForMockStep(signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(resolve, 220)
    signal.addEventListener(
      'abort',
      () => {
        window.clearTimeout(timer)
        reject(new DOMException('分析已停止', 'AbortError'))
      },
      { once: true },
    )
  })
}

function mockCurve(
  dimension: AITrendDraftCurve['condition']['dimension'],
  value: string,
  label: string,
  matchStatus: AITrendDraftCurve['match_status'],
  subQueryID: string,
  reason: string,
): AITrendDraftCurve {
  return {
    condition: { dimension, value, includeSecondary: false, operator: '' },
    label,
    match_status: matchStatus,
    reason,
    selected: true,
    sub_query_ids: [subQueryID],
  }
}

function mockDraftForQuery(query: string): AITrendDraft {
  const curves = query.includes('项目')
    ? [
        mockCurve(
          'project',
          'game.genshin-impact',
          '原神',
          'exact',
          'projects',
          '按当前周期岗位平均数量排序。',
        ),
        mockCurve(
          'project',
          'game.zenless-zone-zero',
          '绝区零',
          'possible',
          'projects',
          '名称相近，保留供确认。',
        ),
        mockCurve(
          'project',
          'game.honkai-star-rail',
          '崩坏：星穹铁道',
          'exact',
          'projects',
          '当前正式项目值。',
        ),
      ]
    : [
        mockCurve(
          'technology',
          'redis',
          'Redis',
          'exact',
          'databases',
          '正式技术值，当前周期有岗位。',
        ),
        mockCurve(
          'technology',
          'mysql',
          'MySQL',
          'exact',
          'databases',
          '正式技术值，当前周期有岗位。',
        ),
        mockCurve(
          'technology',
          'postgresql',
          'PostgreSQL',
          'possible',
          'databases',
          '可能匹配，保留供确认。',
        ),
        mockCurve('technology', 'mongodb', 'MongoDB', 'alias', 'databases', '按正式别名匹配。'),
        mockCurve('job_family', 'backend', '后端', 'possible', 'roles', '组合问题中的岗位族候选。'),
      ]
  return {
    contract_version: 'ai-trend-draft-v1',
    status: 'complete',
    curves,
    explanation: '按当前周期岗位平均数量整理候选，曲线应用前仍可修改。',
    warnings: query.includes('项目')
      ? ['本地预览未读取当前周期岗位数量，应用时会按正式项目值重新校验。']
      : undefined,
  }
}

async function runMockAITrend(
  query: string,
  onEvent: (event: AITrendStreamEvent) => void,
  signal: AbortSignal,
): Promise<void> {
  const operations: Array<[string, Record<string, unknown>]> = query.includes('项目')
    ? [
        ['list_available_values', { dimension: 'project' }],
        ['rank_values', { dimension: 'project', metric: 'avg_daily_active_jobs' }],
        ['batch_trend_summary', { dimension: 'project', limit: 8 }],
      ]
    : [
        ['list_available_categories', {}],
        ['list_category_values', { category: 'databases-and-storage' }],
        ['rank_values', { dimension: 'technology', metric: 'avg_daily_active_jobs' }],
        ['batch_trend_summary', { dimension: 'technology', limit: 8 }],
      ]
  for (const [tool, params] of operations) {
    onEvent({ type: 'operation', status: 'running', tool, params })
    await waitForMockStep(signal)
    onEvent({ type: 'operation', status: 'completed', tool, params })
  }
  onEvent({ type: 'draft', status: 'completed', draft: mockDraftForQuery(query) })
  onEvent({ type: 'done', status: 'success' })
}

export function createAITrendDraftState(): AITrendDraftState {
  const state = reactive<AITrendDraftState>({
    durationLabel: '0秒',
    error: '',
    operations: [],
    operationsExpanded: false,
    query: '',
    status: 'idle',
    stop: () => undefined,
    submit: async () => undefined,
    applySelected: () => undefined,
  })
  let controller: AbortController | undefined
  let requestID = 0
  let durationTimer: number | undefined
  let startedAt = 0

  const stopDurationTimer = () => {
    if (durationTimer !== undefined) window.clearInterval(durationTimer)
    durationTimer = undefined
    if (startedAt > 0) state.durationLabel = formatElapsed(Date.now() - startedAt)
  }

  state.stop = () => {
    controller?.abort()
    controller = undefined
    if (state.status === 'running') state.status = 'stopped'
  }

  state.submit = async (query = state.query) => {
    const normalized = query.trim()
    if (normalized === '' || state.status === 'running') return
    const currentRequest = ++requestID
    controller?.abort()
    controller = new AbortController()
    state.query = normalized
    startedAt = Date.now()
    state.durationLabel = '0秒'
    durationTimer = window.setInterval(() => {
      state.durationLabel = formatElapsed(Date.now() - startedAt)
    }, 1000)
    state.operations = []
    state.operationsExpanded = true
    state.error = ''
    state.status = 'running'
    let nextDraft: AITrendDraft | undefined
    try {
      const handleEvent = (event: AITrendStreamEvent) => {
        if (currentRequest !== requestID) return
        if (event.type === 'operation' && event.tool) {
          const status =
            event.status === 'failed'
              ? 'failed'
              : event.status === 'completed'
                ? 'completed'
                : 'running'
          const runningOperation = [...state.operations].reverse().find(
            (operation) => operation.tool === event.tool && operation.status === 'running',
          )
          if (runningOperation && status !== 'running') {
            runningOperation.status = status
            runningOperation.params = event.params
          } else {
            state.operations.push({ tool: event.tool, params: event.params, status })
          }
        }
        if (event.type === 'draft' && event.draft) nextDraft = event.draft
        if (event.type === 'done') {
          if (
            event.status === 'success' ||
            event.status === 'partial' ||
            event.status === 'user_stopped'
          ) {
            if (nextDraft) state.draft = nextDraft
            state.status =
              event.status === 'success'
                ? 'success'
                : event.status === 'partial'
                  ? 'partial'
                  : 'stopped'
            state.operationsExpanded = false
          } else {
            state.status = 'failure'
            state.error = event.error?.message ?? '分析失败'
          }
        }
      }
      if (mockAITrendEnabled) await runMockAITrend(normalized, handleEvent, controller.signal)
      else await streamAITrendDraft(normalized, handleEvent, controller.signal)
      if (state.status === 'running') state.status = nextDraft ? 'success' : 'failure'
      if (nextDraft) state.draft = nextDraft
    } catch (error) {
      if (currentRequest !== requestID) return
      if (controller?.signal.aborted) {
        if (nextDraft) state.draft = nextDraft
        state.status = 'stopped'
      } else {
        state.status = 'failure'
        state.error = error instanceof Error ? error.message : '分析失败'
      }
    } finally {
      stopDurationTimer()
      if (currentRequest === requestID) controller = undefined
    }
  }

  state.applySelected = (apply) => {
    for (const curve of state.draft?.curves ?? []) {
      if (!curve.selected) continue
      apply(curve.condition)
    }
  }
  return state
}
