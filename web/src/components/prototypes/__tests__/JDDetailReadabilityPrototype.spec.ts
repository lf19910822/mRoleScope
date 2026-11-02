import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createMemoryHistory } from 'vue-router'

import JDDetailReadabilityPrototype from '../JDDetailReadabilityPrototype.vue'
import { createJDRouter } from '@/router'

const job = {
  activity_state: 'effective' as const,
  body: {
    requirements: '熟悉 Go 与 PostgreSQL。',
    responsibilities: '1. 负责服务端接口设计与开发。 2. 维护线上服务的稳定性。',
  },
  education: { explicit_requirement: true, lower_bound: 'bachelor' as const },
  experience: { explicit_requirement: true, minimum_years: 3 },
  first_discovered_date: '2026-10-02',
  id: 'jd-stepper-example',
  is_urgent: false,
  locations: ['上海'],
  main_job_family: { label: '后端', value: 'backend' },
  other_job_families: [],
  projects: [],
  technology_keywords: [
    { label: 'Go', value: 'go' },
    { label: 'PostgreSQL', value: 'postgresql' },
  ],
  title: '服务端工程师',
}

describe('JD detail readability prototype', () => {
  it('keeps B-variant source items readable and exposes their step order as read-only context', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({
      name: 'job-detail',
      params: { id: job.id },
      query: { prototype: 'jd-detail', variant: 'B' },
    })
    await router.isReady()

    const wrapper = mount(JDDetailReadabilityPrototype, {
      props: { job },
      global: { plugins: [router] },
    })

    const steps = wrapper.findAll('[aria-label^="第 "]')
    expect(steps.map((step) => step.attributes('aria-label'))).toEqual(['第 1 条', '第 2 条'])
    expect(steps.map((step) => step.classes())).toEqual([
      expect.arrayContaining(['prototype-marker--single-digit']),
      expect.arrayContaining(['prototype-marker--single-digit']),
    ])
    expect(steps.every((step) => step.element.closest('button') === null)).toBe(true)
    expect(wrapper.findAll('.prototype-step-connector')).toHaveLength(0)
    expect(wrapper.text()).toContain('负责服务端接口设计与开发。')
    expect(wrapper.text()).toContain('维护线上服务的稳定性。')

    wrapper.unmount()
  })

  it('presents the selected B direction as JD content rather than a prototype control panel', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({
      name: 'job-detail',
      params: { id: job.id },
      query: { prototype: 'jd-detail', variant: 'B' },
    })
    await router.isReady()

    const wrapper = mount(JDDetailReadabilityPrototype, {
      props: { job },
      global: { plugins: [router] },
    })

    expect(wrapper.text()).not.toContain('视觉原型')
    expect(wrapper.text()).not.toContain('浮岛气泡')
    expect(wrapper.text()).not.toContain('01岗位职责')
    expect(wrapper.text()).not.toContain('02任职要求')
    expect(wrapper.find('[aria-label="视觉方案选择器"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('lets readers expand a dense technology keyword preview without losing any terms', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({
      name: 'job-detail',
      params: { id: job.id },
      query: { prototype: 'jd-detail', variant: 'B' },
    })
    await router.isReady()

    const technologies = Array.from({ length: 9 }, (_, index) => ({
      label: `技术词 ${index + 1}`,
      value: `technology-${index + 1}`,
    }))
    const wrapper = mount(JDDetailReadabilityPrototype, {
      props: { job: { ...job, technology_keywords: technologies } },
      global: { plugins: [router] },
    })

    const toggle = wrapper.get('button[aria-controls^="prototype-technology-keywords-"]')
    expect(toggle.text()).toContain('展开全部技术词')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(toggle.text()).toContain('收起技术词')
    expect(wrapper.text()).toContain('技术词 9')

    wrapper.unmount()
  })

  it('does not turn a lead-in sentence into an invented step', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({
      name: 'job-detail',
      params: { id: job.id },
      query: { prototype: 'jd-detail', variant: 'B' },
    })
    await router.isReady()

    const wrapper = mount(JDDetailReadabilityPrototype, {
      props: {
        job: {
          ...job,
          body: {
            ...job.body,
            responsibilities:
              '岗位职责包括： 1. 负责服务端接口设计与开发。 2. 维护线上服务的稳定性。',
          },
        },
      },
      global: { plugins: [router] },
    })

    expect(
      wrapper.findAll('[aria-label^="第 "]').map((step) => step.attributes('aria-label')),
    ).toEqual(['第 1 条', '第 2 条'])
    expect(wrapper.text()).toContain('岗位职责包括：')

    wrapper.unmount()
  })

  it('starts a bulleted source list at step one even when it follows a lead-in', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({
      name: 'job-detail',
      params: { id: job.id },
      query: { prototype: 'jd-detail', variant: 'B' },
    })
    await router.isReady()

    const wrapper = mount(JDDetailReadabilityPrototype, {
      props: {
        job: {
          ...job,
          body: {
            ...job.body,
            responsibilities:
              '岗位职责包括： - 负责服务端接口设计与开发。 - 维护线上服务的稳定性。',
          },
        },
      },
      global: { plugins: [router] },
    })

    expect(
      wrapper.findAll('[aria-label^="第 "]').map((step) => step.attributes('aria-label')),
    ).toEqual(['第 1 条', '第 2 条'])

    wrapper.unmount()
  })
})
