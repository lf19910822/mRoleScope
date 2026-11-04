import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { createMemoryHistory } from 'vue-router'

import App from '@/App.vue'
import { createJDRouter } from '@/router'

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
