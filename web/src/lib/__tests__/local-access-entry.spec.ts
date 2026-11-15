import { describe, expect, it } from 'vitest'

import { localAccessEntryPath } from '@/lib/local-access-entry'

describe('local access entry', () => {
  it.each(['/access', '/access/'])('maps %s to the invite page', (url) => {
    expect(localAccessEntryPath(url)).toBe('/access/index.html')
  })

  it('preserves query parameters', () => {
    expect(localAccessEntryPath('/access?from=local')).toBe('/access/index.html?from=local')
  })

  it('does not rewrite application routes', () => {
    expect(localAccessEntryPath('/job-trends')).toBeUndefined()
    expect(localAccessEntryPath(undefined)).toBeUndefined()
  })
})
