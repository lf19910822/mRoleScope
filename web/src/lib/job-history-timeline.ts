import type { HistoryNode, HistoryNodeTag, HistoryNodeType, JobHistoryResponse } from '@/api/jd-api'

export const HISTORY_CACHE_TTL_MS = 10 * 60 * 1000

type HistoryCacheEntry = {
  expiresAt: number
  response: JobHistoryResponse
}

const historyCache = new Map<string, HistoryCacheEntry>()

export const historyNodeTypeLabels: Record<HistoryNodeType, string> = {
  discovered: '首次发现',
  modified: '岗位变动',
  confirmed_closed: '确认下线',
  reopened: '重新上线',
  latest: '最新观察',
}

export const historyNodeChartLabels: Record<HistoryNodeType, string> = {
  discovered: '首次发现',
  modified: '修改',
  confirmed_closed: '确认下线',
  reopened: '重新上线',
  latest: '最新观察',
}

export const historyNodeChartColors: Record<HistoryNodeType, string> = {
  discovered: '#0072B2',
  modified: '#E69F00',
  confirmed_closed: '#D55E00',
  reopened: '#CC79A7',
  latest: '#22C55E',
}

export const historyNodeChartSymbols: Record<HistoryNodeType, 'circle' | 'diamond' | 'rect'> = {
  discovered: 'circle',
  modified: 'diamond',
  confirmed_closed: 'rect',
  reopened: 'diamond',
  latest: 'circle',
}

export const historyNodeTagLabels: Record<HistoryNodeTag, string> = {
  discovered: '首次发现',
  latest: '最新观察',
  dimension: '维度变动',
  body_responsibilities: '岗位职责',
  body_requirements: '任职要求',
  body_supplemental: '补充说明',
  confirmed_closed: '确认下线',
  reopened: '重新上线',
}

export function historyNodeTypeLabel(type: HistoryNodeType): string {
  return historyNodeTypeLabels[type]
}

export function historyNodeChartLabel(type: HistoryNodeType): string {
  return historyNodeChartLabels[type]
}

export function historyNodeTagLabel(tag: HistoryNodeTag): string {
  return historyNodeTagLabels[tag]
}

export function historyNodeAccessibleLabel(node: HistoryNode): string {
  const tags = node.tags.map(historyNodeTagLabel).join('、')
  return `${node.date} · ${historyNodeTypeLabel(node.type)}${tags ? ` · ${tags}` : ''}`
}

export function readHistoryCache(id: string, now = Date.now()): JobHistoryResponse | undefined {
  const entry = historyCache.get(id)
  if (!entry) return undefined
  if (entry.expiresAt <= now) {
    historyCache.delete(id)
    return undefined
  }
  return entry.response
}

export function writeHistoryCache(
  id: string,
  response: JobHistoryResponse,
  now = Date.now(),
): void {
  historyCache.set(id, { expiresAt: now + HISTORY_CACHE_TTL_MS, response })
}

export function clearHistoryCache(id?: string): void {
  if (id) historyCache.delete(id)
  else historyCache.clear()
}

export function historyTimelineMinWidth(nodeCount: number, zoom: number): string {
  return `${Math.max(100, nodeCount * 148 * zoom)}px`
}
