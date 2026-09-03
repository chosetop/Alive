<script setup lang="ts">
import { computed } from 'vue'

import type { AdminWorldDefinition } from '../../content-worlds/registry'
import { renderMarkdown } from '../../editor/render-preview'
import type { AdminWorldSetting, EntryListItem, WorldKey, WorldStatus } from '../../types/api'
import { UiButton } from '../ui'

export interface WorldDeskSnapshot {
  setting: AdminWorldSetting
  recentEntry: EntryListItem | null
  recentContentMd: string | null
  entryCount: number
  categoryCount: number
  error: string | null
  canEnter: boolean
  canRetry: boolean
}

const props = defineProps<{
  definition: AdminWorldDefinition
  snapshot: WorldDeskSnapshot
}>()

const emit = defineEmits<{
  enter: [WorldKey]
  settings: [WorldKey]
  retry: [WorldKey]
}>()

const STATUS_TEXT: Record<WorldStatus, string> = {
  unopened: '未开放',
  open: '已开放',
  hidden: '暂时隐藏',
}

const statusText = computed(() => STATUS_TEXT[props.snapshot.setting.status])
const previewHtml = computed(() => {
  if (
    props.definition.material !== 'manuscript'
    || !props.snapshot.recentEntry
    || !props.snapshot.recentContentMd
  ) return ''
  return renderMarkdown(props.snapshot.recentContentMd)
})
const leadText = computed(() => {
  if (props.snapshot.error) return props.snapshot.error
  if (props.snapshot.recentEntry) return props.snapshot.recentEntry.summary || '继续最近一次编辑。'
  return '还没有内容。开始第一篇。'
})

const enterLabel = computed(() => {
  if (!props.snapshot.canEnter) return '暂时无法进入'
  return props.snapshot.recentEntry ? '进入最近编辑' : '进入空白工作台'
})
</script>

<template>
  <article
    class="world-desk"
    :data-world-desk="definition.key"
    :data-world-material="definition.material"
    data-surface="glass"
  >
    <div class="world-desk__topline">
      <div @click="emit('enter', definition.key)">
        <h2>{{ snapshot.setting.nav_label }}</h2>
      </div>
      <UiButton
        class="world-desk__settings"
        variant="quiet"
        data-world-settings
        @click="emit('settings', definition.key)"
      >
        设置
      </UiButton>
    </div>

    <div class="world-desk__meta" @click="emit('enter', definition.key)">
      <span class="world-desk__status" data-world-status>{{ statusText }}</span>
      <span>{{ snapshot.entryCount }} 篇内容</span>
      <span>{{ snapshot.categoryCount }} 个分类</span>
    </div>

    <div
      class="world-desk__body"
      :data-has-preview="previewHtml ? 'true' : undefined"
      @click="emit('enter', definition.key)"
    >
      <template v-if="snapshot.recentEntry">
        <p class="world-desk__label">最近编辑</p>
        <h3>{{ snapshot.recentEntry.title }}</h3>
      </template>
      <template v-else>
        <p class="world-desk__label">空白工作台</p>
        <h3>开始创作</h3>
      </template>
      <div
        v-if="previewHtml"
        class="world-desk__preview"
        data-world-preview
        v-html="previewHtml"
      />
      <p
        v-else
        class="world-desk__lead"
        :data-world-error="snapshot.error ? 'true' : undefined"
      >
        {{ leadText }}
      </p>

      <UiButton
        v-if="snapshot.canRetry"
        class="world-desk__retry"
        variant="secondary"
        data-world-retry
        @click.stop="emit('retry', definition.key)"
      >
        重试最近编辑
      </UiButton>
    </div>

    <UiButton
      class="world-desk__enter-surface"
      variant="primary"
      data-world-enter
      :disabled="!snapshot.canEnter"
      @click="emit('enter', definition.key)"
    >
      <span class="world-desk__enter-copy">{{ enterLabel }}</span>
    </UiButton>
  </article>
</template>

<style scoped>
.world-desk {
  position: relative;
  display: grid;
  grid-template-rows: auto auto minmax(7.5rem, 1fr);
  gap: var(--space-3);
  min-height: 20rem;
  padding: 1.25rem;
  border: 1px solid transparent;
  border-radius: var(--radius-surface);
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--c-paper) 82%, transparent), transparent 42%),
    var(--c-glass);
  box-shadow:
    0 10px 28px color-mix(in srgb, var(--c-shadow-float) 42%, transparent),
    inset 0 1px 0 color-mix(in srgb, var(--c-paper) 32%, transparent);
  backdrop-filter: blur(18px) saturate(130%);
  transition: border-color var(--motion-fast) ease, box-shadow var(--motion-fast) ease;
}

.world-desk:hover {
  border-color: var(--c-accent);
  box-shadow: var(--shadow-float), 0 0 0 1px color-mix(in srgb, var(--c-accent) 18%, transparent);
}

.world-desk__enter-surface {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  align-items: end;
  justify-content: stretch;
  padding: 1.25rem;
  border-radius: inherit;
  border: 1px solid transparent;
  background: transparent;
  box-shadow: none;
}

