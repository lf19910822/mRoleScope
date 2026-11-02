import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import JDSectionList from '@/components/JDSectionList.vue'

describe('JDSectionList', () => {
  it('renders a proven ordered source list as individually readable items', () => {
    const wrapper = mount(JDSectionList, {
      props: { text: '岗位职责包括： 1. 负责接口设计。 2. 维护服务稳定性。' },
    })

    expect(wrapper.get('ol').text()).toContain('负责接口设计。')
    expect(wrapper.findAll('ol > li')).toHaveLength(2)
    expect(wrapper.findAll('[aria-label^="第 "]')).toHaveLength(2)
    expect(wrapper.text()).toContain('岗位职责包括：')

    wrapper.unmount()
  })

  it('keeps ambiguous prose as one selectable fallback block', () => {
    const text = '要求 1.28 年相关经验，熟悉 kube-proxy 与 service-mesh。'
    const wrapper = mount(JDSectionList, { props: { text } })

    expect(wrapper.find('ol, ul').exists()).toBe(false)
    expect(wrapper.get('.jd-section-list__fallback').text()).toBe(text)

    wrapper.unmount()
  })
})
