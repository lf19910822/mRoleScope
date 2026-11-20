import { reactive } from 'vue'

import {
  streamAITrendDraft,
  type AITrendDraft,
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

function formatElapsed(milliseconds: number): string {
  const seconds = Math.max(0, Math.floor(milliseconds / 1000))
  if (seconds < 60) return `${seconds}秒`
  const minutes = Math.floor(seconds / 60)
  const remainder = seconds % 60
  if (minutes < 60) return `${minutes}分${remainder}秒`
  return `${Math.floor(minutes / 60)}小时${minutes % 60}分`
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
    state.draft = undefined
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
      await streamAITrendDraft(normalized, handleEvent, controller.signal)
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
