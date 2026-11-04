import { describe, expect, it } from 'vitest'

import { formatEducation, formatExperience } from '../job-display'

describe('formal JD field display', () => {
  it('keeps education upper bounds visible instead of treating them as a minimum', () => {
    expect(formatEducation({ explicit_requirement: true, upper_bound: 'bachelor' })).toBe(
      '本科及以下',
    )
    expect(formatEducation({ explicit_requirement: true, lower_bound: 'master' })).toBe(
      '硕士及以上',
    )
  })

  it('never turns an absent or non-configured requirement into zero', () => {
    expect(formatEducation(undefined)).toBe('-')
    expect(formatEducation({ explicit_requirement: false })).toBe('-')
    expect(formatExperience(undefined)).toBe('-')
    expect(formatExperience({ explicit_requirement: false })).toBe('-')
  })
})
