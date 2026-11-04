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
