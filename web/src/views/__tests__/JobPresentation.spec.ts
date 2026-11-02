import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'

import JobDetailView from '../JobDetailView.vue'
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

const job = {
  activity_state: 'effective',
  education: { explicit_requirement: true, upper_bound: 'bachelor' },
  experience: { explicit_requirement: false },
  first_discovered_date: '2026-10-30',
  id: 'jd_opaque',
  is_urgent: true,
  locations: ['上海'],
  main_job_family: { value: 'backend', label: '后端' },
  other_job_families: [{ value: 'ai_application', label: 'AI 应用' }],
  projects: [{ value: 'genshin_impact', label: '原神' }],
  technology_keywords: [{ value: 'go', label: 'Go' }],
  title: '服务端工程师',
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('JD presentation', () => {
  it('keeps a rendered detail stable while a newer route supersedes a pending detail response', async () => {
    let resolveSecond: (response: Response) => void = () => undefined
    let resolveThird: (response: Response) => void = () => undefined
    const detailResponse = (id: string, title: string) =>
      new Response(
        JSON.stringify({
          ...job,
          id,
          title,
          body: { requirements: '熟悉 Go', responsibilities: '负责服务端开发' },
        }),
      )
    vi.stubGlobal(
      'fetch',
      vi.fn((input: RequestInfo | URL) => {
        switch (String(input)) {
          case '/api/v1/jobs/first':
            return Promise.resolve(detailResponse('first', 'First detail'))
          case '/api/v1/jobs/second':
            return new Promise<Response>((resolve) => {
              resolveSecond = resolve
            })
          case '/api/v1/jobs/third':
            return new Promise<Response>((resolve) => {
              resolveThird = resolve
            })
          default:
            throw new Error(`unexpected request ${String(input)}`)
        }
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'job-detail', params: { id: 'first' } })
    await router.isReady()
    const wrapper = mount(JobDetailView, { global: { plugins: [router] } })
    await flushPromises()
    expect(wrapper.text()).toContain('First detail')

    await router.push({ name: 'job-detail', params: { id: 'second' } })
    await flushPromises()
    await router.push({ name: 'job-detail', params: { id: 'third' } })
    await flushPromises()
    expect(wrapper.text()).toContain('First detail')

    resolveSecond(detailResponse('second', 'Stale detail'))
    await flushPromises()
    expect(wrapper.text()).toContain('First detail')
    expect(wrapper.text()).not.toContain('Stale detail')

    resolveThird(detailResponse('third', 'Newest detail'))
    await flushPromises()
    expect(wrapper.text()).toContain('Newest detail')

    wrapper.unmount()
  })

  it('reserves the JD detail reading area during an initial request before showing the result', async () => {
    let resolveDetail: (response: Response) => void = () => undefined
    const pendingDetail = new Promise<Response>((resolve) => {
      resolveDetail = resolve
    })
    vi.stubGlobal(
      'fetch',
      vi.fn(() => pendingDetail),
    )

    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'job-detail', params: { id: job.id } })
    await router.isReady()
    const wrapper = mount(JobDetailView, { global: { plugins: [router] } })

    const loadingRegion = wrapper.get('[role="status"]')
    expect(loadingRegion.attributes('aria-busy')).toBe('true')
    expect(loadingRegion.text()).toContain('正在读取岗位详情')
    expect(wrapper.text()).not.toContain(job.title)

    resolveDetail(
      new Response(
        JSON.stringify({
          ...job,
          body: { requirements: '熟悉 Go', responsibilities: '负责服务端开发' },
        }),
      ),
    )
    await flushPromises()

    expect(wrapper.find('[role="status"]').exists()).toBe(false)
    expect(wrapper.text()).toContain(job.title)

    wrapper.unmount()
  })

  it('keeps main and other job families distinct and renders formal bounds readably', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === '/api/v1/jobs/filter-options') {
          return new Response(JSON.stringify(options))
        }
        return new Response(
          JSON.stringify({ items: [job], page: { number: 1, page_size: 20, total: 1 } }),
        )
      }),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(JobListView, { global: { plugins: [router] } })
    await flushPromises()

    const card = wrapper.find('.job-card')
    expect(card.text()).toContain('主岗位族')
    expect(card.text()).toContain('后端')
    expect(card.text()).toContain('其他命中岗位族')
    expect(card.text()).toContain('AI 应用')
    expect(card.text()).toContain('本科及以下')
    expect(card.text()).toContain('未明确工作年限')
    expect(card.text()).not.toContain('0 年')

    wrapper.unmount()
  })

  it('shows the complete JD sections without exposing processing metadata', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              ...job,
              body: {
                requirements: '熟悉 Go 与 PostgreSQL',
                responsibilities: '1. 负责服务端开发。 2. 维护服务稳定性。',
                supplemental: '具备协作意识',
                technology_highlights: [
                  { end: 5, section: 'requirements', start: 3 },
                  { end: 18, section: 'requirements', start: 8 },
                ],
              },
            }),
          ),
      ),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'job-detail', params: { id: job.id } })
    await router.isReady()
    const wrapper = mount(JobDetailView, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.text()).toContain('岗位职责')
    expect(wrapper.text()).toContain('负责服务端开发')
    expect(wrapper.findAll('.body-section ol > li')).toHaveLength(2)
    expect(wrapper.text()).toContain('任职要求')
    expect(wrapper.text()).toContain('补充说明')
    expect(wrapper.findAll('mark.technology-highlight').map((mark) => mark.text())).toEqual([
      'Go',
      'PostgreSQL',
    ])
    expect(wrapper.text()).not.toContain('parser')
    expect(wrapper.text()).not.toContain('audit')
    expect(wrapper.text()).not.toContain('fallback')

    wrapper.unmount()
  })

  it('keeps a long technology list compact on narrow displays until the reader expands it', async () => {
    const technologies = Array.from({ length: 9 }, (_, index) => ({
      value: `technology_${index + 1}`,
      label: `技术词 ${index + 1}`,
    }))
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              ...job,
              technology_keywords: technologies,
              body: { requirements: '熟悉 Go', responsibilities: '负责服务端开发' },
            }),
          ),
      ),
    )
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'job-detail', params: { id: job.id } })
    await router.isReady()
    const wrapper = mount(JobDetailView, { global: { plugins: [router] } })
    await flushPromises()

    const keywordList = wrapper.get(`#job-technology-keywords-${job.id}`)
    const toggle = wrapper.get('button[aria-controls^="job-technology-keywords-"]')
    expect(keywordList.classes()).toContain('tag-list--collapsed')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(toggle.text()).toContain('展开全部技术词（9）')

    await toggle.trigger('click')

    expect(keywordList.classes()).not.toContain('tag-list--collapsed')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(toggle.text()).toContain('收起技术词')

    wrapper.unmount()
  })
})
