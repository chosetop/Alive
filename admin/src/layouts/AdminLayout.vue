<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()

const isLoggingOut = ref(false)

/**
 * Navigation. `available: false` items are placeholders for the stages that
 * build them — shown, because the shape of the tool is useful information, but
 * not clickable, because a link to a page that does not exist is worse than no
 * link.
 */
const navItems = [
  { label: 'Dashboard', to: { name: 'dashboard' }, available: true },
  { label: 'Entries', to: { name: 'entries' }, available: true },
  { label: 'Categories', to: { name: 'categories' }, available: true },
] as const

async function handleLogout(): Promise<void> {
  if (isLoggingOut.value) return
  isLoggingOut.value = true
  try {
    // The store clears local state even if the request fails, so this always
    // ends signed out locally.
    await auth.logout()
    await router.replace({ name: 'login' })
  } finally {
    isLoggingOut.value = false
  }
}
</script>

<template>
  <div class="shell">
    <header class="header">
      <span class="wordmark">Alive Admin</span>
      <div class="user">
        <span class="user-name">{{ auth.displayName }}</span>
        <button class="logout" type="button" :disabled="isLoggingOut" @click="handleLogout">
          {{ isLoggingOut ? '退出中…' : '退出' }}
        </button>
      </div>
    </header>

    <div class="body">
      <!-- nav, not a bare div: this is the page's primary navigation, and the
           landmark is how a screen reader user skips straight to it. -->
      <nav class="sidebar" aria-label="主导航">
        <ul class="nav-list">
          <li v-for="item in navItems" :key="item.label">
            <RouterLink v-if="item.available && item.to" class="nav-link" :to="item.to">
              {{ item.label }}
            </RouterLink>
            <!-- aria-disabled rather than omitting it: the item is announced,
                 and announced as unavailable. -->
            <span v-else class="nav-link nav-link--disabled" aria-disabled="true">
              {{ item.label }}
            </span>
          </li>
        </ul>
      </nav>

      <main class="content">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: var(--header-height);
  flex-shrink: 0;
  padding: 0 var(--space-5);
  border-bottom: 1px solid var(--c-line);
  background: var(--c-surface);
}

.wordmark {
  font-size: 0.9375rem;
  font-weight: 600;
  letter-spacing: -0.01em;
}

.user {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

.user-name {
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.logout {
  padding: 0.25rem 0.5rem;
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
  cursor: pointer;
  transition:
    color 0.12s ease,
    border-color 0.12s ease;
}

.logout:hover:not(:disabled) {
  border-color: var(--c-danger);
  color: var(--c-danger);
}

.logout:disabled {
  color: var(--c-ink-faint);
  cursor: not-allowed;
}

.body {
  display: flex;
  flex: 1;
  min-height: 0;
}

.sidebar {
  width: var(--sidebar-width);
  flex-shrink: 0;
  padding: var(--space-5) var(--space-3);
  border-right: 1px solid var(--c-line);
}

.nav-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  margin: 0;
  padding: 0;
  list-style: none;
}

.nav-link {
  display: block;
  padding: 0.375rem var(--space-3);
  border-radius: var(--radius-sm);
  color: var(--c-ink-muted);
  font-size: 0.875rem;
  transition:
    background-color 0.12s ease,
    color 0.12s ease;
}

a.nav-link:hover {
  background: var(--c-surface-sunken);
  color: var(--c-ink);
  text-decoration: none;
}

/* router-link-active is added by vue-router to the link matching the current
   route. The weight shift carries the state; colour alone would not survive
   being viewed in greyscale. */
a.nav-link.router-link-active {
  background: var(--c-surface-sunken);
  color: var(--c-ink);
  font-weight: 500;
}

.nav-link--disabled {
  color: var(--c-ink-faint);
  cursor: default;
}

.content {
  flex: 1;
  min-width: 0;
  padding: var(--space-8) var(--space-6);
}

@media (max-width: 40rem) {
  /* Below this width the sidebar and content cannot share a row without both
     becoming cramped, so they stack. */
  .body {
    flex-direction: column;
  }

  .sidebar {
    width: 100%;
    padding: var(--space-3);
    border-right: none;
    border-bottom: 1px solid var(--c-line);
  }

  .nav-list {
    flex-direction: row;
    flex-wrap: wrap;
  }

  .content {
    padding: var(--space-5) var(--space-4);
  }
}
</style>
