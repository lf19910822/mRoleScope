import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(process.cwd(), 'src/views/JobTrendView.vue'), 'utf8')

describe('JobTrendView', () => {
  it('uses a compact dashboard title that remains readable in a 390px viewport', () => {
    expect(source).toContain('岗位需求变化趋势')
    expect(source).not.toContain('需求变化，看见时间的方向')
  })

  it('keeps a stable chart region with a concise no-curve prompt instead of a mock line', () => {
    expect(source).toContain('data-testid="trend-chart-region"')
    expect(source).toContain('data-testid="trend-empty-state"')
    expect(source).toContain('添加一条曲线，开始查看岗位趋势。')
    expect(source).not.toMatch(/mock|fixture|示例曲线/i)
  })

  it('keeps the configuration rail visible on desktop and reachable from a narrow viewport panel', () => {
    expect(source).toContain('data-testid="trend-configuration-rail"')
    expect(source).toContain('data-testid="open-trend-configuration"')
    const narrowStyles = source.slice(source.indexOf('@media (max-width: 760px)'))

    expect(source).toMatch(/\.job-trend-view__rail \{[\s\S]*?position:\s*fixed;/u)
    expect(narrowStyles).toMatch(/\.job-trend-view__rail \{[\s\S]*?display:\s*none;/u)
    expect(narrowStyles).toMatch(/\.job-trend-view__mobile-panel \{[\s\S]*?display:\s*block;/u)
  })
})
