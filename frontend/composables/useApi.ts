import type { ApiPage } from '~/types'
import { toApiError } from '~/utils/api-error'

/** Backend API version prefix. The health probes deliberately sit outside it. */
const API_PREFIX = '/api/v1'

type QueryValue = string | number | boolean | undefined | null
export type ApiQuery = Record<string, QueryValue>

type RequestOptions = {
  query?: ApiQuery
  body?: unknown
}

/**
 * Drops `undefined` and `null` entries so optional query parameters can be
 * passed straight through without building the object conditionally.
 *
 * Empty strings are kept: `?category=` is meaningful to the backend (it reads
 * as "no filter", which is what a "show all" selector sends).
 */
function cleanQuery(query: ApiQuery | undefined): Record<string, string> | undefined {
  if (!query) return undefined

  const entries = Object.entries(query).filter(
    (entry): entry is [string, string | number | boolean] =>
      entry[1] !== undefined && entry[1] !== null,
  )
  if (entries.length === 0) return undefined

  return Object.fromEntries(entries.map(([key, value]) => [key, String(value)]))
}

/**
 * The single place HTTP requests to the backend are made.
 *
 * Two things it exists to guarantee:
 *
 * 1. `credentials: 'include'`. The session is an HttpOnly cookie, so a request
 *    without this sends no credentials and every authenticated call 401s.
 * 2. On the server, the browser's cookie header is forwarded explicitly.
 *    `credentials` has no meaning during SSR — there is no cookie jar — so a
 *    signed-in visitor would otherwise render as signed out and then flip once
 *    the client hydrated.
 *
 * Failures reject with `ApiError`, which carries `code`, `status`, `fields` and
 * `request_id`. Successes unwrap the envelope, so callers get `T` rather than
 * `{ data: T }` — except for paginated endpoints, where `meta` is needed and the
 * envelope is returned whole.
 */
export function useApi() {
  const config = useRuntimeConfig()
  const baseURL = `${config.public.apiBase}${API_PREFIX}`

  // Only forwards on the server; returns {} in the browser, where the cookie is
  // attached by credentials: 'include'.
  const requestHeaders = import.meta.server ? useRequestHeaders(['cookie']) : undefined

  async function request<T>(
    method: 'GET' | 'POST' | 'PATCH' | 'DELETE',
    path: string,
    options: RequestOptions = {},
  ): Promise<T> {
    try {
      return (await $fetch<T>(path, {
        baseURL,
        method,
        credentials: 'include',
        headers: requestHeaders,
        query: cleanQuery(options.query),
        body: options.body as Record<string, unknown> | undefined,
      })) as T
    } catch (cause) {
      throw toApiError(cause)
    }
  }

  return {
    /** Unwrapped GET: returns the contents of `data`. */
    async get<T>(path: string, query?: ApiQuery): Promise<T> {
      const response = await request<{ data: T }>('GET', path, { query })
      return response.data
    },

    /**
     * GET keeping the envelope, for paginated endpoints where `meta` matters.
     * Returns `{ data, meta }`.
     */
    getPage<T>(path: string, query?: ApiQuery): Promise<ApiPage<T>> {
      return request('GET', path, { query })
    },

    /** GET keeping the envelope, for unpaginated list endpoints (no `meta`). */
    getList<T>(path: string, query?: ApiQuery): Promise<{ data: T[] }> {
      return request('GET', path, { query })
    },

    async post<T>(path: string, body?: unknown): Promise<T> {
      const response = await request<{ data: T }>('POST', path, { body })
      return response.data
    },

    async patch<T>(path: string, body: unknown): Promise<T> {
      const response = await request<{ data: T }>('PATCH', path, { body })
      return response.data
    },

    /** For endpoints answering 204 with an empty body (logout, delete). */
    async requestNoContent(method: 'POST' | 'DELETE', path: string): Promise<void> {
      await request<unknown>(method, path)
    },
  }
}
