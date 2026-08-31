import { request } from './client'

export interface MediaRecord {
  id: number
  url: string
  object_key: string
  mime_type: string
  byte_size: number
  width?: number
  height?: number
  created_at: string
}

export interface PresignResult {
  object_key: string
  upload: { url: string; method: string; headers: Record<string, string>; expires_at: string }
}

export function presignMedia(body: { entry_id: number; filename: string; mime_type: string; size_bytes: number }): Promise<PresignResult> {
  return request<PresignResult>('/admin/media/presign', { method: 'POST', body })
}

export function registerMedia(body: { entry_id: number; object_key: string; mime_type: string; size_bytes: number }): Promise<MediaRecord> {
  return request<MediaRecord>('/admin/media', { method: 'POST', body })
}

export function listEntryMedia(entryId: number): Promise<MediaRecord[]> {
  return request<MediaRecord[]>(`/admin/entries/${entryId}/media`)
}

export function setPrimaryVideo(entryId: number, body: { media_id: number; revision: number }): Promise<{ revision: number; media_id: number }> {
  return request(`/admin/entries/${entryId}/primary-video`, { method: 'PUT', body })
}
