/**
 * Date formatting for display.
 *
 * One constraint shapes every function here: **the server and the browser must
 * produce identical strings**. Nuxt renders the page once on the server and then
 * hydrates it in the browser; if the two disagree on what a date says, Vue
 * reports a hydration mismatch and replaces the markup.
 *
 * The trap is the timezone. `new Date(...).toLocaleDateString()` uses whatever
 * zone the runtime is in, so a server in UTC and a reader in UTC+8 disagree about
 * which day a late-evening timestamp falls on. Every function below therefore
 * pins the zone explicitly rather than trusting the environment.
 *
 * `Asia/Shanghai` is the author's zone: dates are stated as the author
 * experienced them, which is what a personal site means by a date.
 */

const ZONE = 'Asia/Shanghai'
const LOCALE = 'zh-CN'

/**
 * The API sends RFC 3339 with an offset. Returns `null` for absent or malformed
 * input so callers can decide what to show, rather than rendering `Invalid Date`.
 */
function parse(value: string | null): Date | null {
  if (value === null || value === '') return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

/** Extracts calendar parts in the fixed zone, not the runtime's. */
function partsIn(date: Date): { year: string; month: string; day: string } {
  const formatter = new Intl.DateTimeFormat('en-CA', {
    timeZone: ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
  // en-CA gives ISO order, so the parts are unambiguous to pick apart.
  const [year = '', month = '', day = ''] = formatter.format(date).split('-')
  return { year, month, day }
}

/** `2026 年 8 月 25 日`. For an entry's own dateline, where space is available. */
export function formatFullDate(value: string | null): string {
  const date = parse(value)
  if (date === null) return ''
  return new Intl.DateTimeFormat(LOCALE, {
    timeZone: ZONE,
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  }).format(date)
}

/** `08.25`. For list rows, where the year is already stated by the group. */
export function formatMonthDay(value: string | null): string {
  const date = parse(value)
  if (date === null) return ''
  const { month, day } = partsIn(date)
  return `${month}.${day}`
}

/** Four-digit year, used to group the timeline. */
export function formatYear(value: string | null): string {
  const date = parse(value)
  if (date === null) return ''
  return partsIn(date).year
}

/**
 * `2026-08-25`, for the `datetime` attribute of `<time>`.
 *
 * Machine-readable, and the reason the visible text can be abbreviated: crawlers
 * and assistive technology read this, so `08.25` losing the year costs nothing.
 */
export function toDateAttribute(value: string | null): string {
  const date = parse(value)
  if (date === null) return ''
  const { year, month, day } = partsIn(date)
  return `${year}-${month}-${day}`
}

/**
 * The date an entry should be filed under.
 *
 * `happened_at` when it exists, `published_at` otherwise. An entry about a trip
 * belongs to when the trip happened, not to when it was written up. Both are
 * nullable, so this can still return `null` — a draft has no `published_at`, and
 * the public API only serves published entries, but the type permits it and the
 * caller has to handle it.
 */
export function entryDate(entry: {
  happened_at: string | null
  published_at: string | null
}): string | null {
  return entry.happened_at ?? entry.published_at
}
