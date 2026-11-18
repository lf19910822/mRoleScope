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
  error: string
  operations: AITrendOperation[]
  operationsExpanded: boolean
  query: string
  status: 'idle' | 'running' | 'success' | 'partial' | 'failure' | 'stopped'
  stop(): void
  submit(query?: string): Promise<void>
  applySelected(apply: (condition: TrendCondition) => void): void
}

export function createAITrendDraftState(): AITrendDraftState {
  const state = reactive<AITrendDraftState>({
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
    state.operations = []
    state.operationsExpanded = true
    state.error = ''
    state.status = 'running'
    let nextDraft: AITrendDraft | undefined
    try {
      await streamAITrendDraft(
        normalized,
        (event: AITrendStreamEvent) => {
          if (currentRequest !== requestID) return
          if (event.type === 'operation' && event.tool) {
            state.operations.push({ tool: event.tool, params: event.params, status: event.status === 'failed' ? 'failed' : event.status === 'completed' ? 'completed' : 'running' })
          }
          if (event.type === 'draft' && event.draft) nextDraft = event.draft
          if (event.type === 'done') {
            if (event.status === 'success' || event.status === 'partial' || event.status === 'user_stopped') {
              if (nextDraft) state.draft = nextDraft
              state.status = event.status === 'success' ? 'success' : event.status === 'partial' ? 'partial' : 'stopped'
              state.operationsExpanded = false
            } else {
              state.status = 'failure'
              state.error = event.error?.message ?? '分析失败'
            }
          }
        },
        controller.signal,
      )
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
