import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { resolveAdminWorld } from '../content-worlds/registry'
import AdminLayout from '../layouts/AdminLayout.vue'
import WritingLayout from '../layouts/WritingLayout.vue'
import { useAuthStore } from '../stores/auth'

/**
 * `requiresAuth` marks routes that need a live session. `guestOnly` marks the
 * ones a signed-in person should not see, so /login does not sit there behind
 * an account that is already valid.
 *
 * `writingWorkspace` marks the routes served by the immersive shell. It is
 * declared as metadata rather than inferred from the path because the two shells
 * share the `/entries` prefix, and code outside the router -- anything that needs
 * to know whether the utility chrome is on screen -- should read the decision
 * rather than re-derive it from a string.
 */
declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    guestOnly?: boolean
    writingWorkspace?: boolean
  }
}

export const routes: RouteRecordRaw[] = [
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
        path: 'categories',
        name: 'categories',
        component: () => import('../views/Categories.vue'),
      },
      {
        path: 'worlds',
        name: 'worlds',
        component: () => import('../views/Worlds.vue'),
      },
    ],
  },
  {
    /**
     * The writing shell, a sibling of the utility shell rather than a child.
     * Nesting would mount the nav rail and the canvas together, which is the
     * layout this workspace exists to replace.
     *
     * **This record must stay below the utility record.** A parent path gets a
     * matcher of its own even with no empty-path child, so bare `/entries` is
     * claimed by both records at an identical score, and Vue Router breaks that
     * tie by declaration order. Verified, not assumed: listing this one first
     * makes `/entries` resolve to this unnamed parent and render the writing
     * shell around an empty canvas instead of the article library. `routes.test.ts`
     * pins the order for that reason.
     *
     * The deeper paths need no such care -- `/entries/new` and `/entries/41` are
     * unambiguous, since the utility record offers nothing at that depth.
     *
     * `requiresAuth` is repeated because it is per-record metadata, not
     * inherited state -- but the guard reading it is still the single one in this
     * file. Neither layout checks a session itself.
     */
    path: '/entries',
    component: WritingLayout,
    meta: { requiresAuth: true, writingWorkspace: true },
    children: [
      {
        path: 'new',
        name: 'entry-new',
        component: () => import('../views/NewEntry.vue'),
      },
      {
        path: 'new/saying',
        name: 'saying-editor-new',
        component: () => import('../views/SayingEditor.vue'),
      },
      {
        path: 'new/video',
        name: 'video-editor-new',
        component: () => import('../views/EntryEditor.vue'),
        props: { world: 'video' },
      },
      {
        path: 'new/:world',
        name: 'entry-new-world',
        component: () => import('../views/EntryEditor.vue'),
        props: (route) => ({ world: String(route.params.world) }),
        beforeEnter: (to) => {
          const world = resolveAdminWorld(String(to.params.world))
          if (world === null || world.editorRouteName === null) return { name: 'not-found' }
          return true
        },
      },
      {
        path: 'blank',
        name: 'entry-blank',
        component: () => import('../views/EntryEditor.vue'),
        props: { blank: true },
      },
      {
        // `props: true` hands `id` to the component as a prop, so the editor
        // does not reach into the route to find its own subject.
        path: ':id',
        name: 'entry-edit',
        component: () => import('../views/EntryEditor.vue'),
        props: true,
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
