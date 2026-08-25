import { useAuthStore } from '~/stores/auth'

/**
 * Fallback session check for renders that never touched the server plugin:
 * a statically generated page, or an SPA fallback response.
 *
 * In the normal SSR flow this is a no-op. `initialized` is part of the
 * serialised Pinia payload, so it is already true by the time the client boots
 * and no second request is made.
 */
export default defineNuxtPlugin(async () => {
  const auth = useAuthStore()
  if (auth.initialized) return

  try {
    await auth.fetchMe()
  } catch {
    // Public pages render fine without a session; nothing to do here.
  }
})
