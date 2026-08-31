import type { ApiPage } from '~/types'
import { useApi, type ApiQuery } from './useApi'
export function useVideosApi(){ const api=useApi(); return { list(q:ApiQuery={}):Promise<ApiPage<import('~/types').VideoListItem>>{return api.getPage('/videos',q)}, getBySlug(slug:string){return api.get<import('~/types').VideoDetail>(`/videos/${encodeURIComponent(slug)}`)} } }
