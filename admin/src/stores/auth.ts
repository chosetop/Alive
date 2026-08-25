import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { authApi, isApiClientError } from '../api'
import type { LoginRequest, User } from '../types/api'

/**
 * Authentication state, derived from the server and never from local storage.
 *
 * The session is an HttpOnly cookie, so this store cannot see it. That is the
 * point: `isAuthenticated` here means "the server answered /me with an
 * account", not "a flag we wrote is still set". A persisted `isLoggedIn=true`
 * would survive the session it describes and let the UI render a logged-in
 * shell whose every request then 401s.
 */
export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)

  /**
   * True from app start until the first /me has settled. The router guard waits
   * on this, because during that window "no user" does not yet mean "logged
   * out" — treating it as logged out is what bounces a signed-in person to
   * /login on every refresh.
   */
  const isInitializing = ref(true)

  /**
   * Set when initialization failed for a reason that is not "logged out" — the
   * backend being down, say. Distinct from "not authenticated" so the UI can
   * tell someone the server is unreachable rather than silently showing a login
   * form that cannot succeed.
   */
  const initializationError = ref<string | null>(null)

  const isAuthenticated = computed(() => user.value !== null)

  /**
   * The in-flight initialization, kept so that anything needing auth state can
   * await the same request instead of starting another. Not exposed as state:
   * it is a deduplication detail, and `isInitializing` is what the UI reads.
   */
  let initialization: Promise<void> | null = null

  /** What to call the person: their display name, or their username. */
  const displayName = computed(() => {
    if (!user.value) return ''
    // display_name is nullable in the database and serialises as "" when unset.
    return user.value.display_name.trim() || user.value.username
  })

  /**
   * Reads the current account from the server.
   *
   * Returns true when a session is live. A 401 is not an exception here: it is
   * the expected answer for a browser that is not logged in, so it resolves
   * false and clears the user. Anything else still throws, because "the server
   * is broken" must not be recorded as "you are logged out".
   */
  async function fetchMe(): Promise<boolean> {
    try {
      user.value = await authApi.fetchMe()
      return true
    } catch (error) {
      if (isApiClientError(error) && error.status === 401) {
        user.value = null
        return false
      }
      throw error
    }
  }

  /**
   * Restores state on a page load. Called once, before the router starts.
   *
   * Never rejects: a failure to reach the server leaves the app unauthenticated
   * with `initializationError` set, so the guard can still route and the login
   * page can explain why signing in is unlikely to work.
   */
  async function initialize(): Promise<void> {
    // Concurrent callers share one request. The router guard and app startup
    // both want initialization done, and two /me calls would race to set state.
    if (initialization) return initialization
    initialization = runInitialize()
    return initialization
  }

  async function runInitialize(): Promise<void> {
    isInitializing.value = true
    initializationError.value = null
    try {
      await fetchMe()
    } catch (error) {
      user.value = null
      initializationError.value = isApiClientError(error)
        ? error.message
        : 'failed to reach the server'
    } finally {
      // Set last and unconditionally: the guard is waiting on it, and leaving
      // it true on an error path would hang every navigation.
      isInitializing.value = false
    }
  }

  /**
   * Signs in. Throws `ApiClientError` on failure so the form can map the code
   * to a message; the store stays out of presentation.
   */
  async function login(credentials: LoginRequest): Promise<void> {
    // The response carries the account, so this needs no follow-up /me.
    user.value = await authApi.login(credentials)
    initializationError.value = null
  }

  /**
   * Signs out. Clears local state even if the request fails: the intent was to
   * end the session, and the endpoint is idempotent, so the worst case is a
   * server-side row that expires on its own while this browser has already
   * forgotten the account.
   */
  async function logout(): Promise<void> {
    try {
      await authApi.logout()
    } finally {
      clearSession()
    }
  }

  /**
   * Drops local state without calling the server. Used when the server has
   * already rejected the session, i.e. a 401 on some other request.
   */
  function clearSession(): void {
    user.value = null
  }

  return {
    user,
    isInitializing,
    initializationError,
    isAuthenticated,
    displayName,
    login,
    logout,
    fetchMe,
    initialize,
    clearSession,
  }
})
