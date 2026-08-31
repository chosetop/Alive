import type { TagEntriesPage } from '~/types'
import type { ApiQuery } from './useApi'

export function useTagsApi() {
  const api = useApi()
  return {
    getEntries(slug: string, query: ApiQuery = {}) {
      return api.get<TagEntriesPage>(`/tags/${encodeURIComponent(slug)}/entries`, query)
    },
  }
}
