import { request } from './client'

export interface Tag { id: number; name: string; slug: string; usage_count: number }
export function listTags(query: { q?: string; limit?: number } = {}): Promise<Tag[]> {
  return request<Tag[]>('/admin/tags', { query })
}
export function replaceEntryTags(id: number, revision: number, tagIds: number[]): Promise<{ revision: number; tag_ids: number[] }> {
  return request(`/admin/entries/${id}/tags`, { method: 'PUT', body: { revision, tag_ids: tagIds } })
}
