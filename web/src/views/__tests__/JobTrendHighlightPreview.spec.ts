import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import JobTrendHighlightPreview from '../JobTrendHighlightPreview.vue'

describe('JobTrendHighlightPreview', () => {
  it('shows all three incomplete-card treatments and lets the reader focus one', async () => {
    const wrapper = mount(JobTrendHighlightPreview)

    expect(wrapper.findAll('.highlight-preview__option')).toHaveLength(3)
    expect(wrapper.findAll('.highlight-preview__curve-card')).toHaveLength(3)
    expect(wrapper.text()).toContain('高对比边缘')
    expect(wrapper.text()).toContain('浅色状态底板')
    expect(wrapper.text()).toContain('左侧聚焦轨道')

    const chooseButtons = wrapper.findAll('.highlight-preview__choose')
    await chooseButtons[2]!.trigger('click')

    expect(wrapper.get('.highlight-preview__selection').text()).toContain('左侧聚焦轨道')
    expect(chooseButtons[2]!.attributes('aria-pressed')).toBe('true')
  })
})
