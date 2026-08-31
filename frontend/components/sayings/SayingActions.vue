<script setup lang="ts">
import { computed } from 'vue'

import { rememberSayingAnchor } from '~/composables/useSayingView'

const props = defineProps<{
  shortId: string
}>()

const href = computed(() => `/sayings/${props.shortId}`)

function copyLink(): void {
  if (!import.meta.client || typeof window === 'undefined' || !window.navigator.clipboard) return
  void window.navigator.clipboard.writeText(new URL(href.value, window.location.origin).toString())
}

async function shareLink(): Promise<void> {
  if (!import.meta.client || typeof window === 'undefined') return

  const url = new URL(href.value, window.location.origin).toString()
  if (window.navigator.share) {
    await window.navigator.share({ url })
    return
  }

  copyLink()
}

function rememberAnchor(event: MouseEvent): void {
  if (!import.meta.client || typeof window === 'undefined') return

  const target = event.currentTarget
  if (!(target instanceof HTMLElement)) return

  const root = target.closest<HTMLElement>('[data-saying-id]')
  if (!root) return

  rememberSayingAnchor(window.history, props.shortId, root.offsetTop)
}
</script>

<template>
  <div class="actions" aria-label="片语操作">
    <NuxtLink class="action action--open" :to="href" @click="rememberAnchor">打开</NuxtLink>
    <button type="button" class="action" @click="copyLink">复制链接</button>
    <button type="button" class="action" @click="shareLink">分享</button>
  </div>
</template>

<style scoped>
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

.action {
  padding: 0.4rem 0.75rem;
  border: 1px solid var(--c-line-strong);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink-muted);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
  text-decoration: none;
  cursor: pointer;
}

.action:hover,
.action:focus-visible {
  border-color: var(--c-accent);
  color: var(--c-accent);
  outline: none;
}
</style>
