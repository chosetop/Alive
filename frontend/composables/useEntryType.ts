import type { EntryType } from '~/types'

/**
 * Per-type presentation rules.
 *
 * This is the design signature: the six `type` values are not one layout with a
 * changing label, they lay out differently. A photo entry leads with its image;
 * a journal entry is text only; a book, film or record carries its own line of
 * metadata. The list is meant to read as a mixed notebook rather than a uniform
 * feed.
 *
 * Presentation lives here rather than inside components so that the rules are in
 * one readable table instead of scattered across template conditionals.
 */

/**
 * How prominent the cover image is.
 *
 * - `lead`    the image comes first, full width, and carries the entry
 * - `inline`  a small image sits beside the text
 * - `none`    the cover is ignored in the list even when one exists
 */
export type CoverTreatment = 'lead' | 'inline' | 'none'

export type EntryTypeStyle = {
  /** Shown in the meta line. Single characters where a word would be noise. */
  label: string
  cover: CoverTreatment
  /**
   * Whether a summary is worth showing in the list.
   *
   * False for photo entries, where the image is the summary and a caption
   * underneath it would repeat what is already visible.
   */
  showSummary: boolean
}

const STYLES: Record<EntryType, EntryTypeStyle> = {
  /* The default and the majority. Deliberately plain: no image, just words. */
  journal: { label: '日志', cover: 'none', showSummary: true },

  /*
   * A cover here is a book jacket or a film still, which is portrait or nearly
   * square and does not want to be full width. Inline keeps it at the size of a
   * physical object next to the text.
   */
  book: { label: '书', cover: 'inline', showSummary: true },
  movie: { label: '影', cover: 'inline', showSummary: true },
  music: { label: '乐', cover: 'inline', showSummary: true },

  /* Place-driven, so the image leads but the words still matter. */
  travel: { label: '行', cover: 'lead', showSummary: true },

  /* The image is the entry. A summary under it would restate the obvious. */
  photo: { label: '影像', cover: 'lead', showSummary: false },
}

export function entryTypeStyle(type: EntryType): EntryTypeStyle {
  return STYLES[type]
}
