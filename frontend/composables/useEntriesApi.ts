import type {
  AdminEntryListItem,
  AdminEntryListQuery,
  ApiPage,
  EntryAuthorView,
  EntryCreateBody,
  EntryDetail,
  EntryListItem,
  EntryListQuery,
  EntryUpdateBody,
} from '~/types'
import { useApi } from './useApi'

/**
 * Entry endpoints. Source of truth: docs/api.md section 4.
 *
 * Addressing differs by audience and is not interchangeable: the public
 * endpoints take a **slug**, the write and admin endpoints take an **id**.
 * Readers identify an entry by its URL, while an editor may be changing the
 * slug itself — addressing writes by slug would turn "rename this" into
 * "replace this resource".
 */
export function useEntriesApi() {
  const api = useApi()

  return {
    /**
     * Public, paginated list for the Journal world. Returns only published +
     * public + not-deleted entries. Items carry neither `content_md` nor `id`.
     *
     * An unknown `category` slug is a 404, not an empty page — a mistyped link
     * and an empty category are different things and must read differently.
     */
    list(query: EntryListQuery = {}): Promise<ApiPage<EntryListItem>> {
      return api.getPage<EntryListItem>('/journals', { ...query })
    },

    /**
     * Public detail by slug in the Journal world. The only endpoint that can
     * reach an `unlisted` entry. Drafts, archived, private and soft-deleted
     * entries all 404 with a response identical to a slug that was never used.
     */
    getBySlug(slug: string): Promise<EntryDetail> {
      return api.get<EntryDetail>(`/journals/${encodeURIComponent(slug)}`)
    },

    /** Requires a session. 201. */
    create(body: EntryCreateBody): Promise<EntryAuthorView> {
      return api.post<EntryAuthorView>('/entries', body)
    },

    /**
     * Requires a session. Partial update by id.
     *
     * Field conventions are the caller's responsibility (docs/api.md 1.7):
     * omitted or `null` leaves a field alone, an empty value clears it. This
     * function passes the body through untouched — it will not convert
     * `undefined` to `null`, and it will not fill in absent fields.
     *
     * `status` is not part of `EntryUpdateBody`: the server rejects it with a
     * 400 that points at the transition endpoints below.
     */
    update(id: number, body: EntryUpdateBody): Promise<EntryAuthorView> {
      return api.patch<EntryAuthorView>(`/entries/${id}`, body)
    },

    /**
     * Requires a session. Soft delete, 204. A second call 404s.
     *
     * There is no hard delete and no restore. Deleting frees the slug for reuse.
     */
    remove(id: number): Promise<void> {
      return api.requestNoContent('DELETE', `/entries/${id}`)
    },

    /**
     * `draft` or `archived` -> `published`.
     *
     * `published_at` is written on first publish only and never moves
     * afterwards, so unpublishing and republishing preserves the original date.
     */
    publish(id: number): Promise<EntryAuthorView> {
      return api.post<EntryAuthorView>(`/entries/${id}/publish`)
    },

    /** `published` -> `draft`. Leaves `published_at` in place. */
    unpublish(id: number): Promise<EntryAuthorView> {
      return api.post<EntryAuthorView>(`/entries/${id}/unpublish`)
    },

    /** -> `archived`. Leaves `published_at` in place. 404s on the public side. */
    archive(id: number): Promise<EntryAuthorView> {
      return api.post<EntryAuthorView>(`/entries/${id}/archive`)
    },

    /**
     * Requires a session. Every status including drafts, most recently edited
     * first. Soft-deleted entries are excluded. Still no `content_md`.
     *
     * A misspelled `status` is a 400, not an empty page. `category` is not
     * supported here and is ignored if sent.
     */
    adminList(query: AdminEntryListQuery = {}): Promise<ApiPage<AdminEntryListItem>> {
      return api.getPage<AdminEntryListItem>('/admin/entries', { ...query })
    },

    /**
     * Requires a session. Detail by id, any status. Unlike the write responses,
     * `category` is populated here.
     */
    adminGet(id: number): Promise<EntryAuthorView> {
      return api.get<EntryAuthorView>(`/admin/entries/${id}`)
    },
  }
}
