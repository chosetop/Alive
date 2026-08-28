<script setup lang="ts">
import { computed, ref } from 'vue'

import type { EntryDetail } from '../../types/api'
import type { PublishCheck } from '../../editor/publish-checks'
import { UiButton, UiIcon, UiIconButton } from '../ui'

const props = defineProps<{
  open: boolean
  entry: EntryDetail
  checks: { blockers: PublishCheck[]; reminders: PublishCheck[] }
  busy?: boolean
}>()

const emit = defineEmits<{
  'update:open': [boolean]
  publish: [number, number]
}>()

const acknowledged = ref<Set<PublishCheck['field']>>(new Set())
const remindersComplete = computed(() => props.checks.reminders.every((check) => acknowledged.value.has(check.field)))
const blocked = computed(() => props.checks.blockers.length > 0 || !remindersComplete.value)

function acknowledge(field: PublishCheck['field'], checked: boolean): void {
  const next = new Set(acknowledged.value)
  if (checked) next.add(field)
  else next.delete(field)
  acknowledged.value = next
}
</script>

<template>
  <aside
    v-if="open"
    class="publish-panel"
    role="dialog"
    aria-labelledby="publish-title"
    data-writing-panel="publish"
    data-surface="elevated"
  >
    <div class="publish-panel__header">
      <h2 id="publish-title">发布文章</h2>
      <UiIconButton label="关闭发布面板" data-publish-close @click="emit('update:open', false)">
        <UiIcon name="close" />
      </UiIconButton>
    </div>
    <p v-if="checks.blockers.length" class="publish-panel__lead">发布前还需要补齐：</p>
    <ul v-if="checks.blockers.length" data-publish-blockers>
      <li v-for="check in checks.blockers" :key="check.field">{{ check.label }}</li>
    </ul>
    <fieldset v-if="checks.reminders.length" class="publish-panel__reminders">
      <legend>发布提醒</legend>
      <label v-for="check in checks.reminders" :key="check.field">
        <input v-bind="{ ['data-reminder-' + check.field]: true }" type="checkbox" :checked="acknowledged.has(check.field)" @change="acknowledge(check.field, ($event.target as HTMLInputElement).checked)" />
        我知道{{ check.label }}为空
      </label>
    </fieldset>
    <p data-visibility-copy>发布后将以“{{ entry.visibility === 'public' ? '公开' : entry.visibility === 'unlisted' ? '不列出' : '私密' }}”状态显示。</p>
    <UiButton variant="primary" data-publish-confirm :disabled="blocked || busy" @click="emit('publish', entry.id, entry.revision)">确认发布</UiButton>
  </aside>
</template>

<style scoped>
.publish-panel { position: fixed; inset-block: 0; inset-inline-end: 0; z-index: 40; width: min(26rem, calc(100vw - var(--space-4))); max-width: 100vw; padding: var(--space-5); overflow-y: auto; border-inline-start: 1px solid var(--c-glass-border); border-radius: var(--radius-surface) 0 0 var(--radius-surface); background: var(--c-surface); box-shadow: var(--shadow-float); overscroll-behavior: contain; }
.publish-panel__header { display: flex; justify-content: space-between; align-items: center; }
.publish-panel__header { margin-bottom: var(--space-5); }
.publish-panel__header h2 { font-size: 1rem; }
.publish-panel ul { margin: var(--space-3) 0 var(--space-5); padding: var(--space-3) var(--space-5); border-radius: var(--radius-control); background: var(--c-danger-surface); color: var(--c-danger); font-size: 0.875rem; }
.publish-panel__reminders { display: grid; gap: var(--space-3); margin: var(--space-5) 0; padding: 0; border: 0; }
.publish-panel__reminders legend { margin-bottom: var(--space-2); color: var(--c-ink-muted); }
.publish-panel__reminders label { display: flex; align-items: flex-start; gap: var(--space-2); }
.publish-panel__lead { color: var(--c-danger); }
.publish-panel [data-visibility-copy] { margin: var(--space-5) 0; color: var(--c-ink-muted); font-size: 0.875rem; text-wrap: pretty; }

@media (max-width: 23.4375rem) {
  .publish-panel {
    width: 100vw;
    padding: var(--space-4);
    border-radius: 0;
    overflow-x: clip;
  }
}
</style>
