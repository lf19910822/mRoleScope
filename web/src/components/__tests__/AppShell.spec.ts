import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'

import App from '@/App.vue'
import { createJDRouter } from '@/router'

afterEach(() => vi.unstubAllGlobals())

describe('application navigation shell', () => {
  it('keeps the desktop function rail fixed while the browser owns page scrolling', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/App.vue'), 'utf8')
    const shellRule = source.match(/\.app-shell \{(?<declarations>[\s\S]*?)\n\}/u)?.groups
      ?.declarations
    const railRule = source.match(/\.app-sidebar \{(?<declarations>[\s\S]*?)\n\}/u)?.groups
      ?.declarations
    const contentRule = source.match(/\.app-shell__content \{(?<declarations>[\s\S]*?)\n\}/u)
      ?.groups?.declarations

    expect(shellRule).toMatch(/min-height:\s*100vh;/u)
    expect(shellRule).not.toMatch(/height:\s*100dvh;/u)
    expect(shellRule).not.toMatch(/overflow:\s*hidden;/u)
    expect(railRule).toMatch(/height:\s*100dvh;/u)
    expect(railRule).toMatch(/position:\s*fixed;/u)
    expect(contentRule).toMatch(/overflow:\s*visible;/u)
  })

  it('returns the application shell to ordinary document flow at narrow widths', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/App.vue'), 'utf8')
    const narrowStyles = source.slice(source.indexOf('@media (max-width: 760px)'))

    expect(narrowStyles).toMatch(
      /\.app-shell \{[\s\S]*?display:\s*block;[\s\S]*?height:\s*auto;[\s\S]*?overflow:\s*visible;/u,
    )
    expect(narrowStyles).toMatch(/\.app-shell__content \{[\s\S]*?overflow:\s*visible;/u)
  })

  it('releases the trend dashboard beside the job browser and keeps the browser active for list and detail routes', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(App, {
      global: { plugins: [router], stubs: { RouterView: true } },
    })

    const navigation = wrapper.get('[data-testid="application-navigation"]')
    expect(navigation.text()).toContain('岗位筛选')
    expect(navigation.text()).toContain('趋势看板')
    const links = navigation.findAll('a')
    expect(links[0]?.classes()).toContain('app-navigation__link--active')
    expect(links[1]?.classes()).not.toContain('app-navigation__link--active')

    await router.push({ name: 'job-detail', params: { id: 'jd_opaque' } })
    await flushPromises()
    expect(links[0]?.classes()).toContain('app-navigation__link--active')

    await router.push({ name: 'job-trends' })
    await flushPromises()
    expect(links[0]?.classes()).not.toContain('app-navigation__link--active')
    expect(links[1]?.classes()).toContain('app-navigation__link--active')

    wrapper.unmount()
  })

  it('reuses the mounted job list when returning from detail or trends', async () => {
    const requests: string[] = []
    const job = {
      activity_state: 'effective',
      education: { explicit_requirement: true, upper_bound: 'bachelor' },
      experience: { explicit_requirement: false },
      first_discovered_date: '2026-10-30',
      id: 'jd_opaque',
      is_urgent: false,
      locations: [],
      other_job_families: [],
      projects: [],
      technology_keywords: [],
      title: '服务端工程师',
    }
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const path = String(input)
        requests.push(path)
        if (path === '/api/v1/jobs/filter-options') {
          return new Response(
            JSON.stringify({
              activity_states: [],
              education_levels: [],
              experience_years: [],
              job_families: [],
              locations: [],
              main_job_families: [],
              other_job_families: [],
              projects: [],
              technology_keywords: [],
              urgent_options: [],
            }),
          )
        }
        if (path === '/api/v1/jobs/jd_opaque') {
          return new Response(
            JSON.stringify({
              ...job,
              body: { requirements: '熟悉 Go', responsibilities: '负责服务端开发' },
            }),
          )
        }
        if (path === '/api/v1/jobs/jd_opaque/history') {
          return new Response(JSON.stringify({ nodes: [] }))
        }
        return new Response(
          JSON.stringify({ items: [job], page: { number: 1, page_size: 20, total: 1 } }),
        )
      }),
    )

    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs', query: { title: '服务端', page: '2' } })
    await router.isReady()
    const wrapper = mount(App, { global: { plugins: [router] } })
    await flushPromises()

    const listRequestCount = () =>
      requests.filter((path) => path.startsWith('/api/v1/jobs?')).length
    const filterOptionsRequestCount = () =>
      requests.filter((path) => path === '/api/v1/jobs/filter-options').length
    expect(listRequestCount()).toBe(1)
    expect(filterOptionsRequestCount()).toBe(1)

    await router.push({ name: 'job-detail', params: { id: 'jd_opaque' } })
    await flushPromises()
    await router.back()
    await flushPromises()
    expect(listRequestCount()).toBe(1)
    expect(filterOptionsRequestCount()).toBe(1)

    await router.push({ name: 'job-trends', query: { title: 'unrelated' } })
    await flushPromises()
    expect(listRequestCount()).toBe(1)
    const filterCountAfterTrend = filterOptionsRequestCount()
    await wrapper.get('[data-testid="application-navigation"] a').trigger('click')
    await flushPromises()
    expect(listRequestCount()).toBe(1)
    expect(filterOptionsRequestCount()).toBe(filterCountAfterTrend)
    expect(router.currentRoute.value.query).toEqual({ title: '服务端', page: '2' })

    await router.push({ name: 'jobs', query: { title: '新的筛选' } })
    await flushPromises()
    expect(listRequestCount()).toBe(2)

    await router.push({ name: 'job-trends' })
    await flushPromises()
    await router.push({ name: 'jobs', query: { title: '统计入口' } })
    await flushPromises()
    expect(listRequestCount()).toBe(3)
    const listRequests = requests.filter((path) => path.startsWith('/api/v1/jobs?'))
    expect(listRequests[listRequests.length - 1]).toContain(
      'title=%E7%BB%9F%E8%AE%A1%E5%85%A5%E5%8F%A3',
    )

    wrapper.unmount()
  })

  it('opens and closes the narrow-viewport navigation without leaving an overlay behind', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(App, {
      global: { plugins: [router], stubs: { RouterView: true } },
    })

    await wrapper.get('[data-testid="open-navigation"]').trigger('click')
    expect(wrapper.find('[data-testid="navigation-overlay"]').exists()).toBe(true)
    await wrapper.get('[data-testid="close-navigation"]').trigger('click')
    expect(wrapper.find('[data-testid="navigation-overlay"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('exposes the active colour scheme at the application shell seam', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(App, {
      global: { plugins: [router], stubs: { RouterView: true } },
    })

    expect(wrapper.get('.app-shell').attributes('data-color-scheme')).toBe('light')

    wrapper.unmount()
  })
})
