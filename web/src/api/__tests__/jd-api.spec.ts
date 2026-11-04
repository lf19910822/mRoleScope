import { describe, expect, it } from 'vitest'

import { jobListPath, jobTrendPath } from '../jd-api'
import type { TrendRequest } from '@/lib/job-trend-dashboard-state'
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

  it('sends exactly one bounded historical trend condition without internal data selectors', () => {
    const request: TrendRequest = {
      condition: {
        dimension: 'job_family',
        includeSecondary: true,
        operator: '',
        value: 'client',
      },
      range: 'all',
    }

    const path = new URL(jobTrendPath(request), 'https://jd-observatory.invalid')

    expect(path.pathname).toBe('/api/v1/job-trends')
    expect(path.searchParams.get('range')).toBe('all')
    expect(path.searchParams.get('dimension')).toBe('job_family')
    expect(path.searchParams.get('value')).toBe('client')
    expect(path.searchParams.get('include_secondary')).toBe('true')
    expect(path.searchParams.get('operator')).toBeNull()
    expect([...path.searchParams.keys()]).toEqual([
      'range',
      'dimension',
      'value',
      'include_secondary',
    ])
  })
})
