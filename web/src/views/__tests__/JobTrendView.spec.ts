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
    expect(source).toContain('开始配置首条曲线')
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

  it('provides an all-history reset control that restores only the chart viewport', () => {
    expect(source).toContain('data-testid="reset-trend-view"')
    expect(source).toContain("type: 'dataZoom'")
    expect(source).toContain('start: 0')
    expect(source).toContain('end: 100')
  })

  it('places an inclusive custom date control beside all history and reuses the shared date picker', () => {
    expect(source).toContain('data-testid="open-custom-trend-range"')
    expect(source).toContain('data-testid="apply-custom-trend-range"')
    expect(source).toContain('FilterDatePicker')
    expect(source).toContain('function applyCustomRange')
  })

  it('keeps range help and the custom calendar out of the chart layout flow', () => {
    expect(source).toContain('job-trend-view__range-stage')
    expect(source).toContain("dashboard.range === 'custom' || customRangeOpen")
    expect(source).toMatch(
      /\.job-trend-view__range-stage \{[\s\S]*?position:\s*relative;/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__custom-range \{[\s\S]*?position:\s*absolute;/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__all-history-gestures \{[\s\S]*?position:\s*absolute;/u,
    )
  })

  it('keeps the fixed narrow-screen configurator clear of the chart and gives the dark rail local tokens', () => {
    const narrowStyles = source.slice(source.indexOf('@media (max-width: 760px)'))

    expect(narrowStyles).toMatch(
      /\.job-trend-view \{[\s\S]*?padding-bottom:\s*calc\(88px \+ env\(safe-area-inset-bottom\)\);/u,
    )
    expect(narrowStyles).toMatch(
      /\.job-trend-view__mobile-panel \{[\s\S]*?bottom:\s*calc\(18px \+ env\(safe-area-inset-bottom\)\);/u,
    )
    expect(source).toContain('--trend-muted: #a8c3ca')
    expect(source).toContain('function chartPointClicked')
    expect(source).toContain('job-trend-view__drilldown')
    expect(source).toContain('该日没有匹配岗位')
    expect(source).not.toContain('pointFeedback')
  })

  it('uses the shared technology browser and constrains it to the configuration field width', () => {
    expect(source).toContain('TechnologyKeywordSelect')
    expect(source).toMatch(
      /\.job-trend-view__technology-select \{[\s\S]*?min-width:\s*0;[\s\S]*?width:\s*100%;/u,
    )
  })

  it('keeps curve settings compact and card-like while preserving readable grouping', () => {
    expect(source).toMatch(
      /\.job-trend-view__curve-row \{[\s\S]*?border-radius:\s*12px;[\s\S]*?gap:\s*7px;[\s\S]*?padding:\s*11px;/u,
    )
    expect(source).toContain('job-trend-view__curve-identity')
    expect(source).toContain('job-trend-view__curve-actions')
    expect(source).toMatch(/\.job-trend-view__rail \{[\s\S]*?padding:\s*20px 16px 14px;/u)
  })

  it('makes the curve dimension the visual headline and uses the line color on the card edge', () => {
    expect(source).toContain("{{ trendCurveLabel(row) || '未设置曲线' }}")
    expect(source).toMatch(
      /\.job-trend-view__curve-identity strong \{[\s\S]*?font-size:\s*0\.84rem;[\s\S]*?font-weight:\s*800;/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__curve-row \{[\s\S]*?border-top:\s*4px solid var\(--curve-color\);/u,
    )
    expect(source).not.toContain('curveObservationText')
  })

  it('exposes a batch apply action and incomplete-card focus state', () => {
    expect(source).toContain('data-testid="apply-all-curves"')
    expect(source).toContain('data-testid="clear-all-curves"')
    expect(source).toContain('function clearAllCurves')
    expect(source).toContain('应用全部')
    expect(source).toContain('job-trend-view__curve-row--incomplete')
    expect(source).toContain('job-trend-view__curve-row--focused')
    expect(source).toContain('scrollIntoView')
    expect(source).toContain('job-trend-view__curve-pending')
    expect(source).toContain(
      'background: color-mix(in srgb, var(--curve-color), var(--trend-curve-surface) 90%)',
    )
  })

  it('keeps the rail footer outside the scrollable curve list', () => {
    expect(source).toContain('data-testid="curve-rail-footer"')
    expect(source).toMatch(
      /\.job-trend-view__curve-list \{[\s\S]*?flex:\s*1 1 auto;[\s\S]*?min-height:\s*0;[\s\S]*?scrollbar-gutter:\s*stable;/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__rail-footer \{[\s\S]*?flex:\s*0 0 auto;/u,
    )
  })

  it('anchors the drilldown arrow to the source point and gives its compact list an explicit scroll region', () => {
    expect(source).toContain("'--drilldown-arrow-offset'")
    expect(source).toContain('job-trend-view__drilldown--above')
    expect(source).toMatch(
      /\.job-trend-view__drilldown \{[\s\S]*?display:\s*flex;[\s\S]*?overflow:\s*hidden;/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__drilldown::before \{[\s\S]*?left:\s*var\(--drilldown-arrow-offset\);/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__drilldown-list \{[\s\S]*?flex:\s*1 1 auto;[\s\S]*?overflow-y:\s*auto;/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__drilldown-list li \{[\s\S]*?grid-template-areas:/u,
    )
  })

  it('opens detail as an upward evidence sheet within the chart workspace and leaves the configuration rail uncovered', () => {
    expect(source).toContain('<Transition name="job-trend-detail">')
    expect(source).toMatch(
      /\.job-trend-view__detail-drawer \{[\s\S]*?inset:\s*auto var\(--trend-workspace-right\) 32px var\(--trend-workspace-left\);/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__drilldown-backdrop \{[\s\S]*?inset:\s*0 var\(--trend-workspace-right\) 0 0;/u,
    )
    expect(source).toMatch(
      /\.job-trend-detail-enter-from,[\s\S]*?transform:\s*translateY\(calc\(100% \+ 28px\)\);/u,
    )
  })

  it('keeps the detail sheet rounded while scrolling its content inside the frame', () => {
    expect(source).toContain('job-trend-view__detail-content')
    expect(source).toMatch(
      /\.job-trend-view__detail-drawer \{[\s\S]*?display:\s*flex;[\s\S]*?overflow:\s*hidden;/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__detail-content \{[\s\S]*?min-height:\s*0;[\s\S]*?overflow-y:\s*auto;/u,
    )
  })
})
