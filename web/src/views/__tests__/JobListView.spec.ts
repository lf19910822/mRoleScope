import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { NSelect } from 'naive-ui'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { createMemoryHistory } from 'vue-router'

import JobListView from '../JobListView.vue'
import { createJDRouter } from '@/router'
import FilterDatePicker from '@/components/FilterDatePicker.vue'

const filterOptionsResponse = {
  activity_states: [
    { value: 'effective', label: '仍有效' },
    { value: 'confirmed_closed', label: '已确认下线' },
    { value: 'all', label: '全部' },
  ],
  education_levels: [{ value: 'bachelor', label: '本科' }],
  experience_years: [{ value: '3', label: '3 年' }],
  job_families: [
    { value: 'backend', label: '后端' },
    { value: 'ai_application', label: 'AI 应用' },
  ],
  locations: [{ value: '上海', label: '上海' }],
  main_job_families: [{ value: 'backend', label: '后端' }],
  other_job_families: [],
  projects: [{ value: 'genshin_impact', label: '原神' }],
  technology_keywords: [
    {
      value: 'go',
      label: 'Go',
      category: { value: 'programming-languages', label: '编程语言' },
    },
  ],
  urgent_options: [{ value: 'true', label: '紧急招聘' }],
}

const emptyListResponse = { items: [], page: { number: 1, page_size: 20, total: 0 } }

const queryCanvasListResponse = {
  items: [
    {
      activity_state: 'effective',
      education: { explicit_requirement: true, lower_bound: 'master' },
      experience: { explicit_requirement: true, minimum_years: 5 },
      first_discovered_date: '2026-10-02',
      last_modified_date: '2026-11-03',
      id: 'query-canvas-role',
      is_urgent: false,
      locations: ['北京'],
      main_job_family: { label: '前端', value: 'frontend' },
      other_job_families: [{ label: '运维', value: 'operations_sre' }],
      projects: [],
      technology_keywords: [],
      title: '平台工程师',
    },
  ],
  page: { number: 1, page_size: 20, total: 1 },
}

const denseQueryCanvasListResponse = {
  items: [
    {
      activity_state: 'effective',
      education: { explicit_requirement: true, lower_bound: 'master' },
      experience: { explicit_requirement: true, minimum_years: 5 },
      first_discovered_date: '2026-10-02',
      id: 'dense-query-canvas-role',
      is_urgent: true,
      locations: ['北京', '上海', '广州'],
      main_job_family: { label: '前端', value: 'frontend' },
      other_job_families: [{ label: '运维', value: 'operations_sre' }],
      projects: [{ label: '预研项目', value: 'research' }],
      technology_keywords: [
        'APISIX',
        'Docker',
        'Envoy',
        'HTTP',
        'Kong',
        'Kubernetes',
        'MySQL',
        'Redis',
        'WebSocket',
        'gRPC',
      ].map((label) => ({ label, value: label.toLocaleLowerCase() })),
      title: '面向多城市内容交付与复杂依赖治理的资深平台研发工程师（预研项目）',
    },
  ],
  page: { number: 1, page_size: 20, total: 1 },
}

function latestRequest(requests: string[]): URL {
  return new URL(requests[requests.length - 1] ?? '', 'https://jd-observatory.invalid')
}

