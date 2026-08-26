import type {
  EntryCreateRequest,
  EntryDetail,
  EntryListItem,
  EntryListQuery,
  EntryUpdateRequest,
  PaginatedResponse,
} from '../types/api'
import { request, requestPaginated } from './client'

/**
 * Entry endpoints.
 *
 * Note the addressing split: writes and the admin detail read go by `id`, while
 * the public detail read goes by `slug`. That is not an inconsistency — a reader
 * identifies an entry by its URL, and what an editor changes may be the slug
 * itself.
 */

/**
 * GET /api/v1/admin/entries
 *
 * Paginated, so this uses `requestPaginated` — unlike the category list, which
 * has no `meta` at all. Returns every status including drafts, ordered by most
 * recently edited, excluding anything soft deleted.
 *
 * `page_size` is clamped server side (100000 comes back as 50), so a caller
 * cannot use it to pull the whole table in one request.
 *
 * `?category=` is **not supported here** and is silently ignored. Filtering the
 * admin list by category would need a backend change.
 */
export function listEntriesAdmin(
  query: EntryListQuery = {},
): Promise<PaginatedResponse<EntryListItem>> {
  return requestPaginated<EntryListItem>('/admin/entries', {
    query: {
      page: query.page,
      page_size: query.page_size,
      // Left undefined when absent; the client drops undefined values rather
      // than sending `status=undefined`, which would be a 400.
      status: query.status,
    },
  })
}

/**
 * GET /api/v1/admin/entries/:id
 *
 * Every status, including drafts. Unlike a write response, `category` is
 * populated here — this is a read, so it can join.
 */
export function getEntry(id: number): Promise<EntryDetail> {
  return request<EntryDetail>(`/admin/entries/${id}`)
}

/**
 * POST /api/v1/entries
 *
 * 409 with `fields.slug` when the slug is taken. 400 with `fields.category_id`
 * when the category does not exist — the server does not pre-check it, because
 * a read-then-write would still race a delete and the foreign key is the real
 * guarantee.
 */
export function createEntry(body: EntryCreateRequest): Promise<EntryDetail> {
  return request<EntryDetail>('/entries', { method: 'POST', body })
}

/**
 * PATCH /api/v1/entries/:id
 *
 * Changing the slug does not conflict with itself, so an editor resubmitting the
 * original slug is fine.
 *
 * Sending `status` is a 400 by design. `EntryUpdateRequest` has no such field,
 * so that mistake cannot be made from here.
 */
export function updateEntry(id: number, body: EntryUpdateRequest): Promise<EntryDetail> {
  return request<EntryDetail>(`/entries/${id}`, { method: 'PATCH', body })
}

/**
 * DELETE /api/v1/entries/:id
 *
 * Soft delete: 204 first, 404 after. The row stays with a `deleted_at`, so the
 * entry becomes unreachable rather than gone. There is no restore endpoint.
 *
 * **This frees the slug.** The unique index is partial
 * (`WHERE deleted_at IS NULL`), so new content can take the same slug later.
 */
export function deleteEntry(id: number): Promise<void> {
  return request<void>(`/entries/${id}`, { method: 'DELETE' })
}

/**
 * POST /api/v1/entries/:id/publish — `draft` or `archived` becomes `published`.
 *
 * `published_at` is written on the first publish only and never moves again, so
 * unpublishing and republishing keeps the original date.
 */
export function publishEntry(id: number, revision: number): Promise<EntryDetail> {
  return request<EntryDetail>(`/entries/${id}/publish`, {
    method: 'POST',
    body: { revision },
  })
}

/** POST /api/v1/entries/:id/unpublish — back to `draft`, keeping `published_at`. */
export function unpublishEntry(id: number, revision: number): Promise<EntryDetail> {
  return request<EntryDetail>(`/entries/${id}/unpublish`, {
    method: 'POST',
    body: { revision },
  })
}

/**
 * POST /api/v1/entries/:id/archive — becomes `archived`, front end 404s at once.
 *
 * This endpoint is the only way into `archived` after creation: PATCH refuses
 * `status`, and publish/unpublish only write `published` and `draft`.
 */
export function archiveEntry(id: number, revision: number): Promise<EntryDetail> {
  return request<EntryDetail>(`/entries/${id}/archive`, {
    method: 'POST',
    body: { revision },
  })
}
