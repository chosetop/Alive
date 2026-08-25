import type { LoginRequest, User } from '../types/api'
import { request } from './client'

/**
 * POST /api/v1/auth/login
 *
 * On success the server sets the session cookie and returns the account. There
 * is no token in the body to store.
 *
 * `skipUnauthorizedHandler` matters here: a 401 from this endpoint is
 * `INVALID_CREDENTIALS`, meaning the password was wrong. Running the
 * session-expired path on it would clear state and redirect to the page the
 * person is already on, hiding the error they need to see.
 */
export function login(credentials: LoginRequest): Promise<User> {
  return request<User>('/auth/login', {
    method: 'POST',
    body: credentials,
    skipUnauthorizedHandler: true,
  })
}

/**
 * POST /api/v1/auth/logout
 *
 * Idempotent and always 204. It needs no valid session: revoking a credential
 * that is already dead is not an error, so this never has to be guarded.
 */
export function logout(): Promise<void> {
  return request<void>('/auth/logout', { method: 'POST' })
}

/**
 * GET /api/v1/me
 *
 * The authoritative answer to "is this browser logged in". Returns 401
 * `UNAUTHORIZED` when the session is missing or expired.
 *
 * The global unauthorized handler is skipped because the caller is the auth
 * store, which interprets the 401 itself as "not logged in". Letting the
 * handler run too would mean two things reacting to one 401, and at startup it
 * would redirect before the router has decided anything.
 */
export function fetchMe(): Promise<User> {
  return request<User>('/me', { skipUnauthorizedHandler: true })
}
