import { describe, expect, it } from 'vitest'

import { presentJobTitle } from '../job-title-presentation'

describe('job title presentation', () => {
  it('emphasizes only the deterministic project label that appears in the title', () => {
    expect(
      presentJobTitle('后端开发工程师（AI 游戏方向）- 星布谷地', [
        { label: '星布谷地', value: 'miliastra_wonderland' },
      ]),
    ).toEqual([
      { kind: 'text', text: '后端开发工程师（AI 游戏方向）- ' },
      { kind: 'project', text: '星布谷地' },
    ])
  })

  it('prefers the longest project label at an overlapping title position', () => {
    expect(
      presentJobTitle('原神启动客户端工程师', [
        { label: '原神', value: 'genshin_impact' },
        { label: '原神启动', value: 'genshin_launch' },
      ]),
    ).toEqual([
      { kind: 'project', text: '原神启动' },
      { kind: 'text', text: '客户端工程师' },
    ])
  })

  it('keeps a title intact when no supplied project label has an exact title match', () => {
    expect(
      presentJobTitle('平台工程师', [{ label: '星布谷地', value: 'miliastra_wonderland' }]),
    ).toEqual([{ kind: 'text', text: '平台工程师' }])
  })
})
