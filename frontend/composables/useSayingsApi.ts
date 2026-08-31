import type { ApiPage, SayingDetail, SayingListItem } from '~/types'
import { useApi, type ApiQuery } from './useApi'

export function useSayingsApi() {
  const api = useApi()
  return {
    list(query: ApiQuery = {}): Promise<ApiPage<SayingListItem>> {
      return api.getPage<SayingListItem>('/sayings', query)
    },
    getByShortId(shortId: string): Promise<SayingDetail> {
      return api.get<SayingDetail>(`/sayings/${encodeURIComponent(shortId)}`)
    },
  }
}
