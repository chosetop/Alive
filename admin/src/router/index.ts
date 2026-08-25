import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import AdminLayout from '../layouts/AdminLayout.vue'
import { useAuthStore } from '../stores/auth'

/**
 * `requiresAuth` marks routes that need a live session. `guestOnly` marks the
 * ones a signed-in person should not see, so /login does not sit there behind
 * an account that is already valid.
 */
declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    guestOnly?: boolean
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/Login.vue'),
    meta: { guestOnly: true },
  },
  {
    // The authenticated area shares one layout, so the chrome is mounted once
    // and page navigation swaps only the content.
    path: '/',
    component: AdminLayout,
    meta: { requiresAuth: true },
    children: [
      { path: '', name: 'home', redirect: { name: 'dashboard' } },
      {
        path: 'dashboard',
        name: 'dashboard',
        component: () => import('../views/Dashboard.vue'),
      },
      {
        path: 'entries',
        name: 'entries',
        component: () => import('../views/Entries.vue'),
      },
      {
        path: 'entries/new',
        name: 'entry-new',
        component: () => import('../views/EntryEditor.vue'),
      },
      {
        // `props: true` hands `id` to the component as a prop, so the editor
        // does not reach into the route to find its own subject.
        path: 'entries/:id',
        name: 'entry-edit',
        component: () => import('../views/EntryEditor.vue'),
        props: true,
      },
      {
        path: 'categories',
        name: 'categories',
        component: () => import('../views/Categories.vue'),
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('../views/NotFound.vue'),
  },
]

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  // Await initialization rather than reading isInitializing and hoping.
  // `initialize` deduplicates, so on every navigation after the first this is
  // an already-resolved promise. Without this, a page load lands here before
  // /me has answered, sees isAuthenticated === false, and redirects a
  // signed-in person to /login.
  await auth.initialize()

  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    // Remember where they were headed so login can return them there instead
    // of always dropping them on the dashboard.
    return { name: 'login', query: to.fullPath === '/' ? {} : { redirect: to.fullPath } }
  }

  if (to.meta.guestOnly && auth.isAuthenticated) {
    return { name: 'dashboard' }
  }

  return true
})

export default router
