<script setup lang="ts">
import { listCreatableAdminWorlds } from '../content-worlds/registry'

const worlds = listCreatableAdminWorlds()
</script>

<template>
  <div class="page">
    <header class="head">
      <h1 class="title">选择内容世界</h1>
      <p class="subtitle">先决定这条内容属于哪个世界，再进入对应的创作界面。</p>
    </header>

    <div class="grid">
      <RouterLink
        v-for="world in worlds"
        :key="world.key"
        class="card"
        :to="{ name: world.editorRouteName!, params: { world: world.key } }"
      >
        <span class="card__eyebrow">{{ world.label }}</span>
        <strong class="card__title">{{ world.createLabel }}</strong>
      </RouterLink>
    </div>
  </div>
</template>

<style scoped>
.page {
  width: min(100%, 42rem);
  margin: 0 auto;
  padding: var(--space-8) var(--space-5);
}

.head {
  margin-bottom: var(--space-6);
}

.title {
  margin-bottom: var(--space-2);
  font-family: var(--font-heading);
  font-size: clamp(1.75rem, 4vw, 2.4rem);
}

.subtitle {
  color: var(--c-ink-muted);
}

.grid {
  display: grid;
  gap: var(--space-4);
}

.card {
  display: grid;
  gap: var(--space-2);
  padding: var(--space-5);
  border: 1px solid var(--c-glass-border);
  border-radius: var(--radius-surface);
  background: var(--c-glass);
  box-shadow: var(--shadow-control);
}

.card:hover {
  text-decoration: none;
  border-color: var(--c-accent);
}

.card__eyebrow {
  color: var(--c-ink-faint);
  font-size: 0.75rem;
}

.card__title {
  color: var(--c-ink);
  font-size: 1.125rem;
}
</style>
