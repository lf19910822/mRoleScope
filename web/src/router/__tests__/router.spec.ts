import { describe, expect, it } from 'vitest'
import { createMemoryHistory } from 'vue-router'

import { createJDRouter, jdScrollBehavior } from '..'

describe('JD browser router', () => {
  it('keeps URL-backed list state when returning from a detail route', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({
      name: 'jobs',
      query: { main_job_family: ['backend', 'frontend'], page: '2', sort: 'education_desc' },
    })
    await router.isReady()
    await router.push({ name: 'job-detail', params: { id: 'jd_opaque' } })

    const returned = new Promise<void>((resolve) => {
      const remove = router.afterEach(() => {
        remove()
        resolve()
      })
    })
    router.back()
    await returned

    expect(router.currentRoute.value.name).toBe('jobs')
    expect(router.currentRoute.value.query).toEqual({
      main_job_family: ['backend', 'frontend'],
      page: '2',
      sort: 'education_desc',
    })
  })

  it('uses the browser saved scroll position when it is available', () => {
    expect(
      jdScrollBehavior({ name: 'jobs' }, { name: 'job-detail' }, { left: 0, top: 480 }),
    ).toEqual({
      left: 0,
      top: 480,
    })
  })

  it('does not force a jump to the top for in-list filters, reset, or pagination', () => {
    expect(jdScrollBehavior({ name: 'jobs' }, { name: 'jobs' }, undefined)).toBe(false)
    expect(jdScrollBehavior({ name: 'job-detail' }, { name: 'jobs' }, undefined)).toEqual({
      left: 0,
      top: 0,
    })
  })

  it('resolves the dedicated trend dashboard route', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'job-trends' })
    await router.isReady()

    expect(router.currentRoute.value.path).toBe('/job-trends')
  })

  it('redirects the previously shared trends address to the canonical dashboard route', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push('/trends')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('job-trends')
    expect(router.currentRoute.value.path).toBe('/job-trends')
  })

  it('keeps the incomplete-card highlight preview separate from the production dashboard', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'job-trend-highlight-preview' })
    await router.isReady()

    expect(router.currentRoute.value.path).toBe('/job-trends/highlight-preview')
    expect(router.currentRoute.value.name).toBe('job-trend-highlight-preview')
  })
})
