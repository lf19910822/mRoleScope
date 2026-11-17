import { flushPromises, mount } from '@vue/test-utils'
import { NSelect } from 'naive-ui'
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
  technology_keywords: [
    {
      value: 'go',
      label: 'Go',
      category: { value: 'programming-languages', label: '编程语言' },
    },
    {
      value: 'python',
      label: 'Python',
      category: { value: 'programming-languages', label: '编程语言' },
    },
  ],
  urgent_options: [],
}

const mountTrendView = async (options: Parameters<typeof mount>[1] = {}) => {
  const { default: JobTrendView } = await import('../JobTrendView.vue')
  return mount(JobTrendView, options)
}

afterEach(() => {
  sessionStorage.clear()
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1024 })
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
    expect(wrapper.find('[data-testid="open-trend-configuration"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('每点为当日匹配岗位数。')
    expect(wrapper.text()).not.toContain('刷新数据')

    await wrapper.get('[data-testid="start-first-curve"]').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.job-trend-view__curve-row')).toHaveLength(1)
    expect(wrapper.find('[aria-labelledby="trend-event-title"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="trend-configuration-rail"]').classes()).toContain(
      'job-trend-view__rail--desktop-expanded',
    )
    expect(wrapper.find('[data-testid="close-trend-configuration"]').exists()).toBe(true)

    wrapper.unmount()
  })

  it('opens and closes the responsive configuration drawer without losing keyboard focus', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        throw new Error(`unexpected request ${String(input)}`)
      }),
    )
    const wrapper = await mountTrendView({ attachTo: document.body })
    await flushPromises()

    await wrapper.get('[data-testid="start-first-curve"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="trend-configuration-backdrop"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="close-trend-configuration"]').exists()).toBe(true)

    await wrapper.get('[data-testid="close-trend-configuration"]').trigger('click')
    await nextTick()
    expect(wrapper.find('[data-testid="trend-configuration-backdrop"]').exists()).toBe(false)
    expect(document.activeElement).toBe(wrapper.get('[data-testid="open-trend-configuration"]').element)

    await wrapper.get('[data-testid="open-trend-configuration"]').trigger('click')
    await nextTick()
    await wrapper.trigger('keydown', { key: 'Escape' })
    await nextTick()
    expect(wrapper.find('[data-testid="trend-configuration-backdrop"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('reuses the categorized technology selector as an independent panel for a technology curve', async () => {
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

    await wrapper.get('[data-testid="start-first-curve"]').trigger('click')
    await flushPromises()

    const selector = wrapper.findComponent({ name: 'TechnologyKeywordSelect' })
    expect(selector.exists()).toBe(true)
    expect(selector.props('options')).toEqual(filterOptions.technology_keywords)
    expect(wrapper.get('.job-trend-view__technology-select').classes()).toContain(
      'job-trend-view__technology-select',
    )

    await selector.get('[data-testid="technology-keyword-trigger"]').trigger('click')
    const technologyMenu = document.body.querySelector<HTMLElement>(
      '[data-testid="technology-keyword-menu"]',
    )
    expect(technologyMenu).not.toBeNull()
    await technologyMenu!
      .querySelector<HTMLElement>('[data-testid="technology-keyword-go"]')
      ?.click()
    await nextTick()
    expect(wrapper.text()).toContain('技术关键词 · go')

    wrapper.unmount()
  })

  it('carries a rendered curve’s stable observation colour into the compact rail card', async () => {
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

    const marker = wrapper.get('[data-testid="curve-marker-trend-1"]')
    expect(marker.attributes('style')).toContain('#0072B2')
    expect(marker.attributes('title')).toBeUndefined()
    expect(
      wrapper
        .get('[data-testid="curve-marker-slot-trend-1"] .job-trend-view__curve-marker-note')
        .text(),
    ).toBe('技术关键词 · Go')
    await marker.trigger('click')
    await nextTick()
    expect(wrapper.get('[data-testid="curve-card-trend-1"]').attributes('style')).toContain(
      '#0072B2',
    )
    expect(wrapper.text()).toContain('当前周期：2026.11.03 至 2026.11.05')
    expect(wrapper.find('[data-testid="curve-observation-trend-1"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('allows several draft cards and applies configured cards while highlighting every incomplete card', async () => {
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

    await wrapper.get('[data-testid="start-first-curve"]').trigger('click')
    await flushPromises()
    const addButton = wrapper.findAll('button').find((button) => button.text().includes('添加曲线'))
    expect(addButton).toBeDefined()
    await addButton!.trigger('click')
    expect(wrapper.findAll('.job-trend-view__curve-row')).toHaveLength(2)

    await wrapper.get('[data-testid="apply-all-curves"]').trigger('click')
    await nextTick()
    expect(wrapper.findAll('.job-trend-view__curve-row--incomplete')).toHaveLength(2)
    expect(wrapper.findAll('.job-trend-view__curve-pending')).toHaveLength(2)
    expect(wrapper.get('[data-testid="curve-card-trend-1"]').classes()).toContain(
      'job-trend-view__curve-row--focused',
    )
    expect(wrapper.text()).toContain('还有 2 张曲线卡待配置。')

    await wrapper.get('[data-testid="clear-all-curves"]').trigger('click')
    expect(wrapper.findAll('.job-trend-view__curve-row')).toHaveLength(0)
    expect(wrapper.find('[data-testid="trend-empty-state"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="curve-rail-footer"]').text()).toContain('0/8 条')
    expect(wrapper.find('[data-testid="clear-all-curves"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="apply-all-curves"]').exists()).toBe(true)

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

  it('renders event counts as drill-down actions while keeping comparison rates non-clickable', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              points: [{ count: 3, date: '2026-11-05' }],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/events?')) {
          return new Response(
            JSON.stringify({
              current: { start: '2026-11-01', end: '2026-11-05' },
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              previous: { start: '2026-10-01', end: '2026-10-05' },
              rows: [
                {
                  current: 9,
                  previous: { count: 7, rate: 28.6 },
                  type: 'entered',
                  year_over_year: { count: 10, rate: -10 },
                },
                {
                  current: 0,
                  previous: { count: 0, rate: null },
                  type: 'modified',
                  year_over_year: { count: 0, rate: null },
                },
                {
                  current: 0,
                  previous: { count: 0, rate: null },
                  type: 'left',
                  year_over_year: { count: 0, rate: null },
                },
                {
                  current: 0,
                  previous: { count: 0, rate: null },
                  type: 'reopened',
                  year_over_year: { count: 0, rate: null },
                },
              ],
              status: 'available',
              year_over_year: { start: '2025-11-01', end: '2025-11-05' },
            }),
          )
        }
        throw new Error(`unexpected request ${path}`)
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

    const firstRow = wrapper.get('.job-trend-view__event-table tbody tr')
    expect(wrapper.findAll('.job-trend-view__event-table tbody tr')).toHaveLength(4)
    expect(wrapper.find('[data-testid="event-count-definition"]').exists()).toBe(false)
    expect(firstRow.classes()).toContain('job-trend-view__event-row--entered')
    expect(wrapper.get('[data-testid="event-curve-selector"]').text()).toContain('技术关键词 · Go')
    expect(wrapper.find('[data-testid="event-period-current"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="event-period-previous"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="event-period-year-over-year"]').exists()).toBe(false)
    expect(
      wrapper.get('[data-testid="event-period-current-help"]').attributes('data-tooltip'),
    ).toBe('当前周期：2026-11-01 至 2026-11-05')
    expect(
      wrapper.get('[data-testid="event-period-year-over-year-help"]').attributes('data-tooltip'),
    ).toBe('同比周期：2025-11-01 至 2025-11-05')
    expect(
      wrapper.get('[data-testid="event-period-previous-help"]').attributes('data-tooltip'),
    ).toBe('环比周期：2026-10-01 至 2026-10-05')
    expect(
      firstRow.get('[data-testid="event-definition-entered"]').attributes('data-tooltip'),
    ).toContain('首次被观察到进入该曲线')
    expect(firstRow.get('th').text()).toContain('新增')
    const comparisonCell = firstRow.findAll('td')[2]!
    expect(comparisonCell.text()).toContain('7')
    expect(comparisonCell.text()).toContain('+28.6%')
    expect(comparisonCell.findAll('button')).toHaveLength(1)
    expect(comparisonCell.get('button').text()).toBe('7')
    expect(firstRow.get('[data-testid="event-count-current"]').classes()).toContain(
      'job-trend-view__event-count--current',
    )
    expect(comparisonCell.get('[data-testid="event-count-previous"]').classes()).toContain(
      'job-trend-view__event-count--comparison',
    )
    expect(comparisonCell.get('[data-testid="event-rate-previous"]').text()).toBe('+28.6%')
    expect(firstRow.findAll('td')[0]!.attributes('data-label')).toBe('当前周期')
    expect(firstRow.findAll('td')[1]!.attributes('data-label')).toBe('同比')
    expect(firstRow.findAll('td')[2]!.attributes('data-label')).toBe('环比')

    const modifiedRow = wrapper.findAll('.job-trend-view__event-table tbody tr')[1]!
    expect(modifiedRow.get('[data-testid="event-count-current"]').text()).toBe('0')
    expect(modifiedRow.get('[data-testid="event-rate-previous"]').text()).toBe('-')
    expect(modifiedRow.get('th').text()).toContain('修改')
    expect(
      wrapper.find('[data-testid="event-definition-left"]').attributes('data-tooltip'),
    ).toContain('移出该曲线')
    expect(wrapper.findAll('.job-trend-view__event-table tbody th')[2]!.text()).toContain('下线')
    expect(wrapper.findAll('.job-trend-view__event-table tbody th')[3]!.text()).toContain('重上线')
    expect(
      wrapper.find('[data-testid="event-definition-reopened"]').attributes('data-tooltip'),
    ).toContain('确认下线后恢复有效')

    wrapper.unmount()
  })

  it('opens an empty event drill-down for a legal zero count', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              points: [{ count: 3, date: '2026-11-05' }],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/events?')) {
          return new Response(
            JSON.stringify({
              current: { start: '2026-11-01', end: '2026-11-05' },
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              previous: { start: '2026-10-01', end: '2026-10-05' },
              rows: ['entered', 'modified', 'left', 'reopened'].map((type) => ({
                current: 0,
                previous: { count: 0, rate: null },
                type,
                year_over_year: { count: 0, rate: null },
              })),
              status: 'available',
              year_over_year: { start: '2025-11-01', end: '2025-11-05' },
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/drilldown?')) {
          return new Response(
            JSON.stringify({
              event_type: 'modified',
              items: [],
              matched_total: 0,
              mode: 'event',
              page: { number: 1, page_size: 20, total: 0 },
              period_end: '2026-11-05',
              period_start: '2026-11-01',
            }),
          )
        }
        throw new Error(`unexpected request ${path}`)
      }),
    )
    const { useJobTrendDashboard } = await import('@/lib/job-trend-dashboard')
    const dashboard = useJobTrendDashboard()
    const wrapper = await mountTrendView()
    await flushPromises()
    const row = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })!
    row.appliedCondition = { ...row.condition }
    row.series = {
      curve: { dimension: 'technology', label: 'Go', value: 'go' },
      freshness: 'safe-marker',
      points: [{ count: 3, date: '2026-11-05' }],
    }
    row.status = 'ready'
    await flushPromises()

    const modifiedRow = wrapper.findAll('.job-trend-view__event-table tbody tr')[1]!
    await modifiedRow.get('button').trigger('click')
    await flushPromises()

    expect(wrapper.get('[aria-label="趋势节点岗位"]').text()).toContain('该周期没有符合条件的岗位')
    expect(wrapper.text()).not.toContain('统计暂不可用')
    wrapper.unmount()
  })

  it('shows unavailable comparison state without exposing an empty drill-down', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              points: [{ count: 3, date: '2026-11-05' }],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/events?')) {
          return new Response(
            JSON.stringify({
              current: { start: '2026-11-01', end: '2026-11-05' },
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              previous: { start: '2026-10-01', end: '2026-10-05' },
              rows: ['entered', 'modified', 'left', 'reopened'].map((type) => ({
                current: 0,
                previous: { count: null, rate: null },
                type,
                year_over_year: { count: null, rate: null },
              })),
              status: 'unavailable',
              year_over_year: { start: '2025-11-01', end: '2025-11-05' },
            }),
          )
        }
        throw new Error(`unexpected request ${path}`)
      }),
    )
    const { useJobTrendDashboard } = await import('@/lib/job-trend-dashboard')
    const dashboard = useJobTrendDashboard()
    const row = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })!
    row.appliedCondition = { ...row.condition }
    row.series = {
      curve: { dimension: 'technology', label: 'Go', value: 'go' },
      freshness: 'safe-marker',
      points: [{ count: 3, date: '2026-11-05' }],
    }
    row.status = 'ready'
    const wrapper = await mountTrendView()
    await flushPromises()

    expect(wrapper.get('.job-trend-view__event-empty').text()).toBe('暂无可比数据')
    expect(wrapper.find('.job-trend-view__event-table').exists()).toBe(false)
    expect(wrapper.find('[aria-label="趋势节点岗位"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps event failure local and retries only the event request', async () => {
    let eventReads = 0
    let trendReads = 0
    let allowEventRetry = false
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          trendReads += 1
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              points: [{ count: 3, date: '2026-11-05' }],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/events?')) {
          eventReads += 1
          if (!allowEventRetry) return new Response('{}', { status: 503 })
          return new Response(
            JSON.stringify({
              current: { start: '2026-11-01', end: '2026-11-05' },
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              previous: { start: '2026-10-01', end: '2026-10-05' },
              rows: ['entered', 'modified', 'left', 'reopened'].map((type) => ({
                current: 0,
                previous: { count: 0, rate: null },
                type,
                year_over_year: { count: 0, rate: null },
              })),
              status: 'available',
              year_over_year: { start: '2025-11-01', end: '2025-11-05' },
            }),
          )
        }
        throw new Error(`unexpected request ${path}`)
      }),
    )
    const { useJobTrendDashboard } = await import('@/lib/job-trend-dashboard')
    const dashboard = useJobTrendDashboard()
    const wrapper = await mountTrendView()
    await flushPromises()
    const row = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })!
    row.appliedCondition = { ...row.condition }
    row.series = {
      curve: { dimension: 'technology', label: 'Go', value: 'go' },
      freshness: 'safe-marker',
      points: [{ count: 3, date: '2026-11-05' }],
    }
    row.status = 'ready'
    await flushPromises()

    expect(wrapper.get('.job-trend-view__event-error').text()).toContain('统计暂不可用')
    expect(wrapper.find('[data-testid="trend-chart-region"]').exists()).toBe(true)
    const trendReadsBeforeRetry = trendReads
    allowEventRetry = true
    await wrapper.get('.job-trend-view__event-error button').trigger('click')
    await flushPromises()
    expect(eventReads).toBeGreaterThanOrEqual(2)
    expect(trendReads).toBe(trendReadsBeforeRetry)
    expect(wrapper.find('.job-trend-view__event-table').exists()).toBe(true)
    wrapper.unmount()
  })

  it('switches the event table with a curve selector without changing curve configuration', async () => {
    let eventReads = 0
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          const query = new URL(path, 'http://localhost').searchParams
          const dimension = query.get('dimension') ?? 'technology'
          const value = query.get('value') ?? 'go'
          return new Response(
            JSON.stringify({
              curve: {
                dimension,
                label: dimension === 'education' ? '本科' : 'Go',
                ...(dimension === 'education' ? { operator: 'eq' } : {}),
                value,
              },
              freshness: 'line-freshness',
              points: [],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/events?')) {
          eventReads += 1
          return new Response(
            JSON.stringify({
              current: { start: '2026-11-01', end: '2026-11-05' },
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              previous: { start: '2026-10-01', end: '2026-10-05' },
              rows: [
                {
                  current: 0,
                  previous: { count: 0, rate: null },
                  type: 'entered',
                  year_over_year: { count: 0, rate: null },
                },
                {
                  current: 0,
                  previous: { count: 0, rate: null },
                  type: 'modified',
                  year_over_year: { count: 0, rate: null },
                },
                {
                  current: 0,
                  previous: { count: 0, rate: null },
                  type: 'left',
                  year_over_year: { count: 0, rate: null },
                },
                {
                  current: 0,
                  previous: { count: 0, rate: null },
                  type: 'reopened',
                  year_over_year: { count: 0, rate: null },
                },
              ],
              status: 'available',
              year_over_year: { start: '2025-11-01', end: '2025-11-05' },
            }),
          )
        }
        throw new Error(`unexpected request ${path}`)
      }),
    )
    const { useJobTrendDashboard } = await import('@/lib/job-trend-dashboard')
    const dashboard = useJobTrendDashboard()
    const education = dashboard.addCurve({
      dimension: 'education',
      includeSecondary: false,
      operator: 'eq',
      value: 'bachelor',
    })!
    education.appliedCondition = { ...education.condition }
    education.series = {
      curve: { dimension: 'education', label: '本科', operator: 'eq', value: 'bachelor' },
      freshness: 'education-freshness',
      points: [],
    }
    education.status = 'ready'
    const technology = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })!
    technology.appliedCondition = { ...technology.condition }
    technology.series = {
      curve: { dimension: 'technology', label: 'Go', value: 'go' },
      freshness: 'technology-freshness',
      points: [],
    }
    technology.status = 'ready'
    const configurationBefore = dashboard.rows.map((row) => ({ ...row.condition }))

    const wrapper = await mountTrendView()
    await flushPromises()
    expect(wrapper.get('#trend-event-title').text()).toContain('学历 = 本科')
    expect(wrapper.get('.job-trend-view__event-development').text()).toBe('开发中')
    expect(eventReads).toBe(0)

    expect(wrapper.find('[aria-label="上一条曲线"]').exists()).toBe(false)
    expect(wrapper.find('[aria-label="下一条曲线"]').exists()).toBe(false)
    const eventSelector = wrapper
      .findAllComponents(NSelect)
      .find((component) =>
        (component.props('options') as Array<{ label?: string }>).some(
          (option) => option.label === '学历 = 本科',
        ),
      )
    expect(eventSelector).toBeDefined()
    await eventSelector!.vm.$emit('update:value', technology.id)
    await flushPromises()
    expect(wrapper.get('#trend-event-title').text()).toContain('技术关键词 · Go')
    expect(wrapper.find('.job-trend-view__event-table').exists()).toBe(true)
    expect(eventReads).toBe(1)
    expect(eventSelector!.props('value')).toBe(technology.id)

    await eventSelector!.vm.$emit('update:value', education.id)
    await flushPromises()
    expect(wrapper.get('#trend-event-title').text()).toContain('学历 = 本科')
    expect(eventReads).toBe(1)
    expect(dashboard.rows.map((row) => row.condition)).toEqual(configurationBefore)
    wrapper.unmount()
  })

  it('keeps curve selection separate from point drill-down selection', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          const query = new URL(path, 'http://localhost').searchParams
          const value = query.get('value') ?? 'go'
          return new Response(
            JSON.stringify({
              curve: {
                dimension: 'technology',
                label: value === 'python' ? 'Python' : 'Go',
                value,
              },
              freshness: 'line-freshness',
              points: [{ count: 1, date: '2026-11-05' }],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/events?')) {
          return new Response(
            JSON.stringify({
              current: { start: '2026-11-01', end: '2026-11-05' },
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              previous: { start: '2026-10-01', end: '2026-10-05' },
              rows: [
                {
                  current: 1,
                  previous: { count: 0, rate: null },
                  type: 'entered',
                  year_over_year: { count: 0, rate: null },
                },
                {
                  current: 0,
                  previous: { count: 0, rate: null },
                  type: 'modified',
                  year_over_year: { count: 0, rate: null },
                },
                {
                  current: 0,
                  previous: { count: 0, rate: null },
                  type: 'left',
                  year_over_year: { count: 0, rate: null },
                },
                {
                  current: 0,
                  previous: { count: 0, rate: null },
                  type: 'reopened',
                  year_over_year: { count: 0, rate: null },
                },
              ],
              status: 'available',
              year_over_year: { start: '2025-11-01', end: '2025-11-05' },
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/drilldown?'))
          return new Response(
            JSON.stringify({
              items: [],
              matched_total: 0,
              node_date: '2026-11-05',
              page: { number: 1, page_size: 20, total: 0 },
            }),
          )
        throw new Error(`unexpected request ${path}`)
      }),
    )
    const { useJobTrendDashboard } = await import('@/lib/job-trend-dashboard')
    const dashboard = useJobTrendDashboard()
    for (const [value, label] of [
      ['go', 'Go'],
      ['python', 'Python'],
    ] as const) {
      const row = dashboard.addCurve({
        dimension: 'technology',
        includeSecondary: false,
        operator: '',
        value,
      })!
      row.appliedCondition = { ...row.condition }
      row.series = {
        curve: { dimension: 'technology', label, value },
        freshness: `${value}-freshness`,
        points: [{ count: 1, date: '2026-11-05' }],
      }
      row.status = 'ready'
    }
    const wrapper = await mountTrendView()
    await flushPromises()
    expect(wrapper.get('#trend-event-title').text()).toContain('技术关键词 · Go')

    await wrapper.findComponent({ name: 'VChart' }).vm.$emit('click', {
      seriesName: '技术关键词 · Python',
    })
    await flushPromises()
    expect(wrapper.get('#trend-event-title').text()).toContain('技术关键词 · Python')

    await wrapper.findComponent({ name: 'VChart' }).vm.$emit('click', {
      data: ['2026-11-05', 1],
      seriesName: '技术关键词 · Go',
    })
    await flushPromises()
    expect(wrapper.get('#trend-event-title').text()).toContain('技术关键词 · Python')
    expect(wrapper.get('[aria-label="趋势节点岗位"]').text()).toContain('节点日期 2026-11-05')
    wrapper.unmount()
  })

  it('shows development status for education without issuing an event request', async () => {
    let eventReads = 0
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends/events?')) eventReads += 1
        throw new Error(`unexpected request ${path}`)
      }),
    )
    const { useJobTrendDashboard } = await import('@/lib/job-trend-dashboard')
    const dashboard = useJobTrendDashboard()
    const row = dashboard.addCurve({
      dimension: 'education',
      includeSecondary: false,
      operator: 'eq',
      value: 'bachelor',
    })!
    row.appliedCondition = { ...row.condition }
    row.series = {
      curve: { dimension: 'education', label: '本科', operator: 'eq', value: 'bachelor' },
      freshness: 'safe-marker',
      points: [{ count: 3, date: '2026-11-05' }],
    }
    row.status = 'ready'
    const wrapper = await mountTrendView()
    await flushPromises()

    expect(wrapper.get('.job-trend-view__event-development').text()).toBe('开发中')
    expect(eventReads).toBe(0)
    expect(wrapper.find('.job-trend-view__event-table').exists()).toBe(false)
    wrapper.unmount()
  })

  it('renders safe event dates and reasons inside the reused drill-down', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              points: [{ count: 3, date: '2026-11-05' }],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/events?')) {
          return new Response(
            JSON.stringify({
              current: { start: '2026-11-01', end: '2026-11-05' },
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              previous: { start: '2026-10-01', end: '2026-10-05' },
              rows: [
                {
                  current: 1,
                  previous: { count: 0, rate: null },
                  type: 'entered',
                  year_over_year: { count: 0, rate: null },
                },
              ],
              status: 'available',
              year_over_year: { start: '2025-11-01', end: '2025-11-05' },
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/drilldown?')) {
          return new Response(
            JSON.stringify({
              event_type: 'entered',
              items: [
                {
                  can_view_historical: true,
                  can_view_latest: false,
                  event_date: '2026-11-05',
                  id: 'opaque-event-job',
                  reason: '历史基线进入',
                  status: 'confirmed_closed',
                  title: 'Go 平台工程师',
                },
              ],
              matched_total: 1,
              mode: 'event',
              page: { number: 1, page_size: 20, total: 1 },
              period_end: '2026-11-05',
              period_start: '2026-11-01',
              reason_counts: { 历史基线进入: 1 },
            }),
          )
        }
        throw new Error(`unexpected request ${path}`)
      }),
    )
    const { useJobTrendDashboard } = await import('@/lib/job-trend-dashboard')
    const dashboard = useJobTrendDashboard()
    const row = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })!
    row.appliedCondition = { ...row.condition }
    row.series = {
      curve: { dimension: 'technology', label: 'Go', value: 'go' },
      freshness: 'safe-marker',
      points: [{ count: 3, date: '2026-11-05' }],
    }
    row.status = 'ready'
    const wrapper = await mountTrendView()
    await flushPromises()

    await wrapper.get('.job-trend-view__event-table tbody button').trigger('click', {
      clientX: 500,
      clientY: 600,
    })
    await flushPromises()
    const drilldown = wrapper.get('[aria-label="趋势节点岗位"]')
    expect(drilldown.classes()).toContain('job-trend-view__drilldown--event')
    expect(drilldown.attributes('style')).toContain('--drilldown-height: 256px')
    expect(drilldown.attributes('style')).toContain('left: 120px')
    expect(drilldown.attributes('style')).toContain('top: 335px')
    expect(drilldown.get('[data-testid="event-drilldown-title"]').text()).toBe(
      '新增 · 技术关键词 · Go',
    )
    expect(drilldown.get('[data-testid="event-drilldown-period"]').text()).toBe(
      '2026-11-01 至 2026-11-05',
    )
    expect(drilldown.get('[data-testid="event-drilldown-count"]').text()).toBe(
      '周期内去重 1 个岗位',
    )
    expect(drilldown.find('[data-testid="event-drilldown-reasons"]').exists()).toBe(false)
    expect(drilldown.get('[data-testid="event-drilldown-list"]').classes()).toContain(
      'job-trend-view__drilldown-list--event',
    )
    expect(drilldown.text()).toContain('2026-11-05')
    expect(drilldown.text()).toContain('历史基线进入')
    wrapper.unmount()
  })

  it('uses the existing modified-history detail for curve-internal modifications', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              points: [{ count: 1, date: '2026-11-05' }],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/events?')) {
          return new Response(
            JSON.stringify({
              current: { start: '2026-11-01', end: '2026-11-05' },
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              previous: { start: '2026-10-01', end: '2026-10-05' },
              rows: [
                {
                  current: 1,
                  previous: { count: 0, rate: null },
                  type: 'modified',
                  year_over_year: { count: 0, rate: null },
                },
              ],
              status: 'available',
              year_over_year: { start: '2025-11-01', end: '2025-11-05' },
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/drilldown?')) {
          return new Response(
            JSON.stringify({
              event_type: 'modified',
              items: [
                {
                  can_view_historical: true,
                  can_view_latest: true,
                  event_date: '2026-11-03',
                  id: 'opaque-modified-job',
                  reason: '曲线内修改',
                  status: 'changed',
                  title: 'Go 平台工程师',
                },
              ],
              matched_total: 1,
              mode: 'event',
              page: { number: 1, page_size: 20, total: 1 },
              period_end: '2026-11-05',
              period_start: '2026-11-01',
              reason_counts: { 曲线内修改: 1 },
            }),
          )
        }
        if (
          path === '/api/v1/jobs/opaque-modified-job/history/detail?date=2026-11-03&type=modified'
        ) {
          return new Response(
            JSON.stringify({
              activity_state: 'effective',
              body: { requirements: '熟悉 Go', responsibilities: '建设平台服务' },
              comparison: {
                status: 'available',
                dimension_changes: [{ field: '技术关键词', historical: 'Go', latest: 'Go、Rust' }],
              },
              first_discovered_date: '2026-11-01',
              history_date: '2026-11-03',
              history_node_type: 'modified',
              id: 'opaque-modified-job',
              is_urgent: false,
              locations: ['上海'],
              other_job_families: [],
              projects: [],
              technology_keywords: [{ label: 'Go', value: 'go' }],
              title: 'Go 平台工程师',
            }),
          )
        }
        throw new Error(`unexpected request ${path}`)
      }),
    )
    const { useJobTrendDashboard } = await import('@/lib/job-trend-dashboard')
    const dashboard = useJobTrendDashboard()
    const row = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })!
    row.appliedCondition = { ...row.condition }
    row.series = {
      curve: { dimension: 'technology', label: 'Go', value: 'go' },
      freshness: 'safe-marker',
      points: [{ count: 1, date: '2026-11-05' }],
    }
    row.status = 'ready'
    const wrapper = await mountTrendView()
    await flushPromises()

    await wrapper
      .get('.job-trend-view__event-row--modified [data-testid="event-count-current"]')
      .trigger('click')
    await flushPromises()
    const drilldown = wrapper.get('[aria-label="趋势节点岗位"]')
    const item = drilldown.get('li')
    expect(item.text()).toContain('有改动')
    await item
      .findAll('button')
      .find((button) => button.text().includes('查看历史 JD'))!
      .trigger('click')
    await flushPromises()

    expect(wrapper.get('[aria-label="岗位详情"]').text()).toContain('变更摘要')
    expect(wrapper.get('[aria-label="岗位详情"]').text()).toContain('1 项变动')
    expect(wrapper.get('[aria-label="岗位详情"]').text()).toContain('历史 JD · 2026-11-03')
    wrapper.unmount()
  })

  it('loads events for an already activated dashboard and refreshes them on demand', async () => {
    let eventReads = 0
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'unchanged-line',
              points: [{ count: 3, date: '2026-11-05' }],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/events?')) {
          eventReads += 1
          return new Response(
            JSON.stringify({
              current: { start: '2026-11-01', end: '2026-11-05' },
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              previous: { start: '2026-10-01', end: '2026-10-05' },
              rows: [
                {
                  type: 'entered',
                  current: eventReads,
                  previous: { count: 0, rate: null },
                  year_over_year: { count: 0, rate: null },
                },
              ],
              status: 'available',
              year_over_year: { start: '2025-11-01', end: '2025-11-05' },
            }),
          )
        }
        throw new Error(`unexpected request ${path}`)
      }),
    )
    const { useJobTrendDashboard } = await import('@/lib/job-trend-dashboard')
    const dashboard = useJobTrendDashboard()
    const curve = dashboard.addCurve({
      dimension: 'technology',
      includeSecondary: false,
      operator: '',
      value: 'go',
    })!
    await dashboard.applyCurve(curve.id)
    await dashboard.activate()

    const wrapper = await mountTrendView()
    await flushPromises()
    expect(eventReads).toBe(1)
    expect(wrapper.get('.job-trend-view__event-table tbody tr').text()).toContain('1')

    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('刷新数据'))!
      .trigger('click')
    await flushPromises()
    expect(eventReads).toBe(2)
    expect(wrapper.get('.job-trend-view__event-table tbody tr').text()).toContain('2')
    wrapper.unmount()
  })

  it('opens one anchored node list without exposing transport identities and preserves it after detail close', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              points: [{ count: 1, date: '2026-11-05' }],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/drilldown?')) {
          return new Response(
            JSON.stringify({
              count_matches_expected: false,
              items: [
                {
                  can_view_historical: false,
                  can_view_latest: true,
                  id: 'opaque-jd-should-not-render',
                  status: 'current',
                  title: 'Go 平台工程师',
                },
              ],
              matched_total: 1,
              node_date: '2026-11-05',
              page: { number: 1, page_size: 20, total: 1 },
            }),
          )
        }
        if (path === '/api/v1/jobs/opaque-jd-should-not-render') {
          return new Response(
            JSON.stringify({
              activity_state: 'effective',
              body: { requirements: '熟悉 Go', responsibilities: '建设平台服务' },
              first_discovered_date: '2026-11-05',
              id: 'opaque-jd-should-not-render',
              is_urgent: false,
              locations: ['上海'],
              other_job_families: [],
              projects: [],
              technology_keywords: [],
              title: 'Go 平台工程师',
            }),
          )
        }
        throw new Error(`unexpected request ${path}`)
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
      points: [{ count: 1, date: '2026-11-05' }],
    }
    row!.status = 'ready'
    const wrapper = await mountTrendView()
    await flushPromises()

    const historyLength = window.history.length
    await wrapper.findComponent({ name: 'VChart' }).vm.$emit('click', {
      data: ['2026-11-05', 1],
      event: { event: { clientX: 400, clientY: 200 } },
      seriesName: '技术关键词 · Go',
    })
    await flushPromises()

    const drilldown = wrapper.get('[aria-label="趋势节点岗位"]')
    expect(drilldown.attributes('style')).toContain('left:')
    expect(drilldown.text()).toContain('Go 平台工程师')
    expect(drilldown.text()).toContain('仍有效')
    expect(drilldown.text()).toContain('数据已更新，请重新打开节点')
    expect(drilldown.text()).not.toContain('opaque-jd-should-not-render')
    expect(window.history.length).toBe(historyLength)

    const latestAction = drilldown
      .findAll('button')
      .find((button) => button.text().includes('查看最新'))
    expect(latestAction).toBeDefined()
    await latestAction!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[aria-label="岗位详情"]').text()).toContain('Go 平台工程师')

    await wrapper.get('[aria-label="关闭岗位详情"]').trigger('click')
    expect(wrapper.find('[aria-label="岗位详情"]').exists()).toBe(false)
    expect(wrapper.get('[aria-label="趋势节点岗位"]').text()).toContain('Go 平台工程师')

    wrapper.unmount()
  })

  it('opens a known short node at its final compact height before the list response arrives', async () => {
    let resolveDrilldown: ((response: Response) => void) | undefined
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends/drilldown?')) {
          return new Promise<Response>((resolve) => {
            resolveDrilldown = resolve
          })
        }
        throw new Error(`unexpected request ${path}`)
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

    await wrapper.findComponent({ name: 'VChart' }).vm.$emit('click', {
      data: ['2026-11-05', 3],
      seriesName: '技术关键词 · Go',
    })
    await nextTick()

    expect(resolveDrilldown).toBeDefined()
    expect(wrapper.get('[aria-label="趋势节点岗位"]').attributes('style')).toContain(
      '--drilldown-height: 368px',
    )

    resolveDrilldown!(
      new Response(
        JSON.stringify({
          items: [
            {
              can_view_historical: false,
              can_view_latest: true,
              id: 'one',
              status: 'current',
              title: '岗位一',
            },
            {
              can_view_historical: false,
              can_view_latest: true,
              id: 'two',
              status: 'current',
              title: '岗位二',
            },
            {
              can_view_historical: false,
              can_view_latest: true,
              id: 'three',
              status: 'current',
              title: '岗位三',
            },
          ],
          matched_total: 3,
          node_date: '2026-11-05',
          page: { number: 1, page_size: 20, total: 3 },
        }),
      ),
    )
    await flushPromises()
    expect(wrapper.get('[aria-label="趋势节点岗位"]').attributes('style')).toContain(
      '--drilldown-height: 368px',
    )

    wrapper.unmount()
  })

  it('renders every permitted historical-node status and action without exposing transport IDs', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              points: [{ count: 3, date: '2026-11-05' }],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/drilldown?')) {
          return new Response(
            JSON.stringify({
              items: [
                {
                  can_view_historical: false,
                  can_view_latest: true,
                  id: 'opaque-current-id',
                  status: 'current',
                  title: '仍有效岗位',
                },
                {
                  can_view_historical: true,
                  can_view_latest: true,
                  id: 'opaque-changed-id',
                  status: 'changed',
                  title: '有改动岗位',
                },
                {
                  can_view_historical: true,
                  can_view_latest: true,
                  id: 'opaque-closed-id',
                  status: 'confirmed_closed',
                  title: '确认下线岗位',
                },
              ],
              matched_total: 3,
              node_date: '2026-11-05',
              page: { number: 1, page_size: 20, total: 3 },
            }),
          )
        }
        if (path === '/api/v1/jobs/opaque-closed-id/detail?date=2026-11-05&version=historical') {
          return new Response(
            JSON.stringify({
              activity_state: 'effective',
              body: { requirements: '熟悉 Go', responsibilities: '建设平台服务' },
              first_discovered_date: '2026-11-05',
              id: 'opaque-closed-id',
              is_urgent: false,
              locations: ['上海'],
              other_job_families: [],
              projects: [],
              technology_keywords: [],
              title: '确认下线岗位',
            }),
          )
        }
        throw new Error(`unexpected request ${path}`)
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

    await wrapper.findComponent({ name: 'VChart' }).vm.$emit('click', {
      data: ['2026-11-05', 3],
      seriesName: '技术关键词 · Go',
    })
    await flushPromises()

    const items = wrapper.get('[aria-label="趋势节点岗位"]').findAll('li')
    expect(items).toHaveLength(3)
    expect(wrapper.get('[aria-label="趋势节点岗位"]').attributes('style')).toContain(
      '--drilldown-height: 368px',
    )
    expect(items[0]!.text()).toContain('仍有效')
    expect(items[0]!.text()).toContain('查看最新')
    expect(items[0]!.text()).not.toContain('查看历史')
    expect(items[1]!.text()).toContain('有改动')
    expect(items[1]!.text()).toContain('查看历史 JD')
    expect(items[1]!.text()).toContain('查看最新 JD')
    expect(items[2]!.text()).toContain('确认下线')
    expect(items[2]!.text()).toContain('查看历史 JD')
    expect(items[2]!.text()).not.toContain('查看最新')
    expect(wrapper.text()).not.toContain('opaque-current-id')
    expect(wrapper.text()).not.toContain('opaque-changed-id')
    expect(wrapper.text()).not.toContain('opaque-closed-id')

    const historicalAction = items[2]!
      .findAll('button')
      .find((button) => button.text().includes('查看历史 JD'))
    expect(historicalAction).toBeDefined()
    await historicalAction!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[aria-label="岗位详情"]').text()).toContain('节点当日：仍有效')
    expect(wrapper.get('[aria-label="岗位详情"]').text()).toContain('当前：确认下线')

    wrapper.unmount()
  })

  it('uses a 390px sheet for a trusted zero and restores background scrolling when it closes', async () => {
    const originalWidth = window.innerWidth
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptions))
        if (path.startsWith('/api/v1/job-trends?')) {
          return new Response(
            JSON.stringify({
              curve: { dimension: 'technology', label: 'Go', value: 'go' },
              freshness: 'safe-marker',
              points: [{ count: 0, date: '2026-11-05' }],
            }),
          )
        }
        if (path.startsWith('/api/v1/job-trends/drilldown?')) {
          return new Response(
            JSON.stringify({
              count_matches_expected: true,
              items: [],
              matched_total: 0,
              node_date: '2026-11-05',
              page: { number: 1, page_size: 20, total: 0 },
            }),
          )
        }
        throw new Error(`unexpected request ${path}`)
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
      points: [{ count: 0, date: '2026-11-05' }],
    }
    row!.status = 'ready'
    const wrapper = await mountTrendView()
    await flushPromises()

    await wrapper.findComponent({ name: 'VChart' }).vm.$emit('click', {
      data: ['2026-11-05', 0],
      seriesName: '技术关键词 · Go',
    })
    await flushPromises()
    await nextTick()

    expect(wrapper.get('[aria-label="趋势节点岗位"]').text()).toContain('该日没有匹配岗位')
    expect(document.body.style.overflow).toBe('hidden')

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(wrapper.find('[aria-label="趋势节点岗位"]').exists()).toBe(false)
    expect(document.body.style.overflow).toBe('')

    wrapper.unmount()
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: originalWidth })
  })
})
