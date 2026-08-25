/**
 * Response envelope shared by every endpoint.
 *
 * Source of truth: docs/api.md section 1.1. The top level has exactly one key:
 * `data` on success, `error` on failure. Clients can therefore tell the two
 * apart without looking at the status code.
 */

/** Success envelope. */
export type ApiSuccess<T> = {
  data: T
}

/** Pagination block. Only present on paginated endpoints. */
export type ApiMeta = {
  page: number
  page_size: number
  /** Total under the *current* filter, not the row count of the table. */
  total: number
}

/** Paginated success envelope. */
export type ApiPage<T> = {
  data: T[]
  meta: ApiMeta
}

/**
 * Machine-readable error codes. Stable once published — branch on these,
 * never on `message`.
 *
 * Note `INVALID_INPUT` maps to both 400 and 405 (unsupported method), so it is
 * not interchangeable with "status === 400". Combine with the HTTP status when
 * the distinction matters. See docs/api.md sections 1.2 and 2.
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

/** Error payload as it appears inside the envelope. */
export type ApiErrorPayload = {
  code: ApiErrorCode
  /** For humans. May change between releases; do not branch on it. */
  message: string
  /** Present on validation failures only. Keyed by field name. */
  fields?: Record<string, string>
  /** Quote this when reporting a failure; it locates the matching log line. */
  request_id?: string
}

/** Failure envelope. */
export type ApiFailure = {
  error: ApiErrorPayload
}

/** Query parameters accepted by every paginated endpoint (docs/api.md 1.3). */
export type PageQuery = {
  /** 1-based. Default 1. Out-of-range pages return an empty array, not an error. */
  page?: number
  /** Default 20, clamped to 50 by the server rather than rejected. */
  page_size?: number
}
