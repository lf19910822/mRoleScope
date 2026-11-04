import { describe, expect, it } from 'vitest'

import { presentJDSection } from '@/lib/jd-body-presentation'

describe('presentJDSection', () => {
  it('keeps an inline consecutive Arabic list and its lead-in text', () => {
    expect(presentJDSection('岗位职责包括： 1. 负责接口设计。 2. 维护服务稳定性。')).toEqual({
      items: [
        { end: 7, kind: 'lead', start: 0, text: '岗位职责包括：' },
        { end: 18, kind: 'ordered-item', marker: '1.', start: 11, text: '负责接口设计。' },
        { end: 30, kind: 'ordered-item', marker: '2.', start: 22, text: '维护服务稳定性。' },
      ],
      kind: 'list',
    })
  })

  it('recognizes consecutive Chinese ordinals without exposing their punctuation as content', () => {
    expect(presentJDSection('一、负责服务设计；二、维护线上稳定性。')).toEqual({
      items: [
        { end: 9, kind: 'ordered-item', marker: '一、', start: 2, text: '负责服务设计；' },
        { end: 19, kind: 'ordered-item', marker: '二、', start: 11, text: '维护线上稳定性。' },
      ],
      kind: 'list',
    })
  })

  it('recognizes parenthesized ordinals whose content starts with an experience year', () => {
    expect(presentJDSection('1）负责算法研究；2）2年以上相关经验；3）熟悉工程实践。')).toEqual({
      items: [
        { end: 9, kind: 'ordered-item', marker: '1）', start: 2, text: '负责算法研究；' },
        { end: 20, kind: 'ordered-item', marker: '2）', start: 11, text: '2年以上相关经验；' },
        { end: 29, kind: 'ordered-item', marker: '3）', start: 22, text: '熟悉工程实践。' },
      ],
      kind: 'list',
    })
  })

  it('recognizes enumeration-comma ordinals whose content starts with an experience year', () => {
    expect(
      presentJDSection('1、3年以上游戏客户端开发经验；2、熟悉UI架构；3、具备软件架构能力。'),
    ).toMatchObject({
      items: [
        { kind: 'ordered-item', marker: '1、', text: '3年以上游戏客户端开发经验；' },
        { kind: 'ordered-item', marker: '2、', text: '熟悉UI架构；' },
        { kind: 'ordered-item', marker: '3、', text: '具备软件架构能力。' },
      ],
      kind: 'list',
    })
  })

  it('recognizes a numeric-dot ordinal with one source whitespace before its dot', () => {
    for (const marker of ['2 .', '2 ．']) {
      expect(
        presentJDSection(`1.负责工具开发；${marker}有分布式管线开发经验；3.熟悉性能优化。`),
      ).toMatchObject({
        items: [
          { kind: 'ordered-item', marker: '1.', text: '负责工具开发；' },
          { kind: 'ordered-item', marker, text: '有分布式管线开发经验；' },
          { kind: 'ordered-item', marker: '3.', text: '熟悉性能优化。' },
        ],
        kind: 'list',
      })
    }
  })

  it('recognizes numeric-leading interior items in a source-proven ordinal sequence', () => {
    expect(
      presentJDSection(
        '1.计算机科学或相关学科的本科及以上学历；2.1-3年游戏从业经历，或者同等时间的业余或独立游戏开发经历；3.1年以上Unity的开发经验、C#使用经验和系统开发经验；4.至少熟悉一款3D游戏引擎；5.良好的工程设计能力与规范的编程风格，能熟练使用各种设计模式；6.良好的沟通协作能力、自驱动的学习能力和分析解决问题能力，责任心强；',
      ),
    ).toMatchObject({
      items: [
        { kind: 'ordered-item', marker: '1.', text: '计算机科学或相关学科的本科及以上学历；' },
        {
          kind: 'ordered-item',
          marker: '2.',
          text: '1-3年游戏从业经历，或者同等时间的业余或独立游戏开发经历；',
        },
        {
          kind: 'ordered-item',
          marker: '3.',
          text: '1年以上Unity的开发经验、C#使用经验和系统开发经验；',
        },
        { kind: 'ordered-item', marker: '4.', text: '至少熟悉一款3D游戏引擎；' },
        {
          kind: 'ordered-item',
          marker: '5.',
          text: '良好的工程设计能力与规范的编程风格，能熟练使用各种设计模式；',
        },
        {
          kind: 'ordered-item',
          marker: '6.',
          text: '良好的沟通协作能力、自驱动的学习能力和分析解决问题能力，责任心强；',
        },
      ],
      kind: 'list',
    })
  })

  it('recognizes separately bounded numeric-leading items from the same JD supplemental text', () => {
    const presentation = presentJDSection(
      '1.有跨平台开发经验；2.1年以上商业或自研引擎源码开发经验；3.设计并开发过多线程框架，ECS等架构；4.1年以上服务器开发经验；5.良好的英语阅读能力；6.热爱游戏，多年游戏经验（主机，PC，手机）；',
    )

    expect(presentation).toMatchObject({ kind: 'list' })
    expect(
      presentation.kind === 'list'
        ? presentation.items
            .filter((item) => item.kind === 'ordered-item')
            .map((item) => item.marker)
        : [],
    ).toEqual(['1.', '2.', '3.', '4.', '5.', '6.'])
  })

  it('uses ordinal context rather than a requirement-content whitelist', () => {
    expect(presentJDSection('1.负责上线；2.3D图形优化与WebGL渲染；3.维护工具链；')).toMatchObject({
      items: [
        { kind: 'ordered-item', marker: '1.', text: '负责上线；' },
        { kind: 'ordered-item', marker: '2.', text: '3D图形优化与WebGL渲染；' },
        { kind: 'ordered-item', marker: '3.', text: '维护工具链；' },
      ],
      kind: 'list',
    })
  })

  it('keeps a numeric-dot candidate at the start of a sequence as fallback text', () => {
    const text = '1.28 年经验；2.负责项目；3.维护质量；'

    expect(presentJDSection(text)).toMatchObject({ kind: 'fallback', text })
  })

  it('preserves compact hyphen subentries within each ordinal source item', () => {
    expect(
      presentJDSection(
        '1. 打包工具开发 -根据项目需求设计方案； -维护构建稳定性。 2. 构建流程设计 -优化日常构建效率； -保障发布质量。',
      ),
    ).toMatchObject({
      items: [
        {
          childItems: [
            { kind: 'bullet-item', marker: '-', text: '根据项目需求设计方案；' },
            { kind: 'bullet-item', marker: '-', text: '维护构建稳定性。' },
          ],
          kind: 'ordered-item',
          marker: '1.',
        },
        {
          childItems: [
            { kind: 'bullet-item', marker: '-', text: '优化日常构建效率；' },
            { kind: 'bullet-item', marker: '-', text: '保障发布质量。' },
          ],
          kind: 'ordered-item',
          marker: '2.',
        },
      ],
      kind: 'list',
    })
  })

  it('keeps one compact hyphen inside an ordinal item as ordinary source text', () => {
    const text = '1. 使用 kube-proxy 与 -debug 参数。 2. 维护服务稳定性。'
    const presentation = presentJDSection(text)

    expect(presentation).toMatchObject({ kind: 'list' })
    expect(presentation.kind === 'list' && presentation.items[0]).not.toHaveProperty('childItems')
  })

  it('preserves explicit source group headings around a restarted ordinal sequence', () => {
    expect(
      presentJDSection(
        '岗位要求 基础 1.熟悉 C++；2.具备项目经验；加分项 1.熟悉工具链；2.具备优化经验。',
      ),
    ).toMatchObject({
      items: [
        { kind: 'lead', text: '岗位要求' },
        { kind: 'group-heading', text: '基础' },
        { kind: 'ordered-item', marker: '1.', text: '熟悉 C++；' },
        { kind: 'ordered-item', marker: '2.', text: '具备项目经验；' },
        { kind: 'group-heading', text: '加分项' },
        { kind: 'ordered-item', marker: '1.', text: '熟悉工具链；' },
        { kind: 'ordered-item', marker: '2.', text: '具备优化经验。' },
      ],
      kind: 'list',
    })
  })

  it('preserves a source-proven group whose later marker skips a number', () => {
    expect(
      presentJDSection(
        '岗位要求 基础 1.熟悉客户端工具链；2.具备构建经验；加分项 1.掌握发布流程；3.熟悉性能优化。',
      ),
    ).toMatchObject({
      items: [
        { kind: 'lead', text: '岗位要求' },
        { kind: 'group-heading', text: '基础' },
        { kind: 'ordered-item', marker: '1.', text: '熟悉客户端工具链；' },
        { kind: 'ordered-item', marker: '2.', text: '具备构建经验；' },
        { kind: 'group-heading', text: '加分项' },
        { kind: 'ordered-item', marker: '1.', text: '掌握发布流程；' },
        { kind: 'ordered-item', marker: '3.', text: '熟悉性能优化。' },
      ],
      kind: 'list',
    })
  })

  it('keeps an ordinal reset without an explicit group heading as fallback text', () => {
    const text = '1.负责工具开发；2.维护构建流程；1.熟悉跨平台发布。'

    expect(presentJDSection(text)).toEqual({
      end: Array.from(text).length,
      kind: 'fallback',
      start: 0,
      text,
    })
  })

  it('keeps an ungrouped skipped ordinal as fallback text', () => {
    const text = '1.负责工具开发；3.熟悉跨平台发布。'

    expect(presentJDSection(text)).toEqual({
      end: Array.from(text).length,
      kind: 'fallback',
      start: 0,
      text,
    })
  })

  it('recognizes repeated spaced bullet markers but preserves a hyphenated technology name', () => {
    expect(presentJDSection('说明： - 熟悉 kube-proxy。 - 维护发布流程。')).toEqual({
      items: [
        { end: 3, kind: 'lead', start: 0, text: '说明：' },
        { end: 20, kind: 'bullet-item', marker: '-', start: 6, text: '熟悉 kube-proxy。' },
        { end: 30, kind: 'bullet-item', marker: '-', start: 23, text: '维护发布流程。' },
      ],
      kind: 'list',
    })
  })

  it('renders one unambiguous top-level ordinal as a source-list item', () => {
    expect(presentJDSection('补充说明： 1、具有大规模集群运维经验。')).toMatchObject({
      items: [
        {
          kind: 'lead',
          text: '补充说明：',
        },
        {
          kind: 'ordered-item',
          marker: '1、',
          text: '具有大规模集群运维经验。',
        },
      ],
      kind: 'list',
    })
  })

  it('falls back without splitting decimals or versions', () => {
    for (const text of [
      '要求 1.28 年相关经验，熟悉 v3.5。',
      '要求 1 .28 年相关经验，熟悉 v3 .5。',
      '使用 kube-proxy 与 service-mesh。',
    ]) {
      expect(presentJDSection(text)).toEqual({
        end: Array.from(text).length,
        kind: 'fallback',
        start: 0,
        text,
      })
    }
  })
})
