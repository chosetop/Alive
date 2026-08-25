/**
 * Authentication. Source of truth: docs/api.md section 3.
 *
 * The session lives in an HttpOnly cookie, not in a Bearer token. There is no
 * token anywhere in a response body, so there is nothing for the client to
 * store — only the current user is worth keeping in memory.
 */

/** Shape of `data` from both `POST /auth/login` and `GET /me`. */
export type User = {
  id: number
  username: string
  role: string
  display_name: string
}

/**
 * Body for `POST /auth/login`. Both fields required.
 *
 * The server deliberately applies no length validation: rejecting a too-short
 * password as malformed would leak how long the real one is. A wrong password
 * and an unknown username produce byte-identical responses.
 */
export type LoginBody = {
  username: string
  password: string
}
