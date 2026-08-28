<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { dashboardApi, toUserMessage } from '../api'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const metrics = ref<Awaited<ReturnType<typeof dashboardApi.getDashboardMetrics>> | null>(null)
const isLoading = ref(true)
const loadError = ref<string | null>(null)

async function loadMetrics(): Promise<void> {
  isLoading.value = true
  loadError.value = null
  try {
    metrics.value = await dashboardApi.getDashboardMetrics()
  } catch (error) {
    loadError.value = toUserMessage(error)
  } finally {
    isLoading.value = false
  }
}

onMounted(loadMetrics)

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

    <section class="section">
      <div class="section-heading">
        <div>
          <p class="eyebrow">YOUR STORY, IN NUMBERS</p>
          <h2 class="section-title">创作概览</h2>
        </div>
        <RouterLink class="section-link" :to="{ name: 'entries' }">查看全部内容 →</RouterLink>
      </div>

      <p v-if="isLoading" class="state" aria-live="polite">正在同步创作数据…</p>
      <p v-else-if="loadError" class="state state--error" role="alert">{{ loadError }}</p>
      <div v-else-if="metrics" class="metrics-grid" aria-label="创作统计">
        <article class="metric-card metric-card--accent">
          <span class="metric-icon" aria-hidden="true">✦</span>
          <p class="metric-label">已写字数</p>
          <p class="metric-value">{{ metrics.total_words.toLocaleString('zh-CN') }}</p>
          <p class="metric-note">累计记录的文字</p>
        </article>
        <article class="metric-card">
          <span class="metric-icon" aria-hidden="true">◌</span>
          <p class="metric-label">总文章数</p>
          <p class="metric-value">{{ metrics.total_entries.toLocaleString('zh-CN') }}</p>
          <p class="metric-note">包含草稿与已发布</p>
        </article>
        <article class="metric-card">
          <span class="metric-icon" aria-hidden="true">↗</span>
          <p class="metric-label">已发布</p>
          <p class="metric-value">{{ metrics.published_entries.toLocaleString('zh-CN') }}</p>
          <p class="metric-note">正在被世界看见</p>
        </article>
      </div>
    </section>
  </div>
</template>

<style scoped>
.page {
  max-width: 58rem;
}

.intro {
  margin-bottom: 2.75rem;
}

.greeting {
  margin-bottom: var(--space-2);
  font-size: clamp(1.75rem, 3vw, 2.5rem);
  letter-spacing: -0.04em;
}

.welcome {
  color: var(--c-ink-muted);
  font-size: 0.9375rem;
}

.section {
  padding: clamp(1.25rem, 3vw, 2rem);
  border: 1px solid color-mix(in srgb, var(--c-line) 72%, transparent);
  border-radius: 1.5rem;
  background: color-mix(in srgb, var(--c-surface) 84%, transparent);
  box-shadow: 0 1.25rem 3rem color-mix(in srgb, var(--c-ink) 7%, transparent);
}

.section-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--space-4);
  margin-bottom: 1.25rem;
}

.eyebrow {
  margin-bottom: 0.375rem;
  color: var(--c-accent);
  font-size: 0.6875rem;
  font-weight: 700;
  letter-spacing: 0.13em;
}

.section-title {
  font-size: 1.25rem;
  letter-spacing: -0.025em;
}

.section-link {
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem;
}

.metric-card {
  min-height: 10.5rem;
  padding: 1.125rem;
  border: 1px solid color-mix(in srgb, var(--c-line) 65%, transparent);
  border-radius: 1.125rem;
  background: color-mix(in srgb, var(--c-paper) 58%, transparent);
}

.metric-card--accent {
  border-color: color-mix(in srgb, var(--c-accent) 32%, var(--c-line));
  background: color-mix(in srgb, var(--c-accent) 10%, var(--c-paper));
}

.metric-icon {
  display: inline-grid;
  width: 2rem;
  height: 2rem;
  place-items: center;
  margin-bottom: 1rem;
  border-radius: 0.75rem;
  background: color-mix(in srgb, var(--c-accent) 13%, transparent);
  color: var(--c-accent);
  font-size: 1.125rem;
}

.metric-label,
.metric-note {
  color: var(--c-ink-muted);
  font-size: 0.75rem;
}

.metric-value {
  margin: 0.2rem 0 0.1rem;
  font-size: clamp(1.75rem, 4vw, 2.375rem);
  font-variant-numeric: tabular-nums;
  font-weight: 650;
  letter-spacing: -0.055em;
}

.state {
  padding: 1rem 0;
  color: var(--c-ink-muted);
  font-size: 0.875rem;
}

.state--error {
  color: var(--c-danger);
}

@media (max-width: 640px) {
  .section-heading {
    align-items: flex-start;
    flex-direction: column;
  }

  .metrics-grid {
    grid-template-columns: 1fr;
  }

  .metric-card {
    min-height: auto;
  }
}
</style>
