import type { ApiErrorCode } from '../types/api'

/**
 * Local pseudo-code for a request that never produced a response body: the
 * network was down, the request was aborted, or the body was not JSON. The
 * server never sends this. Callers branch on it exactly like a real code.
 */
export const NETWORK_ERROR = 'NETWORK' as const

export type ClientErrorCode = ApiErrorCode | typeof NETWORK_ERROR

/**
 * Every failure out of the API client is one of these, so callers have a single
 * shape to catch and never have to inspect a raw Response.
 */
export class ApiClientError extends Error {
  readonly code: ClientErrorCode
  /** HTTP status, or 0 when no response arrived. */
  readonly status: number
  /** Per-field validation messages, keyed by field name. */
  readonly fields: Record<string, string> | undefined
  /** Quote this when reporting a problem; it locates the server log line. */
  readonly requestId: string | undefined
  /** Seconds from the `Retry-After` header, on 429 only. */
  readonly retryAfter: number | undefined

  constructor(init: {
    code: ClientErrorCode
    status: number
    /** The server's `message`. Kept for logs, not for display. */
    message: string
    fields?: Record<string, string> | undefined
    requestId?: string | undefined
    retryAfter?: number | undefined
  }) {
    super(init.message)
    this.name = 'ApiClientError'
    this.code = init.code
    this.status = init.status
    this.fields = init.fields
    this.requestId = init.requestId
    this.retryAfter = init.retryAfter
  }
}

/** Narrowing helper, so callers do not import the class just to type-check a catch. */
export function isApiClientError(value: unknown): value is ApiClientError {
  return value instanceof ApiClientError
}

/**
 * Turns an error into something worth showing a person.
 *
 * The server's own `message` is deliberately not used. It is written for
 * whoever reads the logs, and on a 500 it is the last place a driver-level
 * string could leak into the UI.
 */
export function toUserMessage(error: unknown): string {
  if (!isApiClientError(error)) {
    return '出现了意外错误，请稍后重试。'
  }

  switch (error.code) {
    case NETWORK_ERROR:
      return '无法连接到服务器，请检查网络后重试。'
    case 'INVALID_CREDENTIALS':
      return '用户名或密码错误'
    case 'UNAUTHORIZED':
      return '登录状态已失效，请重新登录。'
    case 'FORBIDDEN':
      return '没有权限执行此操作'
    case 'NOT_FOUND':
      return '请求的资源不存在'
    case 'RATE_LIMITED':
      return error.retryAfter === undefined
        ? '尝试次数过多，请稍后重试。'
        : `尝试次数过多，请在 ${error.retryAfter} 秒后重试。`
    case 'CONFLICT':
      return fieldMessages(error.fields) ?? '与现有数据冲突，请修改后重试。'
    case 'INVALID_INPUT':
      // 405 also carries INVALID_INPUT, and that is a bug in the caller
      // rather than anything the person at the keyboard typed wrong.
      if (error.status === 405) {
        return '请求方式不受支持。'
      }
      return fieldMessages(error.fields) ?? '提交的内容不合法，请检查后重试。'
    case 'INTERNAL':
    case 'UNAVAILABLE':
      return '服务暂时不可用，请稍后重试。'
  }
}

/** Joins per-field validation messages, e.g. `slug: 已被使用`. */
function fieldMessages(fields: Record<string, string> | undefined): string | null {
  if (!fields) return null
  const parts = Object.entries(fields).map(([field, message]) => `${field}: ${message}`)
  return parts.length > 0 ? parts.join('；') : null
}
