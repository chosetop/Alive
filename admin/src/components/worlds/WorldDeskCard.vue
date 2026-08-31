<script setup lang="ts">
import { computed } from 'vue'

import type { AdminWorldDefinition } from '../../content-worlds/registry'
import type { AdminWorldSetting, EntryListItem, WorldKey, WorldStatus } from '../../types/api'
import { UiButton } from '../ui'

export interface WorldDeskSnapshot {
  setting: AdminWorldSetting
  recentEntry: EntryListItem | null
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
const leadText = computed(() => {
  if (props.snapshot.error) return props.snapshot.error
  if (props.snapshot.recentEntry) return props.snapshot.recentEntry.summary || '继续最近一次编辑。'
  return props.definition.emptyCopy
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
    <UiButton
      class="world-desk__enter-surface"
      variant="primary"
      data-world-enter
      :disabled="!snapshot.canEnter"
      @click="emit('enter', definition.key)"
    >
      <span class="world-desk__enter-copy">{{ enterLabel }}</span>
    </UiButton>

    <div class="world-desk__topline">
      <div>
        <p class="world-desk__eyebrow">{{ definition.directoryNoun }}</p>
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

    <p class="world-desk__description">{{ definition.description }}</p>

    <div class="world-desk__meta">
      <span class="world-desk__status" data-world-status>{{ statusText }}</span>
      <span>{{ snapshot.entryCount }} 篇{{ definition.directoryNoun }}</span>
      <span>{{ snapshot.categoryCount }} 个分类</span>
    </div>

    <div class="world-desk__body">
      <template v-if="snapshot.recentEntry">
        <p class="world-desk__label">最近编辑</p>
        <h3>{{ snapshot.recentEntry.title }}</h3>
      </template>
      <template v-else>
        <p class="world-desk__label">空白工作台</p>
        <h3>{{ definition.createLabel }}</h3>
      </template>
      <p
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
        @click="emit('retry', definition.key)"
      >
        重试最近编辑
      </UiButton>
    </div>
  </article>
</template>

<style scoped>
.world-desk {
  position: relative;
  display: grid;
  gap: var(--space-4);
  min-height: 22rem;
  padding: var(--space-5);
  border: 1px solid var(--c-glass-border);
  border-radius: var(--radius-surface);
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--c-paper) 82%, transparent), transparent 42%),
    var(--c-glass);
  box-shadow: var(--shadow-float);
  backdrop-filter: blur(18px) saturate(130%);
}

.world-desk__enter-surface {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  align-items: end;
  justify-content: stretch;
  padding: var(--space-5);
  border-radius: inherit;
  background: transparent;
  box-shadow: none;
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

.world-desk__eyebrow {
  margin: 0 0 var(--space-2);
  color: var(--c-ink-faint);
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.world-desk__settings {
  position: relative;
  z-index: 2;
  min-width: 4.5rem;
}

.world-desk__description {
  position: relative;
  z-index: 0;
  margin: 0;
  color: var(--c-ink-muted);
  font-size: 0.9375rem;
  line-height: 1.6;
  text-wrap: pretty;
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
  display: grid;
  align-content: start;
  gap: var(--space-2);
  min-height: 7.5rem;
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
}

@media (pointer: coarse) {
  .world-desk__settings,
  .world-desk__enter-copy,
  .world-desk__retry {
    min-height: 2.75rem;
  }
}
</style>
