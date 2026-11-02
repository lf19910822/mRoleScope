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
    expect(formatEducation(undefined)).toBe('未明确学历要求')
    expect(formatEducation({ explicit_requirement: false })).toBe('未明确学历要求')
    expect(formatExperience(undefined)).toBe('未明确工作年限')
    expect(formatExperience({ explicit_requirement: false })).toBe('未明确工作年限')
  })
})
