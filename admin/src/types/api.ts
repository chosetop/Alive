/**
 * Types for the Alive backend API, transcribed from docs/api.md.
 *
 * Only what is actually called is declared here. Entry payloads arrive with the
 * article list; guessing their shape now would mean writing types nothing
 * checks.
 */

// ---------------------------------------------------------------------------
// Response envelope
// ---------------------------------------------------------------------------

/**
 * Every response body has exactly one top-level key: `data` on success,
 * `error` on failure. A client can tell the two apart without the status code.
 */
export interface ApiResponse<T> {
  data: T
}

/** Page 1-based; `total` counts rows matching the current filter, not the table. */
export interface PaginationMeta {
  page: number
  page_size: number
  total: number
}

export interface PaginatedResponse<T> {
  data: T[]
  meta: PaginationMeta
}

/**
 * Machine-readable error codes. Stable across releases, unlike `message`.
 *
 * `INVALID_CREDENTIALS` and `UNAUTHORIZED` are both 401 but mean different
 * things: the password was wrong, versus the session is gone. The admin shows
 * different things for each.
 *
 * `INVALID_INPUT` is not the same as "400": a 405 also carries this code.
 * Distinguish those by status, not code.
 */
export type ApiErrorCode =
  | 'INVALID_INPUT'
  | 'INVALID_CREDENTIALS'
  | 'UNAUTHORIZED'
  | 'FORBIDDEN'
  | 'NOT_FOUND'
  | 'CONFLICT'
  | 'RATE_LIMITED'
  | 'INTERNAL'
  | 'UNAVAILABLE'

export interface ApiError {
  /** Branch on this. */
  code: ApiErrorCode
  /** For humans. Wording may change, so never branch on it. */
  message: string
  /** Present only on validation failures; keyed by field name. */
  fields?: Record<string, string>
  /** Quote this when reporting a problem: it locates the matching log line. */
  request_id?: string
}

export interface ApiErrorResponse {
  error: ApiError
}

// ---------------------------------------------------------------------------
// Entry enums
// ---------------------------------------------------------------------------

/** Six values, held by a database CHECK constraint. */
export type EntryType = 'journal' | 'book' | 'movie' | 'music' | 'travel' | 'photo'

/** `draft` is a todo queue, `archived` is not. The admin lists them apart. */
export type EntryStatus = 'draft' | 'published' | 'archived'

/** `unlisted` is absent from listings but reachable by slug. It is not access control. */
export type EntryVisibility = 'public' | 'unlisted' | 'private'

export type WorldKey = 'journal' | 'saying' | 'video'
export type WorldStatus = 'unopened' | 'open' | 'hidden'
export type SayingViewMode = 'stream' | 'wall' | 'focus'
export type WorldViewMode = '' | SayingViewMode

export interface WorldSetting {
  world: WorldKey
  nav_label: string
  sort_order: number
  default_view: WorldViewMode
}

