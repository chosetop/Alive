<script setup lang="ts">
import { computed } from 'vue'
import { UiIcon } from '../components/ui'
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
    <header class="page-header" data-page-header>
      <h1 class="greeting">{{ greeting }}</h1>
      <p class="welcome">欢迎回来，{{ auth.displayName }}。</p>
    </header>

    <!--
      No counts, no charts. The backend has no dashboard statistics endpoint, and
      numbers invented here would be indistinguishable from real ones — which is
      exactly what makes them worse than an empty space.
    -->
    <section class="section" aria-labelledby="dashboard-start-title">
      <h2 id="dashboard-start-title" class="section-title">开始</h2>
      <!-- Links rather than a fabricated "recent activity" list: the backend has
           no such endpoint, and the two real screens are one click away. -->
      <div class="action-grid">
        <RouterLink class="action-card" :to="{ name: 'entries' }">
          <span>内容</span><UiIcon class="action-card__icon" name="chevron-right" />
        </RouterLink>
        <RouterLink class="action-card" :to="{ name: 'categories' }">
          <span>分类</span><UiIcon class="action-card__icon" name="chevron-right" />
        </RouterLink>
      </div>
      <p class="placeholder">编辑器将在下一步接入，目前内容列表是只读的。</p>
    </section>
  </div>
</template>

<style scoped>
.page { max-width: 52rem; }
.page-header { margin-bottom: var(--space-7); }
.greeting { margin: 0 0 var(--space-2); font-family: var(--font-heading); font-size: clamp(1.5rem, 3vw, 2rem); font-weight: 600; letter-spacing: -0.02em; }
.welcome { margin: 0; color: var(--c-ink-muted); font-size: 0.875rem; }

.section-title {
  margin-bottom: var(--space-3);
  color: var(--c-ink-muted);
  font-size: 0.75rem;
  font-weight: 500;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.action-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-3); margin-bottom: var(--space-4); }
.action-card { display: flex; align-items: center; justify-content: space-between; min-height: 5.5rem; padding: var(--space-4); border: 1px solid var(--c-glass-border); border-radius: var(--radius-surface); background: var(--c-glass); box-shadow: var(--shadow-control); color: var(--c-ink); font-size: 0.9375rem; font-weight: 600; transition: background-color var(--motion-fast) ease, box-shadow var(--motion-fast) ease, color var(--motion-fast) ease; backdrop-filter: blur(18px) saturate(140%); }
.action-card:hover { background: var(--c-surface); box-shadow: var(--shadow-float); color: var(--c-accent); text-decoration: none; }
.action-card__icon { width: 1rem; height: 1rem; }

.placeholder {
  margin: 0;
  padding: var(--space-4);
  border-radius: var(--radius-surface);
  background: var(--c-surface-sunken);
  color: var(--c-ink-faint);
  font-size: 0.875rem;
}

@media (max-width: 32rem) { .action-grid { grid-template-columns: 1fr; } }
</style>
