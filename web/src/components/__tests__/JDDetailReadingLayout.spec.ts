import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import JDDetailReadingLayout from '../JDDetailReadingLayout.vue'

const job = {
  activity_state: 'effective' as const,
  body: {
    requirements: '1. 熟悉 Go。 2. 熟悉 PostgreSQL。',
    responsibilities: '1. 负责服务端开发。 2. 维护线上稳定性。',
  },
  education: { explicit_requirement: true, lower_bound: 'bachelor' as const },
  experience: { explicit_requirement: true, minimum_years: 3 },
  first_discovered_date: '2026-10-02',
  id: 'jd-margin-index',
  is_urgent: false,
  locations: ['上海'],
  main_job_family: { label: '后端', value: 'backend' },
  other_job_families: [],
  projects: [],
  technology_keywords: [],
  title: '服务端工程师',
}

const originalScrollIntoView = HTMLElement.prototype.scrollIntoView

afterEach(() => {
  if (originalScrollIntoView) {
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: originalScrollIntoView,
    })
  } else {
    Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView')
  }
})

describe('JDDetailReadingLayout', () => {
  it('shows independent collapsed comparison tags and formatted dimension values', async () => {
    const wrapper = mount(JDDetailReadingLayout, {
      props: {
        job: {
          ...job,
          comparison: {
            status: 'available',
            dimension_changes: [{ field: '技术关键词', historical: 'Go', latest: 'Go、Rust' }],
            body_sections: [
              {
                label: '岗位职责',
                section: 'responsibilities',
                fragments: [
                  { kind: 'unchanged', text: '负责' },
                  { kind: 'deleted', text: '旧系统' },
                  { kind: 'inserted', text: '新系统' },
                ],
              },
            ],
          },
        },
      },
    })

    expect(wrapper.get('.detail-comparison__tag--dimensions').text()).toContain('维度变动')
    expect(wrapper.get('.detail-comparison__tag--dimensions').text()).toContain('1')
    expect(wrapper.get('.detail-comparison__tag--body').text()).toContain('正文变动')
    expect(wrapper.find('.detail-comparison__dimension-list').exists()).toBe(false)

    await wrapper.get('.detail-comparison__tag--dimensions').trigger('click')
    expect(wrapper.get('.detail-comparison__dimension-list').text()).toContain('Go → Go、Rust')
    await wrapper.get('.detail-comparison__tag--body').trigger('click')
    expect(wrapper.get('[aria-label="已删除"]').text()).toBe('旧系统')
    expect(wrapper.get('[aria-label="最新新增"]').text()).toBe('新系统')
    wrapper.unmount()
  })

  it('restores numbered list structure inside deleted and inserted body fragments', async () => {
    const wrapper = mount(JDDetailReadingLayout, {
      props: {
        job: {
          ...job,
          body: {
            ...job.body,
            responsibilities: '1. 维护稳定\n2. 负责平台',
          },
          comparison: {
            status: 'available',
            body_sections: [
              {
                label: '岗位职责',
                section: 'responsibilities',
                fragments: [
                  { kind: 'deleted', text: '1. 负责平台\n2. 维护稳定' },
                  { kind: 'inserted', text: '1. 维护稳定\n2. 负责平台' },
                ],
              },
            ],
          },
        },
      },
    })

    await wrapper.get('.detail-comparison__tag--body').trigger('click')

    expect(wrapper.findAll('.detail-comparison__fragment-line--item')).toHaveLength(4)
    expect(
      wrapper.findAll('.detail-comparison__fragment-marker').map((node) => node.text()),
    ).toEqual(['1', '2', '1', '2'])
    expect(wrapper.findAll('.detail-comparison__fragment--deleted')).toHaveLength(2)
    expect(wrapper.findAll('.detail-comparison__fragment--inserted')).toHaveLength(2)
    wrapper.unmount()
  })

  it('keeps historical content readable and emits a local comparison retry', async () => {
    const wrapper = mount(JDDetailReadingLayout, {
      props: {
        job: {
          ...job,
          comparison: {
            status: 'unavailable',
            message: '变动对比暂不可用',
            retryable: true,
          },
        },
      },
    })

    expect(wrapper.text()).toContain('变动对比暂不可用')
    expect(wrapper.text()).toContain('负责服务端开发')
    await wrapper.get('.detail-comparison__retry').trigger('click')
    expect(wrapper.emitted('retry-comparison')).toHaveLength(1)
    wrapper.unmount()
  })

  it('labels a historical JD status with its node-day context and keeps the current comparison visible', () => {
    const wrapper = mount(JDDetailReadingLayout, {
      props: {
        historicalContext: { currentStatus: 'confirmed_closed' },
        job,
      },
    })

    expect(wrapper.get('.detail-reading-layout__state').text()).toBe('节点当日：仍有效')
    expect(wrapper.get('.detail-reading-layout__current-status').text()).toBe('当前：确认下线')
    wrapper.unmount()
  })

  it('scrolls through the margin index without adding a browser-history entry', () => {
    const scrollIntoView = vi.fn()
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    })
    const wrapper = mount(JDDetailReadingLayout, {
      attachTo: document.body,
      props: { job },
    })
    const indexLink = wrapper.get('.detail-reading-layout__margin-index a')
    const click = new MouseEvent('click', { bubbles: true, cancelable: true })

    const nativeNavigationWasAllowed = indexLink.element.dispatchEvent(click)

    expect(nativeNavigationWasAllowed).toBe(false)
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' })
    wrapper.unmount()
  })
})
