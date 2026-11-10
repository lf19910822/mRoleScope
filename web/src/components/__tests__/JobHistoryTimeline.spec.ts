import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import JobHistoryTimeline from '../JobHistoryTimeline.vue'
import { clearHistoryCache, writeHistoryCache } from '@/lib/job-history-timeline'
import { getJobHistory, getJobHistoryDetail, type JobHistoryResponse } from '@/api/jd-api'

vi.mock('@/api/jd-api', async () => {
  const actual = await vi.importActual<typeof import('@/api/jd-api')>('@/api/jd-api')
  return {
    ...actual,
    getJobHistory: vi.fn(),
    getJobHistoryDetail: vi.fn(),
  }
})

vi.mock('vue-echarts', async () => {
  const { h } = await import('vue')
  return {
    default: {
      emits: ['click'],
      name: 'VChart',
      props: ['option'],
      render() {
        return h('div', { 'data-testid': 'mock-history-chart' })
      },
    },
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

    expect(wrapper.find('#job-detail-reading-history').exists()).toBe(true)
    expect(wrapper.find('[aria-label="岗位历史时间线，可缩放和平移"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="mock-history-chart"]').exists()).toBe(true)

    wrapper.findComponent({ name: 'VChart' }).vm.$emit('click', {
      data: ['2026-11-03', 0.5],
      seriesName: '修改',
    })
    await flushPromises()
    expect(getJobHistoryDetail).toHaveBeenCalledWith('jd-1', '2026-11-03', 'modified')
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain('平台工程师')
    wrapper.unmount()
  })

  it('marks the latest observation as the current timeline step', async () => {
    vi.mocked(getJobHistory).mockResolvedValue({
      nodes: [
        {
          can_view_historical: true,
          connect_previous: true,
          date: '2026-11-10',
          summary: '最新观察',
          tags: ['latest'],
          type: 'latest',
        },
      ],
    })
    const wrapper = mount(JobHistoryTimeline, { props: { jobId: 'jd-1' } })
    await flushPromises()

    const option = wrapper.findComponent({ name: 'VChart' }).props('option') as {
      dataZoom: Array<{ type: string; zoomOnMouseWheel: boolean }>
      legend: { data: string[] }
      series: Array<{ name: string; symbol: string }>
      xAxis: { type: string }
      yAxis: { show: boolean }
    }
    expect(option.xAxis.type).toBe('time')
    expect(option.yAxis.show).toBe(false)
    expect(option.dataZoom[0]?.type).toBe('inside')
    expect(option.dataZoom[0]?.zoomOnMouseWheel).toBe(true)
    expect(option.legend.data).toContain('最新观察')
    expect(
      option.series.some((series) => series.name === '最新观察' && series.symbol === 'circle'),
    ).toBe(true)
    wrapper.unmount()
  })

  it('refreshes a closed job history instead of trusting a stale cached latest node', async () => {
    writeHistoryCache('jd-1', {
      nodes: [
        {
          can_view_historical: true,
          connect_previous: true,
          date: '2026-11-10',
          summary: '最新观察',
          tags: ['latest'],
          type: 'latest',
        },
      ],
    })
    vi.mocked(getJobHistory).mockResolvedValue({ nodes: [historyResponse.nodes[2]!] })

    const wrapper = mount(JobHistoryTimeline, {
      props: { activityState: 'confirmed_closed', jobId: 'jd-1' },
    })
    await flushPromises()

    expect(getJobHistory).toHaveBeenCalledWith('jd-1')
    const option = wrapper.findComponent({ name: 'VChart' }).props('option') as {
      legend: { data: string[] }
      series: Array<{ name: string }>
    }
    expect(option.legend.data).toContain('确认下线')
    expect(option.legend.data).not.toContain('最新观察')
    expect(option.series.some((series) => series.name === '最新观察')).toBe(false)
    wrapper.unmount()
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
    wrapper.unmount()
  })

  it('closes detail with Escape and returns focus to the activated node', async () => {
    vi.mocked(getJobHistory).mockResolvedValue(historyResponse)
    vi.mocked(getJobHistoryDetail).mockResolvedValue(detail)
    const wrapper = mount(JobHistoryTimeline, { attachTo: document.body, props: { jobId: 'jd-1' } })
    await flushPromises()
    wrapper.findComponent({ name: 'VChart' }).vm.$emit('click', {
      data: ['2026-11-01', 0.5],
      seriesName: '首次发现',
    })
    await flushPromises()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(document.activeElement).toBe(wrapper.get('.job-history__chart-region').element)
    wrapper.unmount()
  })

  it('locks body scrolling without shifting the page and restores the original styles', async () => {
    const originalOverflow = document.body.style.overflow
    const originalPaddingRight = document.body.style.paddingRight
    const originalClientWidth = document.documentElement.clientWidth
    Object.defineProperty(document.documentElement, 'clientWidth', {
      configurable: true,
      value: window.innerWidth - 24,
    })
    document.body.style.overflow = 'scroll'
    document.body.style.paddingRight = '4px'

    vi.mocked(getJobHistory).mockResolvedValue(historyResponse)
    vi.mocked(getJobHistoryDetail).mockResolvedValue(detail)
    const wrapper = mount(JobHistoryTimeline, { attachTo: document.body, props: { jobId: 'jd-1' } })
    await flushPromises()
    wrapper.findComponent({ name: 'VChart' }).vm.$emit('click', {
      data: ['2026-11-01', 0.5],
      seriesName: '首次发现',
    })
    await flushPromises()

    expect(document.body.style.overflow).toBe('hidden')
    expect(document.body.style.paddingRight).toBe('24px')

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(document.body.style.overflow).toBe('scroll')
    expect(document.body.style.paddingRight).toBe('4px')

    wrapper.unmount()
    Object.defineProperty(document.documentElement, 'clientWidth', {
      configurable: true,
      value: originalClientWidth,
    })
    document.body.style.overflow = originalOverflow
    document.body.style.paddingRight = originalPaddingRight
  })
})
