<script setup lang="ts">
import type { SayingListItem } from '~/types'

defineProps<{
  items: SayingListItem[]
}>()
</script>

<template>
  <div class="wall">
    <article v-for="item in items" :key="item.short_id" class="card">
      <SayingItem :item="item" compact />
      <SayingActions :short-id="item.short_id" />
    </article>
  </div>
</template>

<style scoped>
.wall {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
  gap: var(--space-4);
}

.card {
  display: grid;
  gap: var(--space-4);
  padding: var(--space-5);
  border: 1px solid var(--c-line);
  border-radius: 1rem;
  background: color-mix(in srgb, var(--c-surface) 92%, var(--c-accent));
}
</style>
