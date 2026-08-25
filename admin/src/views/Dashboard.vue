<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()

/** Local time of day, so the greeting matches the person's own clock. */
const greeting = computed(() => {
  const hour = new Date().getHours()
  if (hour < 5) return 'Still up.'
  if (hour < 12) return 'Good morning.'
  if (hour < 18) return 'Good afternoon.'
  return 'Good evening.'
})
</script>

<template>
  <div class="page">
    <header class="intro">
      <h1 class="greeting">{{ greeting }}</h1>
      <p class="welcome">欢迎回来，{{ auth.displayName }}。</p>
    </header>

    <!--
      No counts, no charts. The backend has no dashboard statistics endpoint, and
      numbers invented here would be indistinguishable from real ones — which is
      exactly what makes them worse than an empty space.
    -->
    <section class="section">
      <h2 class="section-title">开始</h2>
      <!-- Links rather than a fabricated "recent activity" list: the backend has
           no such endpoint, and the two real screens are one click away. -->
      <p class="links">
        <RouterLink :to="{ name: 'entries' }">内容</RouterLink>
        <RouterLink :to="{ name: 'categories' }">分类</RouterLink>
      </p>
      <p class="placeholder">编辑器将在下一步接入，目前内容列表是只读的。</p>
    </section>
  </div>
</template>

<style scoped>
.page {
  /* A comfortable measure for reading. Full-width text on a wide monitor is
     hard to track from line to line. */
  max-width: 42rem;
}

.intro {
  margin-bottom: var(--space-8);
}

.greeting {
  margin-bottom: var(--space-2);
  font-size: 1.375rem;
}

.welcome {
  color: var(--c-ink-muted);
  font-size: 0.9375rem;
}

.section-title {
  margin-bottom: var(--space-3);
  color: var(--c-ink-muted);
  font-size: 0.75rem;
  font-weight: 500;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.links {
  display: flex;
  gap: var(--space-4);
  margin-bottom: var(--space-4);
  font-size: 0.9375rem;
}

.placeholder {
  padding: var(--space-5);
  border: 1px dashed var(--c-line-strong);
  border-radius: var(--radius-md);
  color: var(--c-ink-faint);
  font-size: 0.875rem;
}
</style>
