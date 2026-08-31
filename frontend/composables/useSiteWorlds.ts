import type { PublicWorldSetting } from '~/types'

import { useApi } from './useApi'

export function useSiteWorlds() {
  const api = useApi()

  return useAsyncData<PublicWorldSetting[]>('site-worlds', async () => {
    const response = await api.getList<PublicWorldSetting>('/worlds')
    return response.data
  }, {
    default: () => [],
  })
}
