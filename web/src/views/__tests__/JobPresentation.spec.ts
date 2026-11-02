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
                requirements: '熟悉 Go',
                responsibilities: '负责服务端开发',
                supplemental: '具备协作意识',
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
    expect(wrapper.text()).toContain('任职要求')
    expect(wrapper.text()).toContain('补充说明')
    expect(wrapper.text()).not.toContain('parser')
    expect(wrapper.text()).not.toContain('audit')
    expect(wrapper.text()).not.toContain('fallback')

    wrapper.unmount()
  })
})