function dispatchPointer(
  target: Element,
  type: 'pointerenter' | 'pointerleave',
  pointerType: string,
): void {
  const event = new Event(type)
  Object.defineProperty(event, 'pointerType', { value: pointerType })
  target.dispatchEvent(event)
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('JD list filters', () => {
  it('keeps desktop rails available while the browser scrolls the full role collection', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/views/JobListView.vue'), 'utf8')
    const pageRule = source.match(/\.job-list-page \{(?<declarations>[\s\S]*?)\n\}/u)?.groups
      ?.declarations
    const canvasRule = source.match(/\.query-canvas \{(?<declarations>[\s\S]*?)\n\}/u)?.groups
      ?.declarations
    const resultRule = source.match(
      /\.query-canvas > \.list-result \{(?<declarations>[\s\S]*?)\n\}/u,
    )?.groups?.declarations
    const consoleRule = source.match(/\.query-console \{(?<declarations>[\s\S]*?)\n\}/u)?.groups
      ?.declarations
    const resultHeadingRule = source.match(
      /\.query-canvas \.result-heading \{(?<declarations>[\s\S]*?)\n\}/u,
    )?.groups?.declarations

    expect(pageRule).toMatch(/display:\s*block;/u)
    expect(pageRule).toMatch(/overflow:\s*visible;/u)
    expect(canvasRule).not.toMatch(/min-height:\s*0;/u)
    expect(resultRule).toMatch(/height:\s*auto;/u)
    expect(resultRule).toMatch(/overflow:\s*visible;/u)
    expect(resultRule).not.toMatch(/overflow-y:\s*auto;/u)
    expect(consoleRule).toMatch(/max-height:\s*calc\(100dvh - 48px\);/u)
    expect(consoleRule).toMatch(/overflow-y:\s*auto;/u)
    expect(consoleRule).toMatch(/position:\s*sticky;/u)
    expect(consoleRule).toMatch(/top:\s*24px;/u)
    expect(resultHeadingRule).toMatch(/background:\s*transparent;/u)
    expect(resultHeadingRule).toMatch(/border:\s*0;/u)
    expect(resultHeadingRule).toMatch(/padding:\s*0 0 12px;/u)
  })

  it('pins the wide-desktop query rail to the top-right viewport edge and reserves its width', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/views/JobListView.vue'), 'utf8')
    const wideStyles = source.slice(source.lastIndexOf('@media (min-width: 1280px)'))

    expect(wideStyles).toMatch(
      /\.job-list-page \{[\s\S]*?margin-left:\s*0;[\s\S]*?width:\s*calc\(100% - 460px\);/u,
    )
    expect(wideStyles).toMatch(
      /\.query-canvas \{[\s\S]*?grid-template-columns:\s*minmax\(0, 1fr\);/u,
    )
    expect(wideStyles).toMatch(
      /\.query-console \{[\s\S]*?height:\s*100dvh;[\s\S]*?max-height:\s*none;[\s\S]*?position:\s*fixed;[\s\S]*?right:\s*0;[\s\S]*?top:\s*0;[\s\S]*?width:\s*428px;/u,
    )
  })

  it('uses one uninterrupted role column and pairs only concise query controls', async () => {
    const source = readFileSync(resolve(process.cwd(), 'src/views/JobListView.vue'), 'utf8')
    const listRule = source.match(/\.query-canvas \.job-list \{(?<declarations>[\s\S]*?)\n\}/u)
      ?.groups?.declarations

    expect(listRule).toMatch(/grid-template-columns:\s*1fr;/u)
    expect(source).toMatch(
      /<div class="filter-field-pair" data-testid="filter-pair-qualifications">[\s\S]*?学历要求[\s\S]*?最低工作年限[\s\S]*?<\/div>/u,
    )
    expect(source).toMatch(
      /<div class="filter-field-pair" data-testid="filter-pair-activity">[\s\S]*?活跃状态[\s\S]*?紧急标记[\s\S]*?<\/div>/u,
    )
    expect(source).toMatch(
      /\.query-console \.filter-field-pair \{[\s\S]*?display:\s*grid;[\s\S]*?grid-template-columns:\s*repeat\(2, minmax\(0, 1fr\)\);/u,
    )

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        return new Response(JSON.stringify(emptyListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.get('[data-testid="filter-pair-qualifications"]').text()).toContain('学历要求')
    expect(wrapper.get('[data-testid="filter-pair-qualifications"]').text()).toContain(
      '最低工作年限',
    )
    expect(wrapper.get('[data-testid="filter-pair-activity"]').text()).toContain('活跃状态')
    expect(wrapper.get('[data-testid="filter-pair-activity"]').text()).toContain('紧急标记')
    wrapper.unmount()
  })

  it('keeps the one-column document flow visible while narrow query controls use a sheet', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/views/JobListView.vue'), 'utf8')
    const narrowStyles = source.slice(source.lastIndexOf('@media (max-width: 760px)'))

    expect(narrowStyles).toMatch(
      /\.job-list-page \{[\s\S]*?display:\s*block;[\s\S]*?height:\s*auto;[\s\S]*?overflow:\s*visible;/u,
    )
    expect(narrowStyles).toMatch(
      /\.query-canvas \{[\s\S]*?display:\s*flex;[\s\S]*?flex-direction:\s*column;/u,
    )
    expect(narrowStyles).toMatch(/\.query-console \{[\s\S]*?display:\s*none;/u)
    expect(narrowStyles).toMatch(
      /\.query-console-host--mobile-open \.query-console \{[\s\S]*?display:\s*block;[\s\S]*?height:\s*auto;[\s\S]*?overflow-y:\s*auto;[\s\S]*?position:\s*static;/u,
    )
    expect(narrowStyles).toMatch(
      /\.query-canvas > \.list-result \{[\s\S]*?height:\s*auto;[\s\S]*?overflow:\s*visible;/u,
    )
    expect(narrowStyles).toMatch(
      /\.query-canvas \.job-list \{[\s\S]*?grid-template-columns:\s*1fr;/u,
    )
    expect(narrowStyles).toMatch(
      /\.query-console \.filter-field-pair \{[\s\S]*?grid-template-columns:\s*1fr;/u,
    )
    expect(narrowStyles).toMatch(
      /\.query-canvas \.pagination :deep\(\.n-pagination\) \{[\s\S]*?flex-wrap:\s*wrap;[\s\S]*?justify-content:\s*center;/u,
    )
    expect(narrowStyles).toMatch(
      /\.query-canvas \.result-heading \{[\s\S]*?flex-direction:\s*row;/u,
    )
  })

  it('presents a three-band compact record and keeps empty technology data absent', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        return new Response(JSON.stringify(queryCanvasListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.get('[data-testid="query-canvas"]')).toBeTruthy()
    expect(wrapper.get('[data-testid="query-console"]')).toBeTruthy()
    const roleCard = wrapper.get('[data-testid="query-canvas-query-canvas-role"]')
    expect(roleCard.get('.job-card__title-icon')).toBeTruthy()
    expect(roleCard.get('.job-card__headline h3').text()).toBe('平台工程师')
    expect(roleCard.get('.job-card__headline-family').text()).toContain('前端')
    expect(roleCard.text()).toContain('北京')
    expect(roleCard.text()).toContain('工作地区')
    expect(roleCard.text()).not.toContain('主岗位族')
    expect(roleCard.text()).toContain('工作年限')
    expect(roleCard.text()).toContain('最低 5 年')
    expect(roleCard.text()).toContain('学历要求')
    expect(roleCard.text()).toContain('硕士')
    expect(roleCard.text()).toContain('首次发现')
    expect(roleCard.text()).toContain('最近修改日期')
    expect(roleCard.text()).toContain('2026.11.03')
    expect(roleCard.text()).toContain('2026.10.02')
    const facts = roleCard.get('.job-card__facts')
    expect(facts.findAll(':scope > div')).toHaveLength(5)
    expect(facts.classes()).toContain('job-card__facts--terminal')
    expect(facts.text()).not.toContain('上线状态')
    expect(facts.text()).not.toContain('紧急标记')
    const statePills = roleCard.get('.job-card__states')
    expect(statePills.text()).toContain('仍有效')
    expect(statePills.text()).toContain('常规')
    expect(roleCard.find('.job-card__technologies').exists()).toBe(false)
    expect(roleCard.get('.job-card__supplementary').classes()).toContain(
      'job-card__supplementary--after-facts',
    )
    expect(roleCard.get('.job-card__supplementary').text()).toContain('运维')
    expect(roleCard.get('.job-card__supplementary').text()).toContain('副岗位族')
    expect(roleCard.get('.job-card__supplementary').text()).not.toContain('其他命中岗位族')

    const source = readFileSync(resolve(process.cwd(), 'src/views/JobListView.vue'), 'utf8')
    expect(source).toMatch(/\.query-canvas \.job-card \{[\s\S]*?padding:\s*14px 18px;/u)
    expect(source).toMatch(
      /\.query-canvas \.job-card__facts \{[\s\S]*?display:\s*flex;[\s\S]*?flex-wrap:\s*wrap;/u,
    )
    expect(source).toMatch(
      /\.query-canvas \.job-card__facts dd \{[\s\S]*?overflow:\s*visible;[\s\S]*?white-space:\s*normal;/u,
    )
    expect(source).toMatch(
      /\.query-canvas \.job-card__facts--terminal \{[\s\S]*?border-bottom:\s*0;[\s\S]*?padding-bottom:\s*0;/u,
    )
    expect(source).toMatch(
      /\.query-canvas \.job-card__supplementary--after-facts \{[\s\S]*?margin-top:\s*0;/u,
    )
    expect(source).toMatch(/\.query-canvas \.job-card h3 \{[\s\S]*?overflow-wrap:\s*anywhere;/u)
    expect(source).not.toContain('-webkit-line-clamp')
    expect(source).toMatch(/@media \(prefers-color-scheme: dark\)/u)
    expect(source).toMatch(
      /\.query-canvas \.job-card__project-mention \{[\s\S]*?background:\s*#fff35c;[\s\S]*?font-weight:\s*800;/u,
    )
    expect(source).toMatch(
      /@media \(prefers-color-scheme: dark\) \{[\s\S]*?\.query-canvas \.job-card__project-mention \{[\s\S]*?background:\s*#ffe45c;/u,
    )

    wrapper.unmount()
  })

  it('keeps long titles, multiple cities, and every technology keyword visible', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        return new Response(JSON.stringify(denseQueryCanvasListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    const roleCard = wrapper.get('[data-testid="query-canvas-dense-query-canvas-role"]')
    expect(roleCard.get('.job-card__facts').text()).toContain('最近修改日期-')
    expect(roleCard.get('.job-card__headline h3').text()).toBe(
      '面向多城市内容交付与复杂依赖治理的资深平台研发工程师（预研项目）',
    )
    expect(roleCard.get('.job-card__project-mention').text()).toBe('预研项目')
    expect(roleCard.get('.job-card__facts').text()).toContain('北京 · 上海 · 广州')
    const technologies = roleCard.get('.job-card__technologies')
    expect(technologies.findAll('.tag')).toHaveLength(10)
    expect(technologies.text()).toContain('APISIX')
    expect(technologies.text()).toContain('gRPC')
    expect(technologies.text()).not.toContain('+10')
    expect(roleCard.find('.job-card__headline-projects').exists()).toBe(false)
    expect(roleCard.get('.job-card__supplementary').text()).not.toContain('预研项目')
    expect(roleCard.get('.job-card__supplementary').text()).toContain('运维')

    wrapper.unmount()
  })

  it('temporarily magnifies only a mouse-focused query-canvas role card', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        return new Response(JSON.stringify(queryCanvasListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    const card = wrapper.get('[data-testid="query-canvas-query-canvas-role"]')
    expect(card.attributes('data-motion-state')).toBe('resting')

    dispatchPointer(card.element, 'pointerenter', 'mouse')
    await nextTick()
    expect(card.attributes('data-motion-state')).toBe('focused')

    dispatchPointer(card.element, 'pointerleave', 'mouse')
    await nextTick()
    expect(card.attributes('data-motion-state')).toBe('resting')

    dispatchPointer(card.element, 'pointerenter', 'touch')
    await nextTick()
    expect(card.attributes('data-motion-state')).toBe('resting')

    const source = readFileSync(resolve(process.cwd(), 'src/views/JobListView.vue'), 'utf8')
    const restingRule = source.match(/\.query-canvas \.job-card \{(?<declarations>[\s\S]*?)\n\}/u)
      ?.groups?.declarations
    expect(restingRule).toBeDefined()
    expect(restingRule).not.toMatch(/^\s*(?:transform|will-change)\s*:/mu)
    expect(restingRule).not.toContain('translateZ')
    expect(source).toMatch(
      /\.query-canvas \.job-card--focused \{[\s\S]*?transform:\s*scale\(1\.15\);[\s\S]*?z-index:\s*10;/u,
    )
    expect(restingRule).toMatch(/transform 360ms/u)

    wrapper.unmount()
  })

  it('reserves the list reading area with a matching skeleton before its first result', async () => {
    let resolveList: (response: Response) => void = () => undefined
    const pendingList = new Promise<Response>((resolve) => {
      resolveList = resolve
    })
    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return Promise.resolve(new Response(JSON.stringify(filterOptionsResponse)))
        }
        return pendingList
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })

    const loadingRegion = wrapper.get('[role="status"]')
    expect(loadingRegion.attributes('aria-busy')).toBe('true')
    expect(loadingRegion.text()).toContain('正在读取岗位列表')
    expect(wrapper.find('.result-heading').exists()).toBe(false)

    resolveList(new Response(JSON.stringify(emptyListResponse)))
    await flushPromises()

    expect(wrapper.find('[role="status"]').exists()).toBe(false)
    expect(wrapper.find('.result-heading').exists()).toBe(true)
    wrapper.unmount()
  })

  it('uses the active default and carries unified job-family unions with cross-field intersections into API requests', async () => {
    const requests: string[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        requests.push(path)
        return new Response(JSON.stringify(emptyListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    const defaultRequest = latestRequest(requests)
    expect(defaultRequest.searchParams.getAll('activity_state')).toEqual(['effective'])

    await router.push({
      name: 'jobs',
      query: {
        activity_state: 'effective',
        job_family: ['backend', 'ai_application'],
        project: ['genshin_impact', 'star_rail'],
        technology: ['go', 'python'],
      },
    })
    await flushPromises()

    const filteredRequest = latestRequest(requests)
    expect(filteredRequest.searchParams.getAll('job_family')).toEqual(['backend', 'ai_application'])
    expect(filteredRequest.searchParams.getAll('project')).toEqual(['genshin_impact', 'star_rail'])
    expect(filteredRequest.searchParams.getAll('technology')).toEqual(['go', 'python'])
    expect(filteredRequest.searchParams.getAll('activity_state')).toEqual(['effective'])
    expect(
      wrapper.findAllComponents(NSelect).some((select) => select.props('filterable') === true),
    ).toBe(true)
    expect(
      wrapper.findAllComponents(NSelect).every((select) => {
        const menuProps = select.props('menuProps') as { class?: string } | undefined
        return menuProps?.class === 'query-console-select-menu'
      }),
    ).toBe(true)
    expect(wrapper.text()).toContain('岗位族')
    expect(wrapper.text()).not.toContain('其他命中岗位族')

    wrapper.unmount()
  })

  it('makes an applied query visible beside results and removes one condition without resetting others', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        return new Response(JSON.stringify(queryCanvasListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({
      name: 'jobs',
      query: {
        education: 'bachelor',
        job_family: 'backend',
        minimum_experience_year: '3',
        sort: 'experience_desc',
        technology: 'go',
      },
    })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    const summary = wrapper.get('[data-testid="applied-query-summary"]')
    expect(summary.text()).toContain('岗位族：后端')
    expect(summary.text()).toContain('技术关键词：Go')
    expect(summary.text()).toContain('学历要求：本科')
    expect(summary.text()).toContain('最低工作年限：3 年及以上')
    expect(summary.text()).toContain('排序：最低工作年限（从高到低）')

    await wrapper.get('[aria-label="移除技术关键词：Go"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.technology).toBeUndefined()
    expect(router.currentRoute.value.query.job_family).toBe('backend')
    expect(router.currentRoute.value.query.education).toBe('bachelor')
    expect(router.currentRoute.value.query.minimum_experience_year).toBe('3')

    wrapper.unmount()
  })

  it('opens and closes the mobile filter sheet without duplicating the query controls', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        return new Response(JSON.stringify(queryCanvasListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    const trigger = wrapper.get('[data-testid="open-mobile-filters"]')
    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(wrapper.find('[data-testid="mobile-filter-sheet"]').exists()).toBe(false)

    await trigger.trigger('click')
    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(
      wrapper.get('[data-testid="mobile-filter-sheet"]').get('[data-testid="query-console"]'),
    ).toBeTruthy()

    await wrapper.get('[data-testid="close-mobile-filters"]').trigger('click')
    expect(wrapper.find('[data-testid="mobile-filter-sheet"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('keeps contextual filters and sorting folded until the current query needs them', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        return new Response(JSON.stringify(queryCanvasListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    const advanced = wrapper.get('[data-testid="advanced-filter-disclosure"]')
    const sorting = wrapper.get('[data-testid="sort-disclosure"]')
    expect((advanced.element as HTMLDetailsElement).open).toBe(false)
    expect((sorting.element as HTMLDetailsElement).open).toBe(false)
    expect(wrapper.get('.filter-actions').text()).toContain('应用筛选')

    await router.push({
      name: 'jobs',
      query: { location: '上海', project: 'genshin_impact', sort: 'experience_desc' },
    })
    await flushPromises()

    expect((advanced.element as HTMLDetailsElement).open).toBe(true)
    expect((sorting.element as HTMLDetailsElement).open).toBe(true)
    wrapper.unmount()
  })

  it('uses a compact workbench context instead of a decorative landing-page masthead', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        return new Response(JSON.stringify(queryCanvasListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('岗位清单')
    expect(wrapper.text()).not.toContain('JD OBSERVATORY')
    expect(wrapper.find('.tide-mark').exists()).toBe(false)

    wrapper.unmount()
  })

  it('keeps console actions reachable and resting record metadata readable', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/views/JobListView.vue'), 'utf8')

    expect(source).toMatch(
      /\.query-console \.filter-actions \{[\s\S]*?bottom:\s*0;[\s\S]*?position:\s*sticky;/u,
    )
    expect(source).toMatch(/\.query-canvas \.job-card__facts dd \{[\s\S]*?font-size:\s*0\.84rem;/u)
    expect(source).toMatch(/\.query-canvas \.tag \{[\s\S]*?font-size:\s*0\.74rem;/u)
    expect(source).toMatch(/\.query-canvas \.result-heading::before \{[\s\S]*?content:\s*'';/u)
    expect(source).not.toContain("content: '⌁'")
  })

  it('keeps four independent sorting rules in priority order and does not offer a fifth', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        return new Response(JSON.stringify(emptyListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({
      name: 'jobs',
      query: {
        sort: ['experience_desc', 'first_discovered_desc', 'education_desc', 'urgent_asc'],
      },
    })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.get('[data-testid="sort-kind-1"]').text()).toContain('最低工作年限')
    expect(
      wrapper
        .get('[data-testid="sort-kind-1"]')
        .findComponent(NSelect)
        .props('consistentMenuWidth'),
    ).toBe(false)
    expect(wrapper.get('[data-testid="sort-direction-1"]').text()).toContain('从高到低')
    expect(
      wrapper
        .get('[data-testid="sort-direction-1"]')
        .findComponent(NSelect)
        .props('consistentMenuWidth'),
    ).toBe(false)
    expect(wrapper.get('[data-testid="sort-kind-2"]').text()).toContain('首次发现日期')
    expect(wrapper.get('[data-testid="sort-direction-2"]').text()).toContain('由新到旧')
    expect(wrapper.get('[data-testid="sort-kind-4"]').text()).toContain('紧急标记')
    expect(wrapper.find('[data-testid="sort-rule-add"]').exists()).toBe(false)
    await wrapper.get('[data-testid="move-sort-rule-up-2"]').trigger('click')
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '应用筛选')
      ?.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.sort).toEqual([
      'first_discovered_desc',
      'experience_desc',
      'education_desc',
      'urgent_asc',
    ])

    wrapper.unmount()
  })

  it('shows the last-modified sort selected from a shared URL', async () => {
    const requests: string[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        requests.push(String(input))
        if (String(input) === '/api/v1/jobs/filter-options')
          return new Response(JSON.stringify(filterOptionsResponse))
        return new Response(JSON.stringify(emptyListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs', query: { sort: 'last_modified_desc' } })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()
    expect(wrapper.get('[data-testid="sort-kind-1"]').text()).toContain('最近修改日期')
    expect(wrapper.get('[data-testid="sort-direction-1"]').text()).toContain('由新到旧')
    expect(requests.some((request) => request.includes('sort=last_modified_desc'))).toBe(true)
    wrapper.unmount()
  })

  it('uses one minimum-experience select with one-through-nine values and omits it when unrestricted', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        return new Response(JSON.stringify(emptyListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    const threshold = wrapper
      .findAllComponents(NSelect)
      .find((select) => select.attributes('data-testid') === 'minimum-experience-select')
    if (threshold === undefined) {
      throw new Error('minimum experience selector was not rendered')
    }
    const options = threshold.props('options') as Array<{ label: string; value: number }>
    expect(options).toEqual([
      { label: '1 年及以上', value: 1 },
      { label: '2 年及以上', value: 2 },
      { label: '3 年及以上', value: 3 },
      { label: '4 年及以上', value: 4 },
      { label: '5 年及以上', value: 5 },
      { label: '6 年及以上', value: 6 },
      { label: '7 年及以上', value: 7 },
      { label: '8 年及以上', value: 8 },
      { label: '9 年及以上', value: 9 },
    ])
    expect(wrapper.text()).not.toContain('同类条件取并集')
    expect(wrapper.text()).not.toContain('仅筛选已明确最低年限的岗位')
    await threshold.vm.$emit('update:value', 5)
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '应用筛选')
      ?.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.minimum_experience_year).toBe('5')

    await threshold.vm.$emit('update:value', null)
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '应用筛选')
      ?.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.minimum_experience_year).toBeUndefined()

    wrapper.unmount()
  })

  it('uses the shared calendar controls and retains ISO discovery boundaries when applied', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(filterOptionsResponse))
        }
        return new Response(JSON.stringify(emptyListResponse))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    const datePickers = wrapper.findAllComponents(FilterDatePicker)
    expect(datePickers).toHaveLength(2)
    expect(wrapper.find('input[type="date"]').exists()).toBe(false)
    await datePickers[0]?.vm.$emit('update:modelValue', '2026-10-01')
    await datePickers[1]?.vm.$emit('update:modelValue', '2026-10-31')
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '应用筛选')
      ?.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.query.first_discovered_from).toBe('2026-10-01')
    expect(router.currentRoute.value.query.first_discovered_until).toBe('2026-10-31')
    wrapper.unmount()
  })
})
