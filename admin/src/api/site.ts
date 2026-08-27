import { request } from './client'

export interface SiteSettings {
  default_theme: string
  revision: number
  updated_at: string
}

export interface UpdateSiteSettingsInput {
  default_theme: string
  revision: number
}

export function getSiteSettings(): Promise<SiteSettings> {
  return request<SiteSettings>('/site')
}

export function updateSiteSettings(input: UpdateSiteSettingsInput): Promise<SiteSettings> {
  return request<SiteSettings>('/admin/site', {
    method: 'PATCH',
    body: input,
  })
}
