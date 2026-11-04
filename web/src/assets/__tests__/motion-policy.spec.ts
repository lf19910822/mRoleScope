import { readdirSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(directory, entry.name)
    return entry.isDirectory() ? sourceFiles(path) : [path]
  })
}

describe('application motion policy', () => {
  it('does not suppress application motion for a host reduced-motion preference', () => {
    const matchingSources = sourceFiles(resolve(process.cwd(), 'src')).filter(
      (path) =>
        /\.(?:css|vue)$/u.test(path) &&
        readFileSync(path, 'utf8').includes('prefers-reduced-motion'),
    )

    expect(matchingSources).toEqual([])
  })
})
