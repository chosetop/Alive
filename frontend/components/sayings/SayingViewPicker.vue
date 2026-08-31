<script setup lang="ts">
import type { SayingView } from '~/types'

const props = defineProps<{
  view: SayingView
}>()

const emit = defineEmits<{
  change: [view: SayingView]
}>()

const options: Array<{ view: SayingView; label: string }> = [
  { view: 'stream', label: '流式' },
  { view: 'wall', label: '纸片墙' },
  { view: 'focus', label: '沉浸' },
]
</script>

<template>
  <div class="picker" role="radiogroup" aria-label="片语浏览方式">
    <button
      v-for="option in options"
      :key="option.view"
      type="button"
      class="choice"
      role="radio"
      :aria-checked="view === option.view"
      @click="emit('change', option.view)"
    >
      {{ option.label }}
    </button>
  </div>
</template>

<style scoped>
.picker {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

.choice {
  padding: 0.45rem 0.85rem;
  border: 1px solid var(--c-line-strong);
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-surface) 88%, var(--c-accent));
  color: var(--c-ink-muted);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  cursor: pointer;
}

.choice[aria-checked='true'] {
  border-color: var(--c-accent);
  color: var(--c-accent);
  background: color-mix(in srgb, var(--c-accent) 12%, var(--c-surface));
}
</style>
