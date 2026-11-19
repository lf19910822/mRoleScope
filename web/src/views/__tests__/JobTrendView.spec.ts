import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(process.cwd(), 'src/views/JobTrendView.vue'), 'utf8')

describe('JobTrendView', () => {
  it('gives the AI workbench room for examples, operations, reasons, and editable curve rows', () => {
    expect(source).toContain('job-trend-view__ai-workbench')
    expect(source).toContain('job-trend-view__ai-example-grid')
    expect(source).toContain('submitAIExample')
    expect(source).toContain('job-trend-view__ai-activity')
    expect(source).toContain('job-trend-view__ai-chat')
    expect(source).toContain('job-trend-view__ai-analysis-grid')
    expect(source).toContain('用时 {{ aiTrend.durationLabel }}')
    expect(source).toContain('job-trend-view__ai-reason')
    expect(source).toContain('job-trend-view__ai-chat-reply')
    expect(source).toContain('job-trend-view__ai-curve-list')
    expect(source).toContain('job-trend-view__ai-curve-color')
    expect(source).toContain('job-trend-view__ai-curve-line')
    expect(source).toContain('aiMatchSymbol')
    expect(source).toContain("return '≈'")
    expect(source).toContain('updateAIDraftDimension')
    expect(source).toContain('替换现有曲线')
    expect(source).toContain('添加到当前图表')
    const exampleHandler = source.slice(
      source.indexOf('function submitAIExample'),
      source.indexOf('function preferredDrilldownHeight'),
    )
    expect(exampleHandler).not.toContain('aiTrend.submit')
  })

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

  it('keeps the configuration rail in the workspace with a compact marker state', () => {
    expect(source).toContain('data-testid="trend-configuration-rail"')
    expect(source).toMatch(/:data-testid="`curve-marker-\$\{row\.id\}`"/u)
    expect(source).toContain('data-testid="close-trend-configuration"')
    expect(source).toContain('aria-expanded')
    expect(source).toContain('job-trend-view--configuration-expanded')
    expect(source).toContain('job-trend-view__curve-marker-rail')
    expect(source).toContain('prefers-reduced-motion')
    expect(source).toContain('window.innerWidth >= 1440')
    const narrowStyles = source.slice(source.indexOf('@media (max-width: 760px)'))

    expect(source).toMatch(/\.job-trend-view \{[\s\S]*?display:\s*grid;/u)
    expect(source).toMatch(/\.job-trend-view__rail \{[\s\S]*?position:\s*sticky;/u)
    expect(source).toMatch(/\.job-trend-view__curve-marker-rail \{[\s\S]*?display:\s*grid;/u)
    expect(source).toMatch(/\.job-trend-view__configuration-trigger[\s\S]*?display:\s*none;/u)
    expect(narrowStyles).toMatch(/\.job-trend-view__mobile-panel \{[\s\S]*?display:\s*block;/u)
    expect(source).not.toMatch(/sessionStorage|localStorage/u)
  })

  it('keeps both desktop rail surfaces mounted while animating and uses one collapse control', () => {
    expect(source).toContain('job-trend-view__rail-content')
    expect(source).toContain('job-trend-view__rail-content--hidden')
    expect(source).not.toContain('v-if="!configurationExpanded && !mobileConfigurationOpen"')
    expect(source).not.toContain('v-if="configurationExpanded || mobileConfigurationOpen"')
    expect(source).not.toContain('data-testid="toggle-trend-configuration"')
    expect(source).toContain('data-testid="close-trend-configuration"')
    expect(source).toMatch(
      /\.job-trend-view__rail \{[\s\S]*?border-radius:\s*18px;/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__curve-marker-rail \{[\s\S]*?align-content:\s*start;[\s\S]*?grid-auto-rows:\s*32px;/u,
    )
  })

  it('uses delayed pull-out notes instead of native marker tooltips', () => {
    expect(source).toContain('job-trend-view__curve-marker-slot')
    expect(source).toContain('job-trend-view__curve-marker-note')
    expect(source).toContain('job-trend-view__curve-marker-divider')
    expect(source).not.toContain(':title="`展开曲线：${trendCurveLabel(row) || \'未设置曲线\'}`"')
    expect(source).toContain('transition-delay: 200ms, 200ms, 0s')
    expect(source).toContain('transition-delay: 0ms, 0ms, 160ms')
  })

  it('does not open a marker note when closing the configuration rail restores focus', () => {
    const markerNoteRules = source.slice(
      source.indexOf('.job-trend-view__curve-marker-slot:hover'),
      source.indexOf('.job-trend-view__curve-marker-core'),
    )

    expect(markerNoteRules).toContain(
      '.job-trend-view__curve-marker-slot:hover .job-trend-view__curve-marker-note',
    )
    expect(markerNoteRules).not.toContain(':focus-within')
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
    expect(source).toContain('{{ safeTrendCurveLabel(row) }}')
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

  it('derives drilldown bounds from the responsive workspace instead of a fixed rail width', () => {
    expect(source).toContain('const workspaceRight =')
    expect(source).toContain('window.innerWidth >= 1440 ? 420')
    expect(source).toContain('window.innerWidth >= 1180 ? 368 : 0')
  })

  it('keeps the detail sheet rounded while scrolling its content inside the frame', () => {
    expect(source).toContain('job-trend-view__detail-content')
    expect(source).toMatch(
      /\.job-trend-view__detail-drawer \{[\s\S]*?display:\s*flex;[\s\S]*?overflow:\s*hidden;/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__detail-content \{[\s\S]*?min-height:\s*0;[\s\S]*?overflow-y:\s*auto;/u,
    )
    const mobileStyles = source.slice(source.indexOf('@media (max-width: 760px)'))
    expect(mobileStyles).toMatch(
      /\.job-trend-detail-enter-from \{[\s\S]*?opacity:\s*1;[\s\S]*?transform:\s*translateY\(0\);/u,
    )
  })

  it('defines four explicit responsive modes for the trend workspace', () => {
    expect(source).toMatch(/@media \(min-width:\s*1440px\)/u)
    expect(source).toMatch(
      /@media \(min-width:\s*1180px\) and \(max-width:\s*1439px\)/u,
    )
    expect(source).toMatch(
      /@media \(min-width:\s*761px\) and \(max-width:\s*1179px\)/u,
    )
    expect(source).toMatch(/@media \(max-width:\s*760px\)/u)
  })

  it('uses the right rail width to reflow the chart workspace', () => {
    const compactStyles = source.slice(source.indexOf('@media (min-width: 761px)'))

    expect(source).toContain('job-trend-view__rail--desktop-expanded')
    expect(source).toContain('job-trend-view__rail--desktop-collapsed')
    expect(compactStyles).toMatch(
      /\.job-trend-view \{[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\)\s*48px;/u,
    )
    expect(compactStyles).toMatch(
      /\.job-trend-view--configuration-expanded \{[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\)\s*320px;/u,
    )
    const mediumStyles = source.match(
      /@media \(min-width:\s*1180px\) and \(max-width:\s*1439px\) \{([\s\S]*?)\n\}/u,
    )?.[1]
    expect(mediumStyles).not.toMatch(
      /\.job-trend-view__rail \{[\s\S]*?width:\s*320px;/u,
    )
  })

  it('keeps collapsed marker slots centered at the medium desktop breakpoint', () => {
    const mediumStart = source.indexOf(
      '@media (min-width: 1180px) and (max-width: 1439px)',
    )
    const nextMedia = source.indexOf('@media', mediumStart + 1)
    const mediumStyles = source.slice(mediumStart, nextMedia)

    expect(mediumStyles).toMatch(
      /\.job-trend-view__rail--desktop-expanded\s*\{[\s\S]*?padding:\s*18px 14px 12px;/u,
    )
    expect(mediumStyles).not.toMatch(/\.job-trend-view__rail\s*\{/u)
  })

  it('keeps compact desktop configuration bounded and its footer outside the scrolling list', () => {
    expect(source).toMatch(
      /\.job-trend-view__rail--drawer \{[\s\S]*?max-width:\s*min\(368px, 92vw\);[\s\S]*?overflow:\s*hidden;/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__rail--drawer \.job-trend-view__curve-list \{[\s\S]*?overflow-y:\s*auto;/u,
    )
    expect(source).toMatch(
      /\.job-trend-view__rail--drawer \.job-trend-view__rail-footer \{[\s\S]*?flex:\s*0 0 auto;/u,
    )
  })
})
