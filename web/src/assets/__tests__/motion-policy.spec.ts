import { readdirSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(directory, entry.name)
    return entry.isDirectory() ? sourceFiles(path) : [path]
  })
}

function reducedMotionBlock(path: string): string {
  const source = readFileSync(path, 'utf8')
  const block = source.match(/@media \(prefers-reduced-motion: reduce\) \{([\s\S]*?)\n\}/u)

  const policy = block?.[1]
  if (policy === undefined) throw new Error(`Missing reduced-motion policy in ${path}`)
  return policy
}

describe('application motion policy', () => {
  it('honors a host reduced-motion preference for the new side-panel transitions', () => {
    const matchingSources = sourceFiles(resolve(process.cwd(), 'src')).filter(
      (path) =>
        /\.(?:css|vue)$/u.test(path) &&
        readFileSync(path, 'utf8').includes('prefers-reduced-motion'),
    )

    expect(matchingSources).toContain(resolve(process.cwd(), 'src/App.vue'))
    expect(matchingSources).toContain(resolve(process.cwd(), 'src/views/JobTrendView.vue'))
  })

  it('uses a short fade instead of an instant side-panel switch for reduced motion', () => {
    const appPolicy = reducedMotionBlock(resolve(process.cwd(), 'src/App.vue'))
    const trendPolicy = reducedMotionBlock(resolve(process.cwd(), 'src/views/JobTrendView.vue'))

    expect(appPolicy).not.toContain('transition-duration: 1ms')
    expect(appPolicy).toContain('transition: opacity 140ms ease !important')
    expect(trendPolicy).not.toContain('transition-duration: 1ms')
    expect(trendPolicy).toContain(
      'transition: opacity 140ms ease, visibility 0s linear 0s !important',
    )
    expect(trendPolicy).toContain('transform: none !important')
  })
})
