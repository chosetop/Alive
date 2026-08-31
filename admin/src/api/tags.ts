import { request } from './client'

export interface Tag { id: number; name: string; slug: string; usage_count?: number }
export function listTags(query: { q?: string; limit?: number } = {}): Promise<Tag[]> {
  return request<Tag[]>('/admin/tags', { query })
}
export function replaceEntryTags(id: number, revision: number, tagIds: number[]): Promise<{ revision: number; tag_ids: number[] }> {
  return request(`/admin/entries/${id}/tags`, { method: 'PUT', body: { revision, tag_ids: tagIds } })
}

export function createTag(body: { name: string; slug: string }): Promise<Tag> {
  return request<Tag>('/admin/tags', { method: 'POST', body })
}

export function updateTag(id: number, body: { name?: string; slug?: string }): Promise<Tag> {
  return request<Tag>(`/admin/tags/${id}`, { method: 'PATCH', body })
}

export function deleteTag(id: number): Promise<void> {
  return request<void>(`/admin/tags/${id}`, { method: 'DELETE' })
}
