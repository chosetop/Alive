<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { UiDialog, UiIcon, UiIconButton } from '../components/ui'
import ThemePicker from '../components/writing/ThemePicker.vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const isLoggingOut = ref(false)
const isMobileNavOpen = ref(false)

/**
 * A way into the writing shell from the utility shell.
 *
 * `entry-new` rather than a link to the article list: the list is already the page
 * this nav item sits beside, and the reason to leave it is to write. The blank
 * The editor stays client-first until the writer makes a meaningful change.
 */
const writeTarget = { name: 'entry-new' } as const

/**
 * Navigation. `available: false` items are placeholders for the stages that
 * build them — shown, because the shape of the tool is useful information, but
 * not clickable, because a link to a page that does not exist is worse than no
 * link.
 */
const navItems = [
  { label: '概览', to: { name: 'dashboard' }, available: true },
  { label: '内容', to: { name: 'entries' }, available: true },
  { label: '分类', to: { name: 'categories' }, available: true },
  { label: '标签', to: { name: 'tags' }, available: true },
  { label: '音乐', to: { name: 'music' }, available: true },
  { label: '世界', to: { name: 'worlds' }, available: true },
  // Leaves this shell entirely rather than swapping the content pane. That is the
  // point: the writing workspace is a sibling layout, not a page inside this one.
  { label: '写作', to: writeTarget, available: true },
] as const

/** The context bar reads the same route contract as the links; no path parsing. */
const currentContext = computed(
  () => navItems.find((item) => item.to.name === route.name)?.label ?? 'Alive',
)

function handleDrawerNavigate(
  navigate: (event?: MouseEvent) => unknown,
  event: MouseEvent,
): void {
  navigate(event)
  isMobileNavOpen.value = false
}

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
    <header class="topbar">
      <div class="brand-context">
        <span class="wordmark">Alive</span>
        <span class="context-separator" aria-hidden="true">/</span>
        <span class="page-context" data-page-context>{{ currentContext }}</span>
      </div>

      <div class="topbar-actions">
        <ThemePicker />
        <UiIconButton
          class="mobile-nav-trigger"
          label="打开主导航"
          :pressed="isMobileNavOpen"
          data-mobile-nav
          @click="isMobileNavOpen = true"
        >
          <UiIcon name="more-horizontal" />
        </UiIconButton>

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
            <RouterLink
              v-if="item.available && item.to"
              v-slot="{ href, navigate, isExactActive }"
              custom
              :to="item.to"
            >
              <a
                class="nav-link"
                :class="{ 'nav-link--active': isExactActive }"
                :href="href"
                :aria-current="isExactActive ? 'page' : undefined"
                :data-active="isExactActive ? 'true' : 'false'"
                @click="navigate"
              >
                <span v-if="isExactActive" class="alive-cursor" data-alive-cursor aria-hidden="true" />
                <span>{{ item.label }}</span>
              </a>
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

    <!-- At the drawer breakpoint the desktop nav is display:none, so the dialog
         is the only navigation landmark available to assistive technology. -->
    <UiDialog title="主导航" :open="isMobileNavOpen" @update:open="isMobileNavOpen = $event">
      <nav class="mobile-drawer" aria-label="主导航">
        <ul class="nav-list nav-list--drawer">
          <li v-for="item in navItems" :key="item.label">
            <RouterLink
              v-if="item.available && item.to"
              v-slot="{ href, navigate, isExactActive }"
              custom
              :to="item.to"
            >
              <a
                class="nav-link"
                :class="{ 'nav-link--active': isExactActive }"
                :href="href"
                :aria-current="isExactActive ? 'page' : undefined"
                :data-active="isExactActive ? 'true' : 'false'"
                @click="handleDrawerNavigate(navigate, $event)"
              >
                <span v-if="isExactActive" class="alive-cursor" data-alive-cursor aria-hidden="true" />
                <span>{{ item.label }}</span>
              </a>
            </RouterLink>
            <span v-else class="nav-link nav-link--disabled" aria-disabled="true">
              {{ item.label }}
            </span>
          </li>
        </ul>
      </nav>
    </UiDialog>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  background: var(--c-paper);
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: var(--header-height);
  padding: 0 var(--space-5);
  border-bottom: 1px solid var(--c-line);
  background: var(--c-surface);
}

.brand-context,
.topbar-actions {
  display: flex;
  align-items: center;
}

.brand-context {
  min-width: 0;
  gap: var(--space-2);
}

.wordmark {
  flex-shrink: 0;
  font-size: 0.9375rem;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.context-separator,
.user-name {
  color: var(--c-ink-faint);
}

.page-context {
  overflow: hidden;
  font-size: 0.875rem;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topbar-actions {
  flex-shrink: 0;
  gap: var(--space-3);
}

.mobile-nav-trigger {
  display: none;
}

.user-name {
  font-size: 0.8125rem;
}

.logout {
  padding: 0.25rem 0.5rem;
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-control);
  background: transparent;
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
  cursor: pointer;
  transition:
    color var(--motion-fast) ease,
    border-color var(--motion-fast) ease,
    background-color var(--motion-fast) ease;
}

.logout:hover:not(:disabled) {
  border-color: var(--c-danger);
  background: var(--c-danger-surface);
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
  background: var(--c-surface);
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
  position: relative;
  display: flex;
  align-items: center;
  min-height: 2.5rem;
  padding: 0.5rem var(--space-3);
  border-radius: var(--radius-control);
  color: var(--c-ink-muted);
  font-size: 0.875rem;
  transition:
    background-color var(--motion-fast) ease,
    color var(--motion-fast) ease;
}

a.nav-link:hover {
  background: var(--c-surface-sunken);
  color: var(--c-ink);
  text-decoration: none;
}

/* The cursor and weight shift survive greyscale, unlike colour-only selection. */
.nav-link--active {
  background: var(--c-surface-sunken);
  color: var(--c-ink);
  font-weight: 600;
}

.alive-cursor {
  position: absolute;
  left: var(--space-1);
  width: 0.1875rem;
  height: 1.25rem;
  border-radius: var(--radius-control);
  background: var(--c-alive);
}

.nav-link--active > span:last-child {
  margin-left: var(--space-2);
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

/* Utility pages own their readable max-width. Give that page frame the full
   content track, then centre it so a wide monitor adds balanced breathing room
   instead of one large empty field on the right. */
.content > :deep(.page) {
  width: 100%;
  margin-inline: auto;
}

.mobile-drawer {
  min-width: min(18rem, calc(100vw - var(--space-6)));
}

.nav-list--drawer {
  gap: var(--space-2);
}

@media (max-width: 40rem) {
  .topbar {
    padding: 0 var(--space-3);
  }

  .mobile-nav-trigger {
    display: inline-flex;
  }

  .user-name {
    display: none;
  }

  .sidebar {
    display: none;
  }

  .content {
    padding: var(--space-5) var(--space-4);
  }
}
</style>
