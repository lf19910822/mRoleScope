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

    expect(wrapper.get('[data-testid="curve-card-trend-1"]').attributes('style')).toContain(
      '#0072B2',
    )
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

  it('opens one anchored node list without exposing transport identities and preserves it after detail close', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options') return new Response(JSON.stringify(filterOptions))
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
        if (path === '/api/v1/jobs/filter-options') return new Response(JSON.stringify(filterOptions))
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
            { can_view_historical: false, can_view_latest: true, id: 'one', status: 'current', title: '岗位一' },
            { can_view_historical: false, can_view_latest: true, id: 'two', status: 'current', title: '岗位二' },
            { can_view_historical: false, can_view_latest: true, id: 'three', status: 'current', title: '岗位三' },
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
        if (path === '/api/v1/jobs/filter-options') return new Response(JSON.stringify(filterOptions))
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
                  can_view_latest: false,
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
        if (path === '/api/v1/jobs/filter-options') return new Response(JSON.stringify(filterOptions))
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
