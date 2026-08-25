import type { ApiErrorCode, ApiErrorPayload } from '~/types'

/**
 * Normalised API failure.
 *
 * Every request rejects with this, so callers never have to dig a `code` out of
 * an `$fetch` error themselves. Branch on `code`; `message` is for humans and
 * may change between releases.
 *
 * `status` is kept alongside `code` because the two are not interchangeable:
 * `INVALID_INPUT` covers both 400 (bad request body) and 405 (method not
 * supported), so telling those apart needs the status.
 */
export class ApiError extends Error {
  readonly code: ApiErrorCode
  readonly status: number
  /** Per-field validation messages, keyed by field name. */
  readonly fields?: Record<string, string>
  /** Quote this when reporting a failure; it locates the matching log line. */
  readonly requestId?: string

  constructor(status: number, payload: ApiErrorPayload) {
    super(payload.message)
    this.name = 'ApiError'
    this.status = status
    this.code = payload.code
    this.fields = payload.fields
    this.requestId = payload.request_id
  }

  /** Missing or expired session. Normal state for a signed-out visitor. */
  get isUnauthorized(): boolean {
    return this.code === 'UNAUTHORIZED'
  }

  /** Wrong username or password, as opposed to "please sign in again". */
  get isInvalidCredentials(): boolean {
    return this.code === 'INVALID_CREDENTIALS'
  }

  get isNotFound(): boolean {
    return this.code === 'NOT_FOUND'
  }

  /** Request was well-formed but conflicts with existing data (slug taken). */
  get isConflict(): boolean {
    return this.code === 'CONFLICT'
  }

  /** A validation failure, as opposed to an unsupported method (405). */
  get isValidationError(): boolean {
    return this.code === 'INVALID_INPUT' && this.status !== 405
  }

  /** Method not allowed. The server reports it as INVALID_INPUT with a 405. */
  get isMethodNotAllowed(): boolean {
    return this.code === 'INVALID_INPUT' && this.status === 405
  }
}

type UnknownRecord = Record<string, unknown>

const isRecord = (value: unknown): value is UnknownRecord =>
  typeof value === 'object' && value !== null

/**
 * Pulls an `{ error: ... }` envelope out of whatever `$fetch` rejected with,
 * falling back to a synthetic INTERNAL/UNAVAILABLE error when the response was
 * not an envelope at all (proxy error page, connection refused).
 */
export function toApiError(cause: unknown): ApiError {
  if (cause instanceof ApiError) return cause

  const status = isRecord(cause) && typeof cause.status === 'number' ? cause.status : 0
  const data = isRecord(cause) ? cause.data : undefined
  const payload = isRecord(data) ? data.error : undefined

  if (isRecord(payload) && typeof payload.code === 'string') {
    return new ApiError(status, {
      code: payload.code as ApiErrorCode,
      message: typeof payload.message === 'string' ? payload.message : 'Request failed',
      fields: isRecord(payload.fields) ? (payload.fields as Record<string, string>) : undefined,
      request_id: typeof payload.request_id === 'string' ? payload.request_id : undefined,
    })
  }

  // No envelope: the request never reached the API, or something in front of it
  // answered instead. Status 0 means the connection itself failed.
  const message = cause instanceof Error ? cause.message : 'Request failed'
  return new ApiError(status, {
    code: status === 0 ? 'UNAVAILABLE' : 'INTERNAL',
    message,
  })
}