export interface AdminWorldSetting extends WorldSetting {
  status: WorldStatus
  revision: number
  updated_at: string
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

/**
 * The account behind the current session.
 *
 * `role` is a plain string, not a union: the column has a default of 'owner'
 * but no CHECK constraint, and the backend records it without ever reading it.
 * A union here would claim a guarantee the schema does not make.
 *
 * `display_name` is nullable in the database and serialises as `""` when
 * unset, so treat empty and absent the same way and fall back to `username`.
 */
export interface User {
  id: number
  username: string
  role: string
  display_name: string
}

export interface LoginRequest {
  username: string
  password: string
}

// ---------------------------------------------------------------------------
// Entries
// ---------------------------------------------------------------------------

/**
 * The category as it appears nested inside an entry: label and link, no more.
 *
 * `null` when uncategorised, which is a normal state rather than missing data.
 * Nested rather than a bare id because everything anyone does with a category
 * needs both the name and something to address it by.
 */
export interface EntryCategoryRef {
  id: number
  name: string
  slug: string
}

/**
 * An entry as `GET /admin/entries` returns it: the public list shape plus `id`,
 * `status`, `visibility` and both timestamps.
 *
 * Saying rows may carry `content_md` so their body, rather than an empty title,
 * can identify them in the writing directory. Other worlds omit it.
 */
export interface EntryListItem {
  id: number
  world: WorldKey
  kind: string
  type?: EntryType
  title: string
  content_md?: string
  slug: string
  summary: string
  cover_url: string
  meta: Record<string, unknown>
  word_count: number
  category: EntryCategoryRef | null
  /** When the thing described happened, not when it was written. Nullable. */
  happened_at: string | null
  /** Set the moment it is first published; `null` while it never has been. */
  published_at: string | null
  status: EntryStatus
  visibility: EntryVisibility
  created_at: string
  updated_at: string
}

/**
 * An entry from the author's side: the list shape plus `content_md` and
 * `category_id`. Returned by create, update, the three status transitions, and
 * `GET /admin/entries/:id`.
 *
 * **`category_id` and `category` disagree on write responses, by design.**
 * Postgres' `RETURNING` sees only the row it wrote, so a write knows the id but
 * has no label to go with it, and `category` comes back `null` even when
 * `category_id` is set. Read `category_id` to confirm what was saved;
 * `category` is populated only on the admin detail read.
 */
export interface EntryDetail extends Omit<EntryListItem, 'category'> {
  revision: number
  content_md: string
  /** `0` means uncategorised, not "unset". */
  category_id: number
  category: EntryCategoryRef | null
  tags?: Array<{ id: number; name: string; slug: string; usage_count?: number }>
}

/**
 * `POST /api/v1/entries`.
 *
 * Creates an incomplete draft. Publication-stage fields are supplied later by
 * revision-aware PATCH requests.
 */
export interface EntryCreateRequest {
  world: WorldKey
  visibility?: EntryVisibility
}

/**
 * `PATCH /api/v1/entries/:id`. Same three-case rule as categories — see
 * docs/api.md 1.7 and `buildEntryPatch`.
 *
 * **`status` is deliberately absent from this type.** The server rejects it with
 * 400 and points at the transition endpoints instead of ignoring it, so a client
 * cannot send `{"status":"published"}`, receive 200, and be left wondering why
 * the entry is still a draft. Publishing goes through `publish` / `unpublish` /
 * `archive`.
 */
export interface EntryUpdateRequest {
  revision: number
  title?: string
  slug?: string
  summary?: string
  content_md?: string
  cover_url?: string
  visibility?: EntryVisibility
  category_id?: number
  meta?: Record<string, unknown>
  happened_at?: string
}

export type EntryPatchFields = Omit<EntryUpdateRequest, 'revision'>

/** Query for `GET /admin/entries`. Omit `status` for every state at once. */
export interface EntryListQuery {
  page?: number
  page_size?: number
  world?: WorldKey
  /**
   * A misspelled value is a 400 with `fields.status`, not an empty list — an
   * empty list would read as "no drafts" when it means "you typed it wrong".
   */
  status?: EntryStatus
  /**
   * Free-text search across title, slug, and summary. Not the body: a common
   * word inside a long article would otherwise bury the article named that.
   *
   * Unlike `status`, an unmatched query is not an error — no match is a real
   * answer about the collection. Blank and whitespace-only are treated as absent
   * by the backend, so the directory search field can send its value on every
   * keystroke including the one that clears it.
   */
  q?: string
  /** Category slug filter; combines with status and q. */
  category?: string
}

// ---------------------------------------------------------------------------
// Categories
// ---------------------------------------------------------------------------

/**
 * A category as the public list returns it, with `entry_count`.
 *
 * The count is of what a reader can reach: undeleted, published, public.
 * Drafts, archived, private and unlisted are all excluded, so it matches the
 * length of the list you get after opening the category.
 *
 * Not used by the admin's own screens — kept because the count is the only
 * place that number exists, and a future "this category has content" warning
 * before deletion would read it.
 */
export interface CategoryWithCount {
  id: number
  world: WorldKey
  name: string
  slug: string
  description: string
  sort_order: number
  entry_count: number
}

/**
 * A category as the admin endpoints return it: timestamps instead of a count.
 *
 * Producing the count here would mean a join on every write response, and the
 * number belongs to the public listing.
 */
export interface Category {
  id: number
  world: WorldKey
  name: string
  slug: string
  description: string
  sort_order: number
  created_at: string
  updated_at: string
}

/** `name` and `slug` are required; both cap at 64. */
export interface CategoryCreateRequest {
  world: WorldKey
  name: string
  slug: string
  description?: string
  /** `0` is a real position (first), not "unspecified". */
  sort_order?: number
}

/**
 * Every field optional, and the difference between the three cases matters —
 * see docs/api.md 1.7:
 *
 *  - absent  = leave unchanged
 *  - `null`  = also leave unchanged (Go cannot tell it from absent)
 *  - `""`/`0` = clear the field / move to first
 *
 * An empty body is 400, not a no-op 200. So is a body containing only `null`s,
 * which is the same request once `null` reads as "not submitted".
 */
export interface CategoryUpdateRequest {
  name?: string
  slug?: string
  description?: string
  sort_order?: number
}

/**
 * Login returns the account only. The session token exists solely in the
 * `Set-Cookie` header, and the cookie is HttpOnly, so no client code sees it.
 */
export type LoginResponse = User
