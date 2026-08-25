import { defineStore } from 'pinia'
import type { LoginBody, User } from '~/types'
import { ApiError } from '~/utils/api-error'

/**
 * Current session state.
 *
 * Deliberately holds no credential. The session is an HttpOnly cookie, so the
 * token is not readable from JavaScript and never appears in a response body —
 * there is nothing to copy into a store. The only thing worth keeping is who is
 * signed in; the browser handles the rest.
 */
export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  /** True while a login / logout / session check is in flight. */
  const loading = ref(false)
  /** True once the session has been checked, whatever the outcome. */
  const initialized = ref(false)

  const isAuthenticated = computed(() => user.value !== null)

  /**
   * Reads the current session.
   *
   * A 401 `UNAUTHORIZED` is the expected answer for a signed-out visitor, not a
   * failure: it is swallowed and recorded as "no user". Anything else (backend
   * down, 500) rethrows, because that is a real problem and silently rendering
   * a signed-out page would hide it.
   */
  async function fetchMe(): Promise<User | null> {
    const api = useApi()
    loading.value = true
    try {
      user.value = await api.get<User>('/me')
      return user.value
    } catch (cause) {
      if (cause instanceof ApiError && cause.isUnauthorized) {
        user.value = null
        return null
      }
      throw cause
    } finally {
      loading.value = false
      initialized.value = true
    }
  }

  /**
   * Signs in. The response carries the user; the session arrives as a
   * `Set-Cookie` header the browser stores itself.
   *
   * Rejects with `ApiError` on failure. Callers should branch on `code`
   * (`INVALID_CREDENTIALS`, `INVALID_INPUT`, `RATE_LIMITED`) rather than on
   * `message`, which is free to change.
   */
  async function login(body: LoginBody): Promise<User> {
    const api = useApi()
    loading.value = true
    try {
      user.value = await api.post<User>('/auth/login', body)
      return user.value
    } finally {
      loading.value = false
      initialized.value = true
    }
  }

  /**
   * Signs out. Idempotent server-side and always answers 204, so no valid
   * session is required to call it.
   *
   * Local state is cleared even if the request fails: the intent is to end the
   * session, and leaving a stale user on screen after a failed logout is worse
   * than a clean sign-out with a dead cookie.
   */
  async function logout(): Promise<void> {
    const api = useApi()
    loading.value = true
    try {
      await api.requestNoContent('POST', '/auth/logout')
    } finally {
      user.value = null
      loading.value = false
      initialized.value = true
    }
  }

  return {
    user,
    loading,
    initialized,
    isAuthenticated,
    fetchMe,
    login,
    logout,
  }
})
