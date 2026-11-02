import { describe, expect, it } from 'vitest'

import { fragmentHighlightedText } from '@/lib/highlighted-text'

describe('fragmentHighlightedText', () => {
  it('uses Unicode code-point offsets after astral characters', () => {
    expect(
      fragmentHighlightedText('🚀熟悉 Go', 0, 6, 'requirements', [
        { end: 6, section: 'requirements', start: 4 },
      ]),
    ).toEqual([
      { highlighted: false, text: '🚀熟悉 ' },
      { highlighted: true, text: 'Go' },
    ])
  })

  it('clips valid section ranges to a presented source block', () => {
    expect(
      fragmentHighlightedText('熟悉 Go 与 PostgreSQL', 3, 13, 'requirements', [
        { end: 5, section: 'requirements', start: 3 },
        { end: 17, section: 'requirements', start: 8 },
      ]),
    ).toEqual([
      { highlighted: true, text: 'Go' },
      { highlighted: false, text: ' 与 ' },
      { highlighted: true, text: 'Postg' },
    ])
  })

  it('drops malformed and wrong-section ranges and merges valid overlaps', () => {
    expect(
      fragmentHighlightedText('Go PostgreSQL', 0, 13, 'requirements', [
        { end: 2, section: 'responsibilities', start: 0 },
        { end: 1, section: 'requirements', start: 2 },
        { end: 99, section: 'requirements', start: 3 },
        { end: 10, section: 'requirements', start: 3 },
        { end: 13, section: 'requirements', start: 8 },
      ]),
    ).toEqual([
      { highlighted: false, text: 'Go ' },
      { highlighted: true, text: 'PostgreSQL' },
    ])
  })

  it('retains literal angle brackets as text fragments', () => {
    expect(
      fragmentHighlightedText('使用 <Go> 开发', 0, 10, 'requirements', [
        { end: 7, section: 'requirements', start: 3 },
      ]),
    ).toEqual([
      { highlighted: false, text: '使用 ' },
      { highlighted: true, text: '<Go>' },
      { highlighted: false, text: ' 开发' },
    ])
  })
})
