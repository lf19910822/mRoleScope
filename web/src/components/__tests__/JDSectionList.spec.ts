import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import JDSectionList from '@/components/JDSectionList.vue'

function dispatchPointer(
  target: Element,
  type: string,
  { clientX, clientY, pointerType }: { clientX: number; clientY: number; pointerType: string },
): void {
  const event = new MouseEvent(type, { bubbles: true, clientX, clientY })
  Object.defineProperty(event, 'pointerType', { value: pointerType })
  target.dispatchEvent(event)
}

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

  it('renders numeric-leading source requirements as separate cards', () => {
    const wrapper = mount(JDSectionList, {
      props: {
        text: '1.计算机科学或相关学科的本科及以上学历；2.1-3年游戏从业经历，或者同等时间的业余或独立游戏开发经历；3.1年以上Unity的开发经验、C#使用经验和系统开发经验；4.至少熟悉一款3D游戏引擎；5.良好的工程设计能力与规范的编程风格，能熟练使用各种设计模式；6.良好的沟通协作能力、自驱动的学习能力和分析解决问题能力，责任心强；',
      },
    })

    expect(wrapper.findAll('ol > li')).toHaveLength(6)
    expect(wrapper.findAll('[aria-label^="第 "]')).toHaveLength(6)
    expect(wrapper.get('ol > li:nth-child(2)').text()).toContain('1-3年游戏从业经历')
    expect(wrapper.get('ol > li:nth-child(3)').text()).toContain('1年以上Unity的开发经验')

    wrapper.unmount()
  })

  it('renders explicitly labeled ordinal groups as separate source lists', () => {
    const wrapper = mount(JDSectionList, {
      props: {
        text: '岗位要求 基础 1.熟悉 C++；2.具备项目经验；加分项 1.熟悉工具链；2.具备优化经验。',
      },
    })

    expect(wrapper.get('.jd-section-list__lead').text()).toBe('岗位要求')
    expect(
      wrapper.findAll('.jd-section-list__group-heading').map((heading) => heading.text()),
    ).toEqual(['基础', '加分项'])
    expect(wrapper.findAll('ol')).toHaveLength(2)
    expect(wrapper.findAll('ol > li')).toHaveLength(4)
    expect(wrapper.text()).toContain('具备项目经验；')
    expect(wrapper.text()).toContain('熟悉工具链；')

    wrapper.unmount()
  })

  it('renders compact hyphen source entries as nested bullets beneath their ordinal parent', () => {
    const wrapper = mount(JDSectionList, {
      props: {
        text: '1. 打包工具开发 -根据项目需求设计方案； -维护构建稳定性。 2. 构建流程设计 -优化日常构建效率； -保障发布质量。',
      },
    })

    expect(wrapper.findAll('.jd-section-list__items > li')).toHaveLength(2)
    expect(wrapper.findAll('.jd-section-list__child-items')).toHaveLength(2)
    expect(wrapper.findAll('.jd-section-list__child-items > li')).toHaveLength(4)
    expect(wrapper.get('.jd-section-list__items > li').text()).toContain('打包工具开发')
    expect(wrapper.get('.jd-section-list__child-items').text()).toContain('维护构建稳定性。')

    wrapper.unmount()
  })

  it('keeps ambiguous prose as one selectable fallback block', () => {
    const text = '要求 1.28 年相关经验，熟悉 kube-proxy 与 service-mesh。'
    const wrapper = mount(JDSectionList, { props: { text } })

    expect(wrapper.find('ol, ul').exists()).toBe(false)
    expect(wrapper.get('.jd-section-list__fallback').text()).toBe(text)

    wrapper.unmount()
  })

  it('keeps source-item rows static while a pointer moves across them', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/components/JDSectionList.vue'), 'utf8')
    const wrapper = mount(JDSectionList, {
      props: { text: '1. 负责接口设计。 2. 维护服务稳定性。' },
    })
    const card = wrapper.get('ol > li')
    dispatchPointer(card.element, 'pointermove', { clientX: 200, clientY: 0, pointerType: 'mouse' })

    expect(card.classes()).not.toContain('jd-section-list__card--active')
    expect(card.attributes('style')).toBeUndefined()
    expect(source).not.toMatch(/@pointer(?:enter|leave|move)=/u)
    expect(source).not.toMatch(/(?:activate|update|reset)FloatingCard/u)
    expect(source).not.toMatch(/--jd-card-/u)
    expect(source).not.toMatch(/\b(?:perspective|transform-style|will-change)\s*:/u)

    wrapper.unmount()
  })
})
