<script setup lang="ts">
import { computed, ref } from 'vue'

import type { EntryDetail } from '../../types/api'
import type { PublishCheck } from '../../editor/publish-checks'

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
  <aside v-if="open" class="publish-panel" role="dialog" aria-labelledby="publish-title">
    <div class="publish-panel__header">
      <h2 id="publish-title">发布文章</h2>
      <button type="button" aria-label="关闭发布面板" data-publish-close @click="emit('update:open', false)">×</button>
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
    <button type="button" data-publish-confirm :disabled="blocked || busy" @click="emit('publish', entry.id, entry.revision)">确认发布</button>
  </aside>
</template>

<style scoped>
.publish-panel { position: fixed; inset-block: 0; inset-inline-end: 0; z-index: 40; width: min(24rem, 100vw); padding: var(--space-5); overflow-y: auto; border-inline-start: 1px solid var(--c-line); background: var(--c-surface); }
.publish-panel__header { display: flex; justify-content: space-between; align-items: center; }
.publish-panel__header h2 { margin: 0 0 var(--space-5); font-size: 1rem; }
.publish-panel__header button { border: 0; background: transparent; font-size: 1.5rem; cursor: pointer; }
.publish-panel__reminders { display: grid; gap: var(--space-2); margin: var(--space-5) 0; padding: 0; border: 0; }
.publish-panel__reminders legend { margin-bottom: var(--space-2); color: var(--c-ink-muted); }
.publish-panel__lead { color: var(--c-danger); }
</style>
