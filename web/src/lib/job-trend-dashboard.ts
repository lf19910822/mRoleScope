import { getJobTrend } from '@/api/jd-api'

import { createTrendDashboardState } from './job-trend-dashboard-state'

const dashboard = createTrendDashboardState({ readTrend: getJobTrend })

export function useJobTrendDashboard() {
  return dashboard
}
