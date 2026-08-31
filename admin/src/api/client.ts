import type { ApiError, ApiErrorResponse, ApiResponse, PaginatedResponse } from '../types/api'
import { ApiClientError, NETWORK_ERROR } from './errors'

/**
 * The single place this app talks HTTP.
 *
 * Two things every caller gets for free and none of them should have to
 * remember:
 *
 *  - `credentials: "include"`. The session is an HttpOnly cookie, and the
 *    admin runs on a different origin from the API in development, so without
 *    this the cookie is neither stored nor sent and login silently does
 *    nothing.
 *  - The envelope is unwrapped, and a failure envelope becomes a thrown
 *    `ApiClientError`. Callers deal in `User`, not in `{ data: User }`.
 *
 * There is no token handling here on purpose. The cookie is HttpOnly, so it is
 * unreadable by script and travels automatically; a token in localStorage would
 * be both readable by any injected script and a second source of truth for
 * "am I logged in".
 */

const BASE_URL = import.meta.env.VITE_API_BASE_URL
const API_PREFIX = '/api/v1'

if (!BASE_URL) {
  // Failing loudly at startup beats every request 404ing against the dev
  // server's own origin, which looks like a backend problem.
  throw new Error('VITE_API_BASE_URL is not set. Copy .env.example to .env.')
}

/** Strip one trailing slash so joining paths cannot produce `//api/v1`. */
const ORIGIN = BASE_URL.replace(/\/$/, '')

/**
 * Called when a request fails with 401 outside of the login flow, meaning the
 * session expired or was revoked. The auth store registers a handler at
 * startup; the client stays unaware of Pinia and the router, which keeps this
 * module free of an import cycle (store -> api -> store).
 */
type UnauthorizedHandler = () => void
let onUnauthorized: UnauthorizedHandler | null = null

export function setUnauthorizedHandler(handler: UnauthorizedHandler): void {
  onUnauthorized = handler
}

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
  /** Serialised as JSON. Omit for GET and DELETE. */
  body?: unknown
  query?: Record<string, string | number | undefined>
  /**
   * Set on the login endpoint. A 401 there means "wrong password" and must not
   * trigger the session-expired path, which would clear state and redirect
   * while the person is already looking at the login form.
   */
  skipUnauthorizedHandler?: boolean
  signal?: AbortSignal
}

/**
 * Performs a request and returns the whole success envelope, so paginated
 * callers can read `meta`.
 *
 * Throws `ApiClientError` for every failure, including transport failures, so
 * callers have one thing to catch.
 */
async function requestEnvelope<T>(
  path: string,
  options: RequestOptions = {},
): Promise<ApiResponse<T> & { meta?: unknown }> {
  const { method = 'GET', body, query, skipUnauthorizedHandler = false, signal } = options

  const url = new URL(ORIGIN + API_PREFIX + path)
  if (query) {
    for (const [key, value] of Object.entries(query)) {
      if (value !== undefined) url.searchParams.set(key, String(value))
    }
  }

  const init: RequestInit = {
    method,
    // The session cookie rides on this. Without it there is no session.
    credentials: 'include',
    headers: { Accept: 'application/json' },
    ...(signal ? { signal } : {}),
  }

  if (body !== undefined) {
    init.headers = { ...init.headers, 'Content-Type': 'application/json' }
    init.body = JSON.stringify(body)
  }

  let response: Response
  try {
    response = await fetch(url, init)
  } catch (cause) {
    // fetch rejects only when no response arrived at all: offline, DNS
    // failure, CORS rejection, or an abort.
    throw new ApiClientError({
      code: NETWORK_ERROR,
      status: 0,
      message: cause instanceof Error ? cause.message : 'network request failed',
    })
  }

  // 204 carries no body. logout is the one endpoint that returns it.
  if (response.status === 204) {
    return { data: undefined as T }
  }

  const payload = await readJson(response)

  if (!response.ok) {
    const error = extractError(payload)
    // A 401 outside the login flow means the session is gone. Handle it here
    // once rather than in every caller.
    if (response.status === 401 && !skipUnauthorizedHandler) {
      onUnauthorized?.()
    }
    throw new ApiClientError({
      code: error?.code ?? 'INTERNAL',
      status: response.status,
      message: error?.message ?? `request failed with status ${response.status}`,
      fields: error?.fields,
      requestId: error?.request_id,
      retryAfter: parseRetryAfter(response.headers.get('Retry-After')),
    })
  }

  // A 2xx whose body is not a `data` envelope means the response did not come
  // from this API — a proxy error page, most likely.
  if (!isRecord(payload) || !('data' in payload)) {
    throw new ApiClientError({
      code: 'INTERNAL',
      status: response.status,
      message: 'response body is missing the data envelope',
    })
  }

  // The envelope's `data` is unvalidated: the API contract is the guarantee,
  // and re-checking every field here would be a second schema to keep in sync.
  return payload as unknown as ApiResponse<T> & { meta?: unknown }
}

/** Performs a request and returns the unwrapped `data`. The common case. */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const envelope = await requestEnvelope<T>(path, options)
  return envelope.data
}

/**
 * Like `request`, but keeps `meta` for paginated endpoints.
 *
 * Note that not every list endpoint is paginated: `GET /categories` returns a
 * bare array with no `meta`, and must be read with `request`, not this.
 */
export async function requestPaginated<T>(
  path: string,
  options: RequestOptions = {},
): Promise<PaginatedResponse<T>> {
  const envelope = await requestEnvelope<T[]>(path, options)
  if (!isPaginationMeta(envelope.meta)) {
    throw new ApiClientError({
      code: 'INTERNAL',
      status: 200,
      message: 'paginated response is missing meta',
    })
  }
  return { data: envelope.data, meta: envelope.meta }
}

/** Reads the body as JSON, returning null rather than throwing on a non-JSON body. */
async function readJson(response: Response): Promise<unknown> {
  try {
    return await response.json()
  } catch {
    // An error page from a proxy, or an empty body on a status that should
    // have had one. Callers treat a null payload as "no usable envelope".
    return null
  }
}

/** Pulls the `error` object out of a failure envelope, if the body has one. */
function extractError(payload: unknown): ApiError | null {
  if (!isRecord(payload)) return null
  const { error } = payload as Partial<ApiErrorResponse>
  if (!isRecord(error) || typeof error.code !== 'string') return null
  return error as unknown as ApiError
}

/**
 * `Retry-After` in seconds. The spec also allows an HTTP date; this API sends
 * seconds, and a date would parse as NaN and be dropped rather than shown.
 *
 * Expect `undefined` in development even on a 429 that carries the header. The
 * backend's CORS `Access-Control-Expose-Headers` lists only `X-Request-ID`, and
 * a cross-origin `fetch` cannot read a header that is not exposed. In
 * production the admin is served from the site's own domain, so the request is
 * same-origin, nothing is hidden, and the exact wait time appears. Callers must
 * therefore treat the seconds as optional rather than assume a 429 has them.
 */
function parseRetryAfter(header: string | null): number | undefined {
  if (!header) return undefined
  const seconds = Number(header)
  return Number.isFinite(seconds) && seconds >= 0 ? Math.ceil(seconds) : undefined
}

function isPaginationMeta(value: unknown): value is PaginatedResponse<never>['meta'] {
  return (
    isRecord(value) &&
    typeof value.page === 'number' &&
    typeof value.page_size === 'number' &&
    typeof value.total === 'number'
  )
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}
