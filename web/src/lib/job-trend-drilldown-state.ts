import { reactive } from 'vue'

import type { TrendCondition } from './job-trend-dashboard-state'

export type TrendDrilldownStatus = 'changed' | 'confirmed_closed' | 'current'

export interface TrendDrilldownItem {
  can_view_historical: boolean
  can_view_latest: boolean
  id: string
  status: TrendDrilldownStatus
  title: string
}

export interface TrendDrilldownResponse {
	count_matches_expected?: boolean
  items: TrendDrilldownItem[]
  matched_total: number
  node_date: string
  page: { number: number; page_size: number; total: number }
}

export interface TrendDrilldownRequest {
  condition: TrendCondition
  count: number
  date: string
  page: number
  pageSize: number
  title: string
}

export interface TrendDrilldownClient {
  readTrendDrilldown(
    request: TrendDrilldownRequest,
    signal?: AbortSignal,
  ): Promise<TrendDrilldownResponse>
}

export interface TrendDrilldownNode {
  condition: TrendCondition
  count: number
  date: string
}

export interface TrendDrilldownState {
  close(): void
  error: string
  isOpen: boolean
  isLoading: boolean
  node?: TrendDrilldownNode
  open(condition: TrendCondition, date: string, count: number | null): Promise<void>
  page: number
  response?: TrendDrilldownResponse
  retry(): Promise<void>
  setPage(page: number): Promise<void>
  setTitle(title: string): void
  title: string
}

export interface TrendDrilldownStateOptions {
  now?: () => number
}

const cacheLifetimeMilliseconds = 10 * 60 * 1000
const pageSize = 20

function copyCondition(condition: TrendCondition): TrendCondition {
  return {
    dimension: condition.dimension,
    includeSecondary: condition.includeSecondary,
    operator: condition.operator,
    value:
      condition.dimension === 'technology'
        ? condition.value.trim().toLocaleLowerCase()
        : condition.value.trim(),
  }
}

function nodeKey(node: TrendDrilldownNode): string {
  const condition = node.condition
  return [node.date, node.count, condition.dimension, condition.includeSecondary, condition.operator, condition.value].join('|')
}

function requestKey(node: TrendDrilldownNode, title: string, page: number): string {
  return `${nodeKey(node)}|${title.trim().toLocaleLowerCase()}|${page}|${pageSize}`
}

export function createTrendDrilldownState(
  client: TrendDrilldownClient,
  options: TrendDrilldownStateOptions = {},
): TrendDrilldownState {
  const now = options.now ?? Date.now
  const cache = new Map<string, { cachedAt: number; response: TrendDrilldownResponse }>()
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let generation = 0
  const state: TrendDrilldownState = reactive({
    close(): void {
      controller?.abort()
      controller = undefined
      if (timer !== undefined) clearTimeout(timer)
      timer = undefined
      generation += 1
      state.error = ''
      state.isLoading = false
      state.isOpen = false
      state.node = undefined
      state.response = undefined
      state.title = ''
      state.page = 1
    },
    error: '',
    isLoading: false,
    isOpen: false,
    node: undefined,
    async open(condition: TrendCondition, date: string, count: number | null): Promise<void> {
      if (count === null) return
      const node: TrendDrilldownNode = { condition: copyCondition(condition), count, date }
      if (state.isOpen && state.node !== undefined && nodeKey(state.node) === nodeKey(node)) {
        state.close()
        return
      }
      if (timer !== undefined) clearTimeout(timer)
      timer = undefined
      controller?.abort()
      controller = undefined
      generation += 1
      state.error = ''
      state.isOpen = true
      state.isLoading = false
      state.node = node
      state.page = 1
      state.response = undefined
      state.title = ''
      await load(false)
    },
    page: 1,
    response: undefined,
    async retry(): Promise<void> {
      await load(true)
    },
    async setPage(nextPage: number): Promise<void> {
      if (!Number.isSafeInteger(nextPage) || nextPage < 1 || state.node === undefined) return
      state.page = nextPage
      await load(false)
    },
    setTitle(title: string): void {
      state.title = title
      state.page = 1
      if (!state.isOpen || state.node === undefined) return
      if (timer !== undefined) clearTimeout(timer)
      timer = setTimeout(() => {
        timer = undefined
        void load(false)
      }, 300)
    },
    title: '',
  })

  async function load(force: boolean): Promise<void> {
    const node = state.node
    if (!state.isOpen || node === undefined) return
    const title = state.title.trim()
    const page = state.page
    const key = requestKey(node, title, page)
    const cached = cache.get(key)
    if (!force && cached !== undefined && now() - cached.cachedAt <= cacheLifetimeMilliseconds) {
      state.error = ''
      state.isLoading = false
      state.response = cached.response
      return
    }
    // A new request represents a new node/search/page context. Do not leave
    // a previous page actionable underneath its loading or error state.
    state.response = undefined
    controller?.abort()
    const nextController = new AbortController()
    controller = nextController
    const requestGeneration = ++generation
    state.error = ''
    state.isLoading = true
    try {
      const response = await client.readTrendDrilldown(
        { condition: node.condition, count: node.count, date: node.date, page, pageSize, title },
        nextController.signal,
      )
      if (generation !== requestGeneration || !state.isOpen || state.node === undefined || nodeKey(state.node) !== nodeKey(node)) return
      cache.set(key, { cachedAt: now(), response })
      state.response = response
    } catch {
      if (generation !== requestGeneration) return
      state.error = '无法读取该日岗位，请重试。'
    } finally {
      if (generation === requestGeneration) {
        state.isLoading = false
        controller = undefined
      }
    }
  }

  return state
}
