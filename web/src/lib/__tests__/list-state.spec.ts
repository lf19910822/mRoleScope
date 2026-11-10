import { describe, expect, it } from 'vitest'
import type { LocationQuery } from 'vue-router'

import { listStateFromRoute, listStateToRoute, type JDListState } from '../list-state'

describe('JD list URL state', () => {
  it('retains either last-modified sort direction', () => {
    for (const sort of ['last_modified_asc', 'last_modified_desc']) {
      const state = listStateFromRoute({ sort } as LocationQuery)
      expect(state.sortRules).toEqual([sort])
      expect(listStateToRoute(state).sort).toBe(sort)
    }
  })
  it('round-trips unified job-family filters, ordered sorts, and pagination through the route query', () => {
    const state: JDListState = {
      activityStates: ['effective'],
      educationLevels: ['bachelor', 'master'],
      minimumExperienceYear: 3,
      firstDiscoveredFrom: '2026-10-01',
      firstDiscoveredUntil: '2026-10-31',
      isUrgent: true,
      locations: ['上海', '北京'],
      jobFamilies: ['backend', 'frontend'],
      page: 2,
      pageSize: 25,
      projects: ['genshin_impact'],
      sortRules: ['experience_desc', 'first_discovered_desc', 'education_desc', 'urgent_asc'],
      technologyKeywords: ['go', 'python'],
      title: '服务端',
    }

    expect(listStateFromRoute(listStateToRoute(state))).toEqual(state)
  })

  it('defaults a route without state to still-effective jobs and the first page', () => {
    expect(listStateFromRoute({} as LocationQuery)).toMatchObject({
      activityStates: ['effective'],
      page: 1,
      pageSize: 20,
      sortRules: [],
    })
  })

  it('represents the explicit all-status choice without losing its URL meaning', () => {
    const state = listStateFromRoute({ activity_state: 'all' } as LocationQuery)

    expect(state.activityStates).toEqual(['effective', 'confirmed_closed'])
    expect(listStateToRoute(state).activity_state).toBe('all')
  })

  it('interprets retained legacy job-family and single-sort URLs as the current controls', () => {
    expect(
      listStateFromRoute({
        main_job_family: 'backend',
        other_job_family: 'security',
        sort: 'education_desc',
      } as LocationQuery),
    ).toMatchObject({
      jobFamilies: ['backend', 'security'],
      sortRules: ['education_desc'],
    })
  })

  it('keeps the zero-year compatibility URL while the browser state supports the visible one-through-nine range', () => {
    expect(
      listStateFromRoute({
        minimum_experience_year: '0',
        experience_year: ['3', '5'],
      } as LocationQuery),
    ).toMatchObject({ minimumExperienceYear: 0 })

    const state = listStateFromRoute({ minimum_experience_year: '9' } as LocationQuery)
    expect(state.minimumExperienceYear).toBe(9)
    expect(listStateToRoute(state).minimum_experience_year).toBe('9')
    expect(
      listStateFromRoute({ minimum_experience_year: '10' } as LocationQuery).minimumExperienceYear,
    ).toBeUndefined()
  })
})
