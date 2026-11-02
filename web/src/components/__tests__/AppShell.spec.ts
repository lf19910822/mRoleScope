import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createMemoryHistory } from 'vue-router'

import App from '@/App.vue'
import { createJDRouter } from '@/router'

describe('application navigation shell', () => {
  it('releases only the job browser and keeps it active for list and detail routes', async () => {
    const router = createJDRouter(createMemoryHistory())
    await router.push({ name: 'jobs' })
    await router.isReady()
    const wrapper = mount(App, {
      global: { plugins: [router], stubs: { RouterView: true } },
    })

    const navigation = wrapper.get('[data-testid="application-navigation"]')
    expect(navigation.text()).toContain('岗位筛选')
    expect(navigation.text()).not.toMatch(/图表|分析|仪表盘|即将推出/i)
    expect(navigation.get('a').classes()).toContain('app-navigation__link--active')

    await router.push({ name: 'job-detail', params: { id: 'jd_opaque' } })
    await flushPromises()
    expect(navigation.get('a').classes()).toContain('app-navigation__link--active')

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
})
