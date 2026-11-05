import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('vue-echarts', async () => {
  const { h } = await import('vue')
  return {
    default: {
      name: 'VChart',
      render: () => h('div', { 'data-testid': 'trend-chart-stub' }),
    },
  }
})

const filterOptions = {
  activity_states: [],
  education_levels: [],
  experience_years: [],
  job_families: [],
  locations: [],
  main_job_families: [],
  other_job_families: [],
  projects: [],
  technology_keywords: [],
  urgent_options: [],
}

const mountTrendView = async () => {
  const { default: JobTrendView } = await import('../JobTrendView.vue')
  return mount(JobTrendView)
}

afterEach(() => {
  sessionStorage.clear()
  vi.resetModules()
  vi.unstubAllGlobals()
})

describe('JobTrendView observation ledger', () => {
  it('offers one first-curve action, then creates and exposes the first draft configuration', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        throw new Error(`unexpected request ${String(input)}`)
      }),
    )
    const wrapper = await mountTrendView()
    await flushPromises()

    expect(wrapper.get('[data-testid="start-first-curve"]').text()).toBe('开始配置首条曲线')
    expect(wrapper.find('[data-testid="open-trend-configuration"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('每点为当日匹配岗位数。')
    expect(wrapper.text()).not.toContain('刷新数据')

    await wrapper.get('[data-testid="start-first-curve"]').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.job-trend-view__curve-row')).toHaveLength(1)
    expect(wrapper.get('[data-testid="trend-configuration-rail"]').classes()).toContain(
      'job-trend-view__rail--mobile-open',
    )
    expect(wrapper.get('[data-testid="open-trend-configuration"]').text()).toContain('1')

    wrapper.unmount()
  })

  it('carries a rendered curve’s stable observation colour and honest latest summary into its rail row', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (String(input).startsWith('/api/v1/job-trends?')) {
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              points: [
                { count: 3, date: '2026-11-03' },
                { count: null, date: '2026-11-04' },
                { count: 0, date: '2026-11-05' },
              ],
            }),
          )
        }
        throw new Error(`unexpected request ${String(input)}`)
      }),
    )
    const { useJobTrendDashboard } = await import('@/lib/job-trend-dashboard')
    const dashboard = useJobTrendDashboard()
    const row = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    expect(row).not.toBeNull()
    row!.appliedCondition = { ...row!.condition }
    row!.series = {
      curve: { dimension: 'technology', label: 'Go', value: 'go' },
      freshness: 'safe-marker',
      points: [
        { count: 3, date: '2026-11-03' },
        { count: null, date: '2026-11-04' },
        { count: 0, date: '2026-11-05' },
      ],
    }
    row!.status = 'ready'
    const wrapper = await mountTrendView()
    await flushPromises()
    await nextTick()

    expect(wrapper.get('[data-testid="curve-marker-trend-1"]').attributes('style')).toContain(
      '#007cab',
    )
    expect(wrapper.get('[data-testid="curve-observation-trend-1"]').text()).toContain(
      '最近观测 0',
    )
    expect(wrapper.get('[data-testid="curve-observation-trend-1"]').text()).toContain(
      '较前一有效日 -3',
    )

    wrapper.unmount()
  })

  it('reveals the available all-history gestures only after the reader selects all history', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (String(input).startsWith('/api/v1/job-trends?')) {
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              points: [{ count: 3, date: '2026-11-05' }],
            }),
          )
        }
        throw new Error(`unexpected request ${String(input)}`)
      }),
    )
    const { useJobTrendDashboard } = await import('@/lib/job-trend-dashboard')
    const dashboard = useJobTrendDashboard()
    const row = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })
    expect(row).not.toBeNull()
    row!.appliedCondition = { ...row!.condition }
    row!.series = {
      curve: { dimension: 'technology', label: 'Go', value: 'go' },
      freshness: 'safe-marker',
      points: [{ count: 3, date: '2026-11-05' }],
    }
    row!.status = 'ready'
    const wrapper = await mountTrendView()
    await flushPromises()

    expect(wrapper.find('[data-testid="all-history-gestures"]').exists()).toBe(false)

    const allHistoryButton = wrapper
      .findAll('button')
      .find((button) => button.text() === '全部历史')
    expect(allHistoryButton).toBeDefined()
    await allHistoryButton!.trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="all-history-gestures"]').text()).toBe(
      '滚轮缩放时间 · Shift+滚轮缩放岗位数 · 拖拽平移',
    )

    wrapper.unmount()
  })
})
