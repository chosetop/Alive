<script setup lang="ts">
/**
 * Unmatched routes. Outside AdminLayout, so it renders for a signed-out visitor
 * too without pulling in chrome that assumes an account.
 */
</script>

<template>
  <main class="page">
    <h1 class="title">404</h1>
    <p class="message">请求的页面不存在。</p>
    <RouterLink class="link" to="/dashboard">回到 Dashboard</RouterLink>
  </main>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-3);
  min-height: 100vh;
  padding: var(--space-6);
}

.title {
  font-size: 1.75rem;
}

.message {
  color: var(--c-ink-muted);
  font-size: 0.9375rem;
}

.link {
  font-size: 0.875rem;
}
</style>
