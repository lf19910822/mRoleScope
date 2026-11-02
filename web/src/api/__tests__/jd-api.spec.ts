import { describe, expect, it } from 'vitest'

import { jobListPath } from '../jd-api'
import type { JDListState } from '@/lib/list-state'

describe('JD browser list request', () => {
  it('sends same-dimension selections as repeated values and intersects dimensions', () => {
    const state: JDListState = {
      activityStates: ['effective'],
      educationLevels: ['bachelor', 'master'],
      minimumExperienceYear: 3,
      firstDiscoveredFrom: '',
      firstDiscoveredUntil: '',
      isUrgent: undefined,
      locations: ['上海'],
      jobFamilies: ['backend', 'ai_application'],
      page: 1,
      pageSize: 20,
      projects: ['genshin_impact', 'star_rail'],
      sortRules: ['experience_desc', 'first_discovered_desc', 'education_desc', 'urgent_asc'],
      technologyKeywords: ['go', 'python'],
      title: '工程师',
    }

    const request = new URL(jobListPath(state), 'https://jd-observatory.invalid')

    expect(request.pathname).toBe('/api/v1/jobs')
    expect(request.searchParams.getAll('technology')).toEqual(['go', 'python'])
    expect(request.searchParams.getAll('project')).toEqual(['genshin_impact', 'star_rail'])
    expect(request.searchParams.getAll('job_family')).toEqual(['backend', 'ai_application'])
    expect(request.searchParams.getAll('main_job_family')).toEqual([])
    expect(request.searchParams.getAll('sort')).toEqual([
      'experience_desc',
      'first_discovered_desc',
      'education_desc',
      'urgent_asc',
    ])
    expect(request.searchParams.getAll('education')).toEqual(['bachelor', 'master'])
    expect(request.searchParams.get('minimum_experience_year')).toBe('3')
    expect(request.searchParams.getAll('experience_year')).toEqual([])
    expect(request.searchParams.get('location')).toBe('上海')
    expect(request.searchParams.get('title')).toBe('工程师')
    expect(request.searchParams.get('activity_state')).toBe('effective')
  })
})
