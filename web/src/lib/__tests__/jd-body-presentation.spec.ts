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

  it('falls back without splitting decimals, versions, or an incomplete ordinal', () => {
    for (const text of [
      '要求 1.28 年相关经验，熟悉 v3.5。',
      '使用 kube-proxy 与 service-mesh。',
      '岗位职责： 1. 负责服务设计。',
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
