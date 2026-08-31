/**
 * Categories. Source of truth: docs/api.md section 5.
 *
 * Categories are site navigation: no owner, no soft delete, no visibility, no
 * nesting. `name` and `slug` cap at 64 characters.
 *
 * The public and admin shapes differ, so they are separate types: the public
 * list carries `entry_count` and no timestamps, the admin reads carry
 * timestamps and no count.
 */
import type { WorldKey } from './world'

/**
 * `GET /categories` item. Public, unpaginated — so the response has no `meta`
 * block. Do not parse it as a paginated response.
 */
export type Category = {
  id: number
  world: WorldKey
  name: string
  slug: string
  description: string
  /** Manual ordering, ascending. 0 is a real position (first), not "unset". */
  sort_order: number
  /**
   * Counts only entries a reader can reach: not deleted, published, public.
   * Drafts, archived, private and unlisted are all excluded, so this matches
   * the list you get after opening the category.
   */
  entry_count: number
}

/**
 * Shape returned by `POST /categories`, `PATCH /categories/:id`,
 * `GET /admin/categories` and `GET /admin/categories/:id`.
 *
 * Timestamps instead of `entry_count`: an editor needs to know when a category
 * last changed, and producing the count here would add a join to every write.
 */
export type AdminCategory = {
  id: number
  world: WorldKey
  name: string
  slug: string
  description: string
  sort_order: number
  created_at: string
  updated_at: string
}

/** Body for `POST /categories`. */
export type CategoryCreateBody = {
  world: WorldKey
  /** Required, max 64 characters. */
  name: string
  /** Required, max 64, same format as an entry slug. */
  slug: string
  description?: string
  /** Defaults to 0, which is a real position rather than "unspecified". */
  sort_order?: number
}

/**
 * Body for `PATCH /categories/:id`. Field conventions are the same as entries
 * (docs/api.md 1.7): omitted or `null` leaves a field alone, an empty value
 * clears it. An empty body is a 400.
 */
export type CategoryUpdateBody = {
  name?: string | null
  slug?: string | null
  /** `''` clears the description. */
  description?: string | null
  /** `0` moves the category to the front. */
  sort_order?: number | null
}

export type CategoryListQuery = {
  world?: WorldKey
}
