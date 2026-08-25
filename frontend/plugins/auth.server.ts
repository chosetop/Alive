import { useAuthStore } from '~/stores/auth'

/**
 * Resolves the session once, during SSR, before the first render.
 *
 * Server-side rather than universal for two reasons: the request already carries
 * the session cookie (forwarded by `useApi`), and resolving it here means the
 * markup sent to the browser is already correct. Doing it on the client instead
 * would render every visitor as signed out and then flip after hydration.
 *
 * Pinia state is serialised into the payload, so the client picks this up
 * without a second request.
 *
 * A signed-out visitor is the normal case: `fetchMe` treats a 401 as "no user".
 * A genuine backend failure is swallowed here as well — the public site reads
 * fine without a session, so a dead API should not turn every page into an
 * error page. It stays visible in the server log.
 */
export default defineNuxtPlugin(async () => {
  const auth = useAuthStore()
  if (auth.initialized) return

  try {
    await auth.fetchMe()
  } catch (cause) {
    console.warn('[auth] session check failed:', cause instanceof Error ? cause.message : cause)
  }
})
