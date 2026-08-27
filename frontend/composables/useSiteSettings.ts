import type { Ref } from 'vue'

import { useApi } from './useApi'

export interface SiteSettings {
  default_theme: string
  revision: number
  updated_at: string
}

export function useSiteSettings(): { data: Ref<SiteSettings>; pending: Ref<boolean>; error: Ref<unknown> } {
  const api = useApi()
  return useAsyncData(
    'site-settings',
    () => api.get<SiteSettings>('/site'),
    { default: () => ({ default_theme: 'ink', revision: 1, updated_at: '' }) },
  )
}
