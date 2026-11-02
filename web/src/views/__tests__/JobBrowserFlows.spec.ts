import { flushPromises, mount } from '@vue/test-utils'
import { NPagination } from 'naive-ui'
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
  it('keeps the last list visible through replacement reads and ignores a late stale response', async () => {
    type Deferred = { resolve: (response: Response) => void; reject: (reason: Error) => void }
    const pending = new Map<string, Deferred>()
    const jobResponse = (id: string, title: string) =>
      new Response(
        JSON.stringify({
          items: [
            {
              activity_state: 'effective',
              first_discovered_date: '2026-10-30',
              id,
              is_urgent: false,
              locations: ['上海'],
              other_job_families: [],
              projects: [],
              technology_keywords: [],
              title,
            },
          ],
          page: { number: 1, page_size: 20, total: 1 },
        }),
      )
    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        const path = String(input)
        if (path === '/api/v1/jobs/filter-options') {
          return Promise.resolve(new Response(JSON.stringify(options)))
        }
        const title = new URL(path, 'https://jd-observatory.invalid').searchParams.get('title')
        if (title === null) {
          return Promise.resolve(jobResponse('first', 'First job'))
        }
        return new Promise<Response>((resolve, reject) => pending.set(title, { resolve, reject }))
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()
    expect(wrapper.text()).toContain('First job')

    await router.push({ name: 'jobs', query: { title: 'older' } })
    await flushPromises()
    await router.push({ name: 'jobs', query: { title: 'newer' } })
    await flushPromises()
    expect(wrapper.text()).toContain('First job')

    pending.get('older')?.resolve(jobResponse('older', 'Stale job'))
    await flushPromises()
    expect(wrapper.text()).toContain('First job')
    expect(wrapper.text()).not.toContain('Stale job')

    pending.get('newer')?.resolve(jobResponse('newer', 'Newest job'))
    await flushPromises()
    expect(wrapper.text()).toContain('Newest job')

    await router.push({ name: 'jobs', query: { title: 'failed' } })
    await flushPromises()
    pending.get('failed')?.reject(new Error('network unavailable'))
    await flushPromises()
    expect(wrapper.text()).toContain('Newest job')
    expect(wrapper.text()).toContain('network unavailable')

    wrapper.unmount()
  })

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

  it('uses numbered pages, page_size, and a quick jump without a cursor', async () => {
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
        const pageSize = Number(request.searchParams.get('page_size') ?? '20')
        return new Response(
          JSON.stringify({ items: [], page: { number: page, page_size: pageSize, total: 167 } }),
        )
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    const pagination = wrapper.getComponent(NPagination)
    expect(pagination.props('showQuickJumper')).toBe(true)
    expect(pagination.props('showSizePicker')).toBe(true)
    await pagination.vm.$emit('update:page', 4)
    await flushPromises()

    let request = new URL(requests[requests.length - 1] ?? '', 'https://jd-observatory.invalid')
    expect(request.searchParams.get('page')).toBe('4')
    expect(request.searchParams.get('cursor')).toBeNull()
    expect(router.currentRoute.value.query.page).toBe('4')

    const onPageSizeChange = pagination.props('onUpdate:pageSize') as (pageSize: number) => void
    onPageSizeChange(50)
    await flushPromises()
    request = new URL(requests[requests.length - 1] ?? '', 'https://jd-observatory.invalid')
    expect(request.searchParams.get('page')).toBe('1')
    expect(request.searchParams.get('page_size')).toBe('50')
    expect(request.searchParams.get('cursor')).toBeNull()

    wrapper.unmount()
  })
})
