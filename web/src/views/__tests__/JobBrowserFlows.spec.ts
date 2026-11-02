import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'

import JobListView from '../JobListView.vue'
import { createJDRouter } from '@/router'

const options = {
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

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('JD browser flows', () => {
  it('sends a fuzzy title query and allows effective and confirmed-closed inventory views', async () => {
    const requests: string[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(options))
        }
        requests.push(path)
        return new Response(
          JSON.stringify({ items: [], page: { number: 1, page_size: 20, total: 0 } }),
        )
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({
      name: 'jobs',
      query: { activity_state: 'effective', title: '服务端功程师' },
    })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    let request = new URL(requests[requests.length - 1] ?? '', 'https://jd-observatory.invalid')
    expect(request.searchParams.get('title')).toBe('服务端功程师')
    expect(request.searchParams.getAll('activity_state')).toEqual(['effective'])

    await router.push({ name: 'jobs', query: { activity_state: 'confirmed_closed' } })
    await flushPromises()
    request = new URL(requests[requests.length - 1] ?? '', 'https://jd-observatory.invalid')
    expect(request.searchParams.getAll('activity_state')).toEqual(['confirmed_closed'])
    expect(wrapper.text()).not.toMatch(/coverage|fallback|processing|parser|audit|provider/i)

    wrapper.unmount()
  })

  it('moves to the next stable page through the visible pagination control', async () => {
    const requests: string[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(options))
        }
        requests.push(path)
        const request = new URL(path, 'https://jd-observatory.invalid')
        const page = Number(request.searchParams.get('page') ?? '1')
        return new Response(
          JSON.stringify({ items: [], page: { number: page, page_size: 20, total: 21 } }),
        )
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    const nextButton = wrapper.findAll('button').find((button) => button.text() === '下一页')
    expect(nextButton).toBeDefined()
    await nextButton?.trigger('click')
    await flushPromises()

    const request = new URL(requests[requests.length - 1] ?? '', 'https://jd-observatory.invalid')
    expect(request.searchParams.get('page')).toBe('2')
    expect(router.currentRoute.value.query.page).toBe('2')

    wrapper.unmount()
  })
})
