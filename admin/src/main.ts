import { createPinia } from 'pinia'
import { createApp } from 'vue'
import { setUnauthorizedHandler } from './api'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth'
import './style.css'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)

// Pinia must be installed before a store is used, and the store must exist
// before the router is installed, because the guard resolves it on the first
// navigation.
const auth = useAuthStore(pinia)

/**
 * A 401 on any request other than login or /me means the session ended while
 * the app was open — it expired, or it was revoked elsewhere. Drop local state
 * and send them to login.
 *
 * Registering the handler here rather than inside the client keeps the client
 * unaware of Pinia and the router, so there is no import cycle between them.
 */
setUnauthorizedHandler(() => {
  auth.clearSession()
  const current = router.currentRoute.value
  if (current.name !== 'login') {
    void router.replace({ name: 'login', query: { redirect: current.fullPath } })
  }
})

app.use(router)

/**
 * Resolve the session before the first render.
 *
 * The guard awaits the same initialization, so this is not strictly required
 * for correct routing. It is here to avoid a visible flash: mounting first
 * would paint the app shell for one frame before /me answers.
 *
 * `initialize` never rejects, so a mount always happens; an unreachable backend
 * surfaces as an unauthenticated app with an explanation on the login page.
 */
void auth.initialize().then(() => {
  app.mount('#app')
})
