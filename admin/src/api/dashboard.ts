import { request } from './client'

export interface DashboardMetrics {
  total_entries: number
  published_entries: number
  total_words: number
}

/** GET /api/v1/admin/dashboard */
export function getDashboardMetrics(): Promise<DashboardMetrics> {
  return request<DashboardMetrics>('/admin/dashboard')
}
