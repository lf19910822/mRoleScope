import { flushPromises, mount } from '@vue/test-utils'
import { NSelect } from 'naive-ui'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'

import JobListView from '../JobListView.vue'
import { createJDRouter } from '@/router'

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
  technology_keywords: [{ value: 'go', label: 'Go' }],
  urgent_options: [{ value: 'true', label: '紧急招聘' }],
}

const emptyListResponse = { items: [], page: { number: 1, page_size: 20, total: 0 } }

function latestRequest(requests: string[]): URL {
  return new URL(requests[requests.length - 1] ?? '', 'https://jd-observatory.invalid')
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('JD list filters', () => {
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
    expect(filteredRequest.searchParams.getAll('job_family')).toEqual([
      'backend',
      'ai_application',
    ])
    expect(filteredRequest.searchParams.getAll('project')).toEqual(['genshin_impact', 'star_rail'])
    expect(filteredRequest.searchParams.getAll('technology')).toEqual(['go', 'python'])
    expect(filteredRequest.searchParams.getAll('activity_state')).toEqual(['effective'])
    expect(
      wrapper.findAllComponents(NSelect).some((select) => select.props('filterable') === true),
    ).toBe(true)
    expect(wrapper.text()).toContain('岗位族')
    expect(wrapper.text()).not.toContain('其他命中岗位族')

    wrapper.unmount()
  })

  it('keeps sorting kind and sorting direction independent while preserving priority', async () => {
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
      query: { sort: ['experience_desc', 'first_discovered_desc'] },
    })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.get('[data-testid="sort-kind-1"]').text()).toContain('最低工作年限')
    expect(wrapper.get('[data-testid="sort-direction-1"]').text()).toContain('从高到低')
    expect(wrapper.get('[data-testid="sort-kind-2"]').text()).toContain('首次发现日期')
    expect(wrapper.get('[data-testid="sort-direction-2"]').text()).toContain('由新到旧')
    await wrapper.get('[data-testid="move-sort-rule-up-2"]').trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === '应用筛选')?.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.sort).toEqual(['first_discovered_desc', 'experience_desc'])

    wrapper.unmount()
  })

  it('uses a fixed zero-through-nine experience threshold and omits it when unrestricted', async () => {
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

    expect(wrapper.get('[data-testid="minimum-experience-0"]').attributes('aria-checked')).toBe('false')
    expect(wrapper.find('[data-testid="minimum-experience-9"]').exists()).toBe(true)
    await wrapper.get('[data-testid="minimum-experience-0"]').trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === '应用筛选')?.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.minimum_experience_year).toBe('0')

    await wrapper.get('[data-testid="minimum-experience-any"]').trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === '应用筛选')?.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.minimum_experience_year).toBeUndefined()

    wrapper.unmount()
  })
})
