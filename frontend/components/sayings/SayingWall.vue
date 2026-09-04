<script setup lang="ts">
import type { SayingListItem } from '~/types'
import { ref } from 'vue'
import { useSayingMotion } from '~/composables/useSayingMotion'

defineProps<{
  items: SayingListItem[]
}>()
const root = ref<HTMLElement | null>(null)
useSayingMotion(root)
</script>

<template>
  <div ref="root" class="wall">
    <article v-for="item in items" :key="item.short_id" class="card">
      <SayingItem :item="item" compact />
      <SayingActions :item="item" />
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
  background: color-mix(in srgb, var(--c-surface) 72%, transparent);
  -webkit-backdrop-filter: blur(10px);
  backdrop-filter: blur(10px);
  box-shadow: 0 10px 28px color-mix(in srgb, var(--c-ink) 7%, transparent);
  transition: transform var(--duration-mid) var(--ease-out), box-shadow var(--duration-mid) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
}
.card:hover, .card:focus-within { transform: translateY(-3px); border-color: var(--c-line-strong); box-shadow: 0 14px 32px color-mix(in srgb, var(--c-ink) 12%, transparent); }
.card:hover :deep(.actions) .action,
.card:focus-within :deep(.actions) .action { opacity: 1; transform: translateY(0); }
</style>
