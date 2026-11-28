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
  assistantMessage: string
  assistantMessageWarning: string
  draft?: AITrendDraft
  draftFromPreviousRequest: boolean
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
    assistantMessage: '',
    assistantMessageWarning: '',
    draftFromPreviousRequest: false,
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
    const requestController = new AbortController()
    controller = requestController
    state.query = normalized
    startedAt = Date.now()
    state.durationLabel = '0秒'
    durationTimer = window.setInterval(() => {
      state.durationLabel = formatElapsed(Date.now() - startedAt)
    }, 1000)
    state.operations = []
    state.operationsExpanded = true
    state.error = ''
    state.assistantMessage = ''
    state.assistantMessageWarning = ''
    const previousDraft = state.draft
    state.draftFromPreviousRequest = !!previousDraft
    state.status = 'running'
    let nextDraft: AITrendDraft | undefined
    const restoreDraft = (candidate?: AITrendDraft) => {
      state.draft = candidate ?? previousDraft
      state.draftFromPreviousRequest = !candidate && !!previousDraft
    }
    const markIncomplete = () => {
      if (state.assistantMessage) {
        state.assistantMessageWarning = '结果未完整接收，以上仅为已显示的部分。'
      }
    }
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
        if (event.type === 'draft' && event.draft && (event.draft.curves.length > 0 || !previousDraft)) {
          nextDraft = event.draft
        }
        if (event.type === 'assistant_message' && event.delta) {
          state.assistantMessage += event.delta
        }
        if (event.type === 'done') {
          if (
            event.status === 'success' ||
            event.status === 'partial' ||
            event.status === 'user_stopped'
          ) {
            restoreDraft(event.status === 'user_stopped' && previousDraft ? undefined : nextDraft)
            state.status =
              event.status === 'success'
                ? 'success'
                : event.status === 'partial'
                  ? 'partial'
                  : 'stopped'
            if (event.status === 'user_stopped') markIncomplete()
            state.operationsExpanded = false
          } else {
            state.status = 'failure'
            state.error = event.error?.message ?? '分析失败'
            restoreDraft()
          }
        }
      }
      await streamAITrendDraft(normalized, handleEvent, requestController.signal)
      if (state.status === 'running') {
        state.status = nextDraft || state.assistantMessage ? 'partial' : 'failure'
        markIncomplete()
        restoreDraft(previousDraft ? undefined : nextDraft)
      }
    } catch (error) {
      if (currentRequest !== requestID) return
      if (requestController.signal.aborted) {
        restoreDraft(previousDraft ? undefined : nextDraft)
        state.status = 'stopped'
        markIncomplete()
      } else {
        state.status = 'failure'
        state.error = error instanceof Error ? error.message : '分析失败'
        restoreDraft()
        markIncomplete()
      }
    } finally {
      if (currentRequest === requestID) {
        stopDurationTimer()
        controller = undefined
      }
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
