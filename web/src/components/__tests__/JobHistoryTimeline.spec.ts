import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import JobHistoryTimeline from '../JobHistoryTimeline.vue'
import { clearHistoryCache } from '@/lib/job-history-timeline'
import { getJobHistory, getJobHistoryDetail, type JobHistoryResponse } from '@/api/jd-api'

vi.mock('@/api/jd-api', async () => {
  const actual = await vi.importActual<typeof import('@/api/jd-api')>('@/api/jd-api')
  return {
    ...actual,
    getJobHistory: vi.fn(),
    getJobHistoryDetail: vi.fn(),
  }
})

const historyResponse: JobHistoryResponse = {
  nodes: [
    {
      can_view_historical: true,
      connect_previous: false,
      date: '2026-11-01',
      summary: '首次发现',
      tags: ['discovered'],
      type: 'discovered' as const,
    },
    {
      can_view_historical: true,
      connect_previous: true,
      date: '2026-11-03',
      summary: '岗位职责',
      tags: ['body_responsibilities'],
      type: 'modified' as const,
    },
    {
      can_view_historical: false,
      connect_previous: true,
      date: '2026-11-04',
      summary: '确认下线',
      tags: ['confirmed_closed'],
      type: 'confirmed_closed' as const,
    },
  ],
}

const detail = {
  activity_state: 'effective' as const,
  body: { responsibilities: '负责服务端', requirements: '熟悉 Go' },
  first_discovered_date: '2026-11-01',
  id: 'jd-1',
  is_urgent: false,
  locations: ['上海'],
  other_job_families: [],
  projects: [],
  technology_keywords: [],
  title: '平台工程师',
}

afterEach(() => {
  clearHistoryCache()
  vi.clearAllMocks()
})

describe('JobHistoryTimeline', () => {
  it('renders event nodes, disables unavailable nodes, and opens adjacent detail', async () => {
    vi.mocked(getJobHistory).mockResolvedValue(historyResponse)
    vi.mocked(getJobHistoryDetail).mockResolvedValue({ ...detail, history_node_type: 'modified' })
    const wrapper = mount(JobHistoryTimeline, { attachTo: document.body, props: { jobId: 'jd-1' } })
    await flushPromises()

    expect(wrapper.find('[aria-label="岗位历史时间线，可横向滚动"]').exists()).toBe(true)
    expect(wrapper.findAll('.job-history__node')).toHaveLength(3)
    expect(wrapper.find('.job-history__node--disabled').attributes('disabled')).toBeDefined()

    await wrapper.findAll('.job-history__node')[1]!.trigger('click')
    await flushPromises()
    expect(getJobHistoryDetail).toHaveBeenCalledWith('jd-1', '2026-11-03', 'modified')
    expect(wrapper.get('[role="dialog"]').text()).toContain('平台工程师')
  })

  it('keeps JD content usable in empty and retryable error states', async () => {
    vi.mocked(getJobHistory).mockRejectedValueOnce(new Error('history unavailable'))
    const wrapper = mount(JobHistoryTimeline, { attachTo: document.body, props: { jobId: 'jd-1' } })
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('history unavailable')

    vi.mocked(getJobHistory).mockResolvedValueOnce({ nodes: [] })
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('暂无可展示的岗位历史')
  })

  it('closes detail with Escape and returns focus to the activated node', async () => {
    vi.mocked(getJobHistory).mockResolvedValue(historyResponse)
    vi.mocked(getJobHistoryDetail).mockResolvedValue(detail)
    const wrapper = mount(JobHistoryTimeline, { attachTo: document.body, props: { jobId: 'jd-1' } })
    await flushPromises()
    const node = wrapper.findAll('.job-history__node')[0]!
    await node.trigger('click')
    await flushPromises()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(document.activeElement).toBe(node.element)
  })
})
