import type {
  Category,
  CategoryUpdateRequest,
  EntryDetail,
  EntryType,
  EntryUpdateRequest,
  EntryVisibility,
} from '../types/api'

/**
 * Builds a PATCH body by diffing a form against the record it was loaded from.
 *
 * This exists so the rule from docs/api.md 1.7 lives in exactly one place. That
 * rule is easy to get wrong and, when wrong, does not fail loudly:
 *
 *  - a field left out       = leave unchanged
 *  - `null`                 = also leave unchanged (Go's json cannot tell an
 *                             explicit null from an absent key: both leave the
 *                             pointer nil)
 *  - `""` or `0`            = clear the field / move to first
 *
 * So clearing a description means sending `{"description": ""}`. Sending
 * `{"description": null}` looks like it should clear it and instead does
 * nothing, with a 200 and an unchanged record to hide the mistake.
 *
 * Sending every field on every save would avoid the diff, but then two people
 * editing different fields of one category would each overwrite the other's
 * change with a stale value. Diffing means a save touches only what was typed.
 *
 * Returns `{}` when nothing changed. **Do not send that** — the server answers
 * an empty patch with 400, on purpose, because a request that changes nothing
 * is a client bug. Callers check for empty and skip the request instead.
 */
export function buildCategoryPatch(
  original: Category,
  form: { name: string; slug: string; description: string; sortOrder: number },
): CategoryUpdateRequest {
  const patch: CategoryUpdateRequest = {}

  // name and slug are non-nullable and length-checked server side, so an empty
  // one is a validation error rather than a clear. The form blocks it before
  // reaching here; if it did arrive, letting the 400 through beats silently
  // dropping the field and reporting success.
  if (form.name !== original.name) patch.name = form.name
  if (form.slug !== original.slug) patch.slug = form.slug

  // description is nullable, so "" is meaningful: it clears the field.
  if (form.description !== original.description) patch.description = form.description

  // 0 is a real position (first), not "unset", so this compares numbers and
  // does not treat 0 as absent.
  if (form.sortOrder !== original.sort_order) patch.sort_order = form.sortOrder

  return patch
}

/** The editable half of an entry, as the editor form holds it. */
export interface EntryFormState {
  title: string
  slug: string
  summary: string
  contentMd: string
  coverUrl: string
  type: EntryType
  visibility: EntryVisibility
  /** `0` = uncategorised. */
  categoryId: number
  /** Empty string = no date. Converted to the zero timestamp when cleared. */
  happenedAt: string
}

/**
 * The zero timestamp, which is how this API spells "clear the date".
 *
 * This is the one awkward corner of the 1.7 convention: `""` is a natural empty
 * string, but a zero timestamp is not a natural empty date. It is still the
 * value the server reads as a clear, and `null` would mean "leave alone".
 */
const ZERO_TIME = '0001-01-01T00:00:00Z'

/**
 * Builds a PATCH body for an entry by diffing the form against what was loaded.
 *
 * Same rules and same reasons as `buildCategoryPatch`, plus two of its own:
 *
 *  - **`status` is never included.** The server rejects it with 400 and points
 *    at the transition endpoints. `EntryUpdateRequest` has no such field, so
 *    this is enforced by the type rather than by remembering.
 *  - **`happened_at` clears with the zero timestamp**, not `""` and not `null`.
 *
 * `category_id` uses `0` for uncategorised, which is a real value here rather
 * than a stand-in for absent — so moving an entry out of a category means
 * sending `0`, and that is a change worth submitting.
 */
export function buildEntryPatch(
  original: EntryDetail,
  form: EntryFormState,
): EntryUpdateRequest {
  const patch: EntryUpdateRequest = {}

  if (form.title !== original.title) patch.title = form.title
  if (form.slug !== original.slug) patch.slug = form.slug
  if (form.summary !== original.summary) patch.summary = form.summary
  if (form.contentMd !== original.content_md) patch.content_md = form.contentMd
  if (form.coverUrl !== original.cover_url) patch.cover_url = form.coverUrl
  if (form.type !== original.type) patch.type = form.type
  if (form.visibility !== original.visibility) patch.visibility = form.visibility
  if (form.categoryId !== original.category_id) patch.category_id = form.categoryId

  // The form holds `happened_at` as a `datetime-local` string while the record
  // holds RFC 3339 or null, so these are compared after normalising both to the
  // form's representation. Comparing the raw strings would report a change on
  // every save.
  const originalHappened = toFormDateTime(original.happened_at)
  if (form.happenedAt !== originalHappened) {
    patch.happened_at = form.happenedAt === '' ? ZERO_TIME : fromFormDateTime(form.happenedAt)
  }

  return patch
}

/**
 * RFC 3339 to the `datetime-local` input format (`YYYY-MM-DDTHH:mm`), in local
 * time. Returns `""` for null and for the zero timestamp, both of which mean
 * "no date".
 */
export function toFormDateTime(value: string | null): string {
  if (value === null || value === '') return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  // Year 1 means the field was cleared, not that something happened in year 1.
  if (date.getUTCFullYear() <= 1) return ''

  const pad = (n: number): string => String(n).padStart(2, '0')
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
    `T${pad(date.getHours())}:${pad(date.getMinutes())}`
  )
}

/** `datetime-local` back to RFC 3339 with an offset, so the zone is explicit. */
export function fromFormDateTime(value: string): string {
  return new Date(value).toISOString()
}

/** True when a patch would change nothing, so the request must be skipped. */
export function isEmptyPatch(patch: object): boolean {
  return Object.keys(patch).length === 0
}
