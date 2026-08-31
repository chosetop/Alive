<script setup lang="ts">
import type { SayingListItem } from '~/types'

defineProps<{
  items: SayingListItem[]
}>()
</script>

<template>
  <div class="focus">
    <article v-for="item in items" :key="item.short_id" class="stage">
      <SayingItem :item="item" compact />
      <SayingActions :short-id="item.short_id" />
    </article>
  </div>
</template>

<style scoped>
.focus {
  display: grid;
  gap: var(--space-9);
}

.stage {
  display: grid;
  gap: var(--space-5);
  justify-items: center;
  padding: var(--space-8) var(--space-6);
  border-radius: 1.25rem;
  background: color-mix(in srgb, var(--c-surface) 88%, var(--c-accent));
  text-align: center;
}

.stage :deep(.body) {
  max-width: 42rem;
  font-size: clamp(1.5rem, 3vw, 2.4rem);
  line-height: 1.9;
}
</style>
