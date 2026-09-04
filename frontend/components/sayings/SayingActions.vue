<script setup lang="ts">
import { computed, ref } from 'vue'

import type { SayingListItem } from '~/types'
import { markdownToText } from '~/utils/markdown'

const props = defineProps<{
  item: SayingListItem
}>()
const text = computed(() => markdownToText(props.item.content_md))
const copied = ref(false)

async function copyContent(): Promise<void> {
  if (!import.meta.client) return
  try {
    if (navigator.clipboard) await navigator.clipboard.writeText(text.value)
    else throw new Error('clipboard unavailable')
  } catch {
    const input = document.createElement('textarea')
    input.value = text.value
    input.style.position = 'fixed'
    input.style.opacity = '0'
    document.body.append(input)
    input.select()
    document.execCommand('copy')
    input.remove()
  }
  copied.value = true
  window.setTimeout(() => { copied.value = false }, 1400)
}
</script>

<template>
  <div class="actions" aria-label="片语操作">
    <button type="button" class="action" :aria-label="copied ? '已复制正文' : '复制正文'" :title="copied ? '已复制' : '复制正文'" @click="copyContent">
      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 8h10v12H8zM6 16H4V4h12v2" /></svg>
    </button>
  </div>
</template>

<style scoped>
.actions {
  display: flex;
  justify-content: flex-end;
  min-height: 2rem;
}

.action {
  display: grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 50%;
  background: var(--c-surface);
  color: var(--c-ink-muted);
  cursor: pointer;
  opacity: 0;
  transform: translateY(0.2rem);
  transition: opacity var(--duration-fast) var(--ease-out), transform var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
}

.actions:hover .action,
.actions:focus-within .action,
.action:focus-visible { opacity: 1; transform: translateY(0); }
.action svg { width: 1rem; height: 1rem; fill: none; stroke: currentColor; stroke-linecap: round; stroke-linejoin: round; stroke-width: 1.5; }

.action:hover,
.action:focus-visible {
  border-color: var(--c-accent);
  color: var(--c-accent);
  outline: none;
}
</style>