/* The stretched hit area is transparent; only its visible CTA copy should
   react to hover. This prevents the primary-button token from flooding the
   entire desk surface. */
.world-desk__enter-surface:hover:not(:disabled),
.world-desk__enter-surface:focus-visible:not(:disabled) {
  border-color: transparent;
  background: transparent;
  color: inherit;
  outline: none;
}

.world-desk__enter-surface:hover:not(:disabled) .world-desk__enter-copy,
.world-desk__enter-surface:focus-visible:not(:disabled) .world-desk__enter-copy {
  background: color-mix(in srgb, var(--c-accent) 88%, var(--c-ink));
}

.world-desk__enter-surface:focus-visible:not(:disabled) .world-desk__enter-copy {
  outline: 2px solid var(--c-focus);
  outline-offset: 2px;
}

.world-desk__enter-copy {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 2.75rem;
  border: 1px solid transparent;
  border-radius: var(--radius-control);
  background: var(--c-accent);
  color: var(--c-on-accent);
}

.world-desk__enter-surface:disabled .world-desk__enter-copy {
  border-color: var(--c-line);
  background: var(--c-line);
  color: var(--c-ink-faint);
}

.world-desk__topline {
  position: relative;
  z-index: 2;
  pointer-events: none;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
}

.world-desk__topline h2 {
  margin: 0;
  font-family: var(--font-heading);
  font-size: clamp(1.4rem, 2vw, 1.8rem);
  line-height: 1.05;
}

.world-desk__settings {
  position: relative;
  z-index: 2;
  min-height: 2.75rem;
  min-width: 4.5rem;
  pointer-events: auto;
}

.world-desk__meta {
  position: relative;
  z-index: 0;
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  color: var(--c-ink-faint);
  font-size: 0.8125rem;
}

.world-desk__meta span {
  display: inline-flex;
  align-items: center;
  min-height: 2rem;
  padding: 0 var(--space-3);
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-paper) 74%, transparent);
}

.world-desk__status {
  color: var(--c-ink);
}

.world-desk__body {
  position: relative;
  z-index: 2;
  pointer-events: none;
  display: grid;
  align-content: start;
  gap: var(--space-2);
  min-height: 7.5rem;
  padding-bottom: calc(2.75rem + var(--space-3));
}

.world-desk__body[data-has-preview='true'] {
  grid-template-rows: auto auto minmax(0, 1fr);
}

.world-desk__preview {
  min-height: 0;
  padding: var(--space-4) var(--space-5);
  overflow: hidden;
  border-inline-start: 1px solid color-mix(in srgb, var(--c-accent) 32%, var(--c-line));
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
  background: color-mix(in srgb, var(--c-paper) 68%, transparent);
  color: var(--c-prose);
  font-family: var(--font-prose);
  font-size: 0.9375rem;
  line-height: 1.85;
  -webkit-mask-image: linear-gradient(to bottom, #000 0%, #000 82%, transparent 100%);
  mask-image: linear-gradient(to bottom, #000 0%, #000 82%, transparent 100%);
}

.world-desk__preview :deep(h2),
.world-desk__preview :deep(h3),
.world-desk__preview :deep(h4) {
  margin: 1.15em 0 0.45em;
  color: var(--c-ink);
  font-family: var(--font-heading);
  line-height: 1.35;
}

.world-desk__preview :deep(p),
.world-desk__preview :deep(blockquote),
.world-desk__preview :deep(ul),
.world-desk__preview :deep(ol),
.world-desk__preview :deep(pre) {
  margin: 0 0 0.85em;
}

.world-desk__preview :deep(blockquote) {
  padding-inline-start: var(--space-4);
  border-inline-start: 1px solid var(--c-line-strong);
  color: var(--c-ink-muted);
}

.world-desk__preview :deep(img) {
  max-width: 100%;
  height: auto;
}

.world-desk__label {
  margin: 0;
  color: var(--c-ink-faint);
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.world-desk__body h3 {
  margin: 0;
  font-size: 1.125rem;
  line-height: 1.3;
}

.world-desk__lead {
  margin: 0;
  color: var(--c-ink-muted);
  font-size: 0.9375rem;
  line-height: 1.65;
  text-wrap: pretty;
}

.world-desk__lead[data-world-error='true'] {
  color: var(--c-danger);
}

.world-desk__retry {
  width: fit-content;
  min-height: 2.5rem;
  pointer-events: auto;
}

@media (max-width: 48rem) {
  .world-desk {
    min-height: auto;
    padding: var(--space-4);
  }

  .world-desk__body {
    min-height: auto;
    padding-bottom: calc(2.75rem + var(--space-3));
  }

  .world-desk__preview {
    max-height: 8.5rem;
    padding: var(--space-3) var(--space-4);
  }
}

@media (pointer: coarse) {
  .world-desk__settings,
  .world-desk__enter-copy,
  .world-desk__retry {
    min-height: 2.75rem;
  }
}
</style>
