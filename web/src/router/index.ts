import { createRouter, createWebHistory, type RouterHistory } from 'vue-router'

import JobDetailView from '@/views/JobDetailView.vue'
import JobListView from '@/views/JobListView.vue'
import JobTrendView from '@/views/JobTrendView.vue'

type JDScrollPosition = {
  behavior?: ScrollOptions['behavior']
  left?: number
  top?: number
}

type JDRouterLocation = { name?: unknown }

export function jdScrollBehavior(
  to: JDRouterLocation,
  from: JDRouterLocation,
  savedPosition: JDScrollPosition | null | undefined,
): JDScrollPosition | false {
  if (savedPosition) {
    return savedPosition
  }
  if (to.name === 'jobs' && from.name === 'jobs') {
    return false
  }
  return { left: 0, top: 0 }
}

export function createJDRouter(history: RouterHistory) {
  return createRouter({
    history,
    routes: [
      { path: '/', redirect: { name: 'jobs' } },
      { path: '/jobs', name: 'jobs', component: JobListView },
      { path: '/jobs/:id', name: 'job-detail', component: JobDetailView, props: true },
      { path: '/job-trends', name: 'job-trends', component: JobTrendView },
    ],
    scrollBehavior: jdScrollBehavior,
  })
}

export default createJDRouter(createWebHistory(import.meta.env.BASE_URL))
