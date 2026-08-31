/**
 * Content (entries). Source of truth: docs/api.md section 4.
 *
 * The same row is served in four different shapes depending on the endpoint, so
 * they are separate types rather than one type with optional fields. A single
 * optional-everything type would let a page read `entry.id` off a public list
 * item, which the server never sends.
 */
import type { WorldKey } from './world'

/**
 * Six values, enforced by a database CHECK constraint. Only `journal` has an
 * implemented `meta` structure so far; adding a seventh value requires a
 * migration server-side.
 */
/**
 * `draft` and `archived` are distinct on purpose: a draft is a todo queue, an
 * archived entry is not.
 */
export type EntryStatus = 'draft' | 'published' | 'archived'

/**
 * `unlisted` is the asymmetric one: absent from lists, counts and sitemaps, but
 * reachable by slug. It is not access control — slugs are guessable. Use
 * `private` for anything that must stay unreachable.
 */
export type EntryVisibility = 'public' | 'unlisted' | 'private'

/** Category label nested into read responses. `null` when uncategorised. */
export type EntryCategoryRef = {
  id: number
  name: string
  slug: string
}

/**
 * `GET /journals` item.
 *
 * No `content_md` (the list does not render bodies) and no `id` (readers
 * address entries by slug; entry ids run consecutively across drafts, so
 * exposing them would report how much unpublished work exists).
 */
export type EntryListItem = {
  world: WorldKey
  kind: string
  title: string
  slug: string
  summary: string
  cover_url: string
  /** Always an object, never null — the column is NOT NULL. */
  meta: Record<string, unknown>
  word_count: number
  category: EntryCategoryRef | null
  /** Nullable: not every entry corresponds to a date. */
  happened_at: string | null
  published_at: string | null
}

/**
 * `GET /journals/:slug`.
 *
 * List shape plus `content_md` and `updated_at`. No `status`/`visibility`:
 * this endpoint only ever returns published entries, so both would be
 * constants in every response.
 */
export type EntryDetail = EntryListItem & {
  content_md: string
  updated_at: string
}

/**
 * Author-facing shape, returned by create, PATCH, the three status transitions,
 * and the admin detail read.
 *
 * Caveat from docs/api.md 4.3: on *write* responses `category_id` has a value
 * while `category` is `null` (Postgres `RETURNING` sees only the written row).
 * Read `category_id` to confirm a save. On admin *reads* `category` is filled in.
 */
export type EntryAuthorView = {
  id: number
  world: WorldKey
  kind: string
  title: string
  slug: string
  summary: string
  content_md: string
  cover_url: string
  status: EntryStatus
  visibility: EntryVisibility
  meta: Record<string, unknown>
  /** Server-computed. Not accepted from the client. */
  word_count: number
  /** 0 means uncategorised. */
  category_id: number
  category: EntryCategoryRef | null
  happened_at: string | null
  published_at: string | null
  created_at: string
  updated_at: string
}

/**
 * `GET /admin/entries` item: public list shape plus id and the editorial
 * fields. Still no `content_md` — the admin list is a todo queue.
 */
export type AdminEntryListItem = EntryListItem & {
  id: number
  status: EntryStatus
  visibility: EntryVisibility
  created_at: string
  updated_at: string
}

/** Query for `GET /journals`. `category` is a slug, not an id. */
export type EntryListQuery = {
  page?: number
  page_size?: number
  /**
   * Category slug. An unknown slug is a 404, not an empty list. An empty
   * string is treated as absent, which is what a "show all" filter sends.
   */
  category?: string
}

/** Query for `GET /admin/entries`. Does not support `category` yet. */
export type AdminEntryListQuery = {
  page?: number
  page_size?: number
  /** Must be a valid enum value; a typo is a 400, not an empty list. */
  status?: EntryStatus
}

/**
 * Body for `POST /entries`. `title`, `slug`, `content_md` and `world` are
 * required; the rest carry server-side defaults (status `draft`, visibility
 * `public`).
 *
 * `slug` must match `^[a-z0-9]+(-[a-z0-9]+)*$` and is supplied by the client,
 * not derived from the title.
 */
export type EntryCreateBody = {
  title: string
  slug: string
  content_md: string
  world: WorldKey
  status?: EntryStatus
  visibility?: EntryVisibility
  summary?: string
  cover_url?: string
  /** Id, not slug. 0 or omitted means uncategorised. */
  category_id?: number
  meta?: Record<string, unknown>
  happened_at?: string
}

/**
 * Body for `PATCH /entries/:id`. Field conventions (docs/api.md 1.7):
 *
 * - omitted   -> unchanged
 * - `null`    -> also unchanged (JSON decoding cannot distinguish it from absent)
 * - empty value (`''`, `0`) -> cleared
 *
 * So clearing a field means sending an empty value, never `null`. Never widen
 * `undefined` into `null` in a client helper, and never send every field just
 * to make the body look complete.
 *
 * An empty body (`{}`) is a 400, as is a body containing only nulls.
 *
 * `status` is absent by design: PATCH rejects it with a 400. Publishing goes
 * through the dedicated transition endpoints.
 */
export type EntryUpdateBody = {
  title?: string | null
  slug?: string | null
  summary?: string | null
  content_md?: string | null
  cover_url?: string | null
  visibility?: EntryVisibility | null
  /** 0 clears the category. */
  category_id?: number | null
  meta?: Record<string, unknown> | null
  /** Clear with the zero timestamp `'0001-01-01T00:00:00Z'`, not `''`. */
  happened_at?: string | null
}
