import type { ApiPage, SayingListItem } from '~/types'
import { useApi, type ApiQuery } from './useApi'

export function useSayingsApi() {
  const api = useApi()
  return {
    list(query: ApiQuery = {}): Promise<ApiPage<SayingListItem>> {
      return api.getPage<SayingListItem>('/sayings', query)
    },
  }
}
