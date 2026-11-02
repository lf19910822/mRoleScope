import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppRequestLoader from '@/components/AppRequestLoader.vue'

describe('AppRequestLoader', () => {
  it('is a fixed, accessible status layer only while requests are visibly pending', () => {
    const wrapper = mount(AppRequestLoader, { props: { visible: true } })

    const layer = wrapper.get('[role="status"]')
    expect(layer.attributes('aria-busy')).toBe('true')
    expect(layer.text()).toContain('正在更新岗位数据')
    expect(layer.classes()).toContain('app-request-loader--visible')

    wrapper.unmount()
  })
})
