import type { AdminWorldSetting, WorldKey, WorldSetting, WorldStatus, WorldViewMode } from '../types/api'
import { request } from './client'

export function listWorlds(): Promise<WorldSetting[]> {
  return request<WorldSetting[]>('/worlds')
}

export function listAdminWorlds(): Promise<AdminWorldSetting[]> {
  return request<AdminWorldSetting[]>('/admin/worlds')
}

export function updateWorld(
  key: WorldKey,
  body: {
    revision: number
    status?: WorldStatus
    nav_label?: string
    default_view?: WorldViewMode
  },
): Promise<AdminWorldSetting> {
  return request<AdminWorldSetting>(`/admin/worlds/${key}`, { method: 'PATCH', body })
}
