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
  columns: 3 16rem;
  column-gap: var(--space-4);
}

.card {
  position: relative;
  display: grid;
  gap: var(--space-4);
  width: 100%;
  margin: 0 0 var(--space-4);
  padding: var(--card-padding, var(--space-5));
  overflow: hidden;
  border: 1px solid var(--c-line);
  border-radius: var(--card-radius, 1.25rem);
  background:
    radial-gradient(circle at 100% 0, color-mix(in srgb, var(--c-accent) var(--card-tint, 7%), transparent) 0, transparent 48%),
    var(--c-surface);
  box-shadow: 0 8px 24px color-mix(in srgb, var(--c-ink) 6%, transparent), inset 0 1px color-mix(in srgb, var(--c-surface) 80%, white 20%);
  break-inside: avoid;
  transition: transform var(--duration-mid) var(--ease-out), box-shadow var(--duration-mid) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
}

.card::before {
  position: absolute;
  top: 0;
  left: var(--space-5);
  width: 2.25rem;
  height: 2px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-accent) 62%, var(--c-line));
  content: '';
}

.card:nth-child(3n + 1) {
  --card-padding: 1.75rem 1.5rem 1.4rem;
  --card-radius: 1.55rem 1rem 1.35rem 0.9rem;
  --card-tint: 10%;
}

.card:nth-child(3n + 2) {
  --card-padding: 1.4rem 1.5rem 1.65rem;
  --card-radius: 1rem 1.5rem 0.95rem 1.4rem;
  --card-tint: 5%;
}

.card:nth-child(3n) {
  --card-padding: 1.6rem 1.65rem 1.5rem;
  --card-radius: 1.35rem 0.9rem 1.55rem 1rem;
  --card-tint: 8%;
}

.card:hover,
.card:focus-within {
  border-color: var(--c-line-strong);
  box-shadow: 0 14px 32px color-mix(in srgb, var(--c-ink) 11%, transparent), inset 0 1px color-mix(in srgb, var(--c-surface) 80%, white 20%);
  transform: translateY(-3px);
}

.card:hover :deep(.actions) .action,
.card:focus-within :deep(.actions) .action { opacity: 1; transform: translateY(0); }

@media (max-width: 56rem) {
  .wall {
    columns: 2 15rem;
  }
}

@media (max-width: 38rem) {
  .wall {
    columns: 1;
  }
}
</style>
