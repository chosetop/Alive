<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'

import { fromFormDateTime, toFormDateTime } from '../../api'
import type { Category, EntryDetail, EntryPatchFields, EntryVisibility } from '../../types/api'
import { UiButton, UiIcon, UiIconButton } from '../ui'

const props = withDefaults(
  defineProps<{
    open: boolean
    entry: EntryDetail
    categories?: Category[]
    disabled?: boolean
  }>(),
  { categories: () => [], disabled: false },
)

const emit = defineEmits<{
  'update:open': [boolean]
  update: [EntryPatchFields]
  delete: []
}>()

const confirmingDelete = ref(false)
const returnFocus = shallowRef<HTMLElement | null>(null)

const VISIBILITIES: ReadonlyArray<{ value: EntryVisibility; label: string }> = [
  { value: 'public', label: '公开' },
  { value: 'unlisted', label: '不列出' },
  { value: 'private', label: '私密' },
]

function update(fields: EntryPatchFields): void {
  if (!props.disabled) emit('update', fields)
}

function close(): void {
  confirmingDelete.value = false
  emit('update:open', false)
  void nextTick(() => returnFocus.value?.focus())
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && props.open) close()
}

watch(() => props.open, (open) => {
  if (open && document.activeElement instanceof HTMLElement) returnFocus.value = document.activeElement
})

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

function confirmDelete(): void {
  confirmingDelete.value = false
  emit('delete')
}
</script>

<template>
  <aside
    v-if="open"
    class="article-settings"
    role="dialog"
    aria-labelledby="settings-title"
    data-writing-panel="settings"
    data-surface="elevated"
  >
    <div class="article-settings__header">
      <h2 id="settings-title">文章设置</h2>
      <UiIconButton label="关闭文章设置" data-settings-close @click="close">
        <UiIcon name="close" />
      </UiIconButton>
    </div>

    <div class="article-settings__body">
      <label class="field" for="e-title">标题</label>
      <input id="e-title" :value="entry.title" :disabled="disabled" @input="update({ title: ($event.target as HTMLInputElement).value })" />

      <label class="field" for="e-slug">slug</label>
      <input id="e-slug" :value="entry.slug" :disabled="disabled" @input="update({ slug: ($event.target as HTMLInputElement).value })" />
      <label class="field" for="e-category">分类</label>
      <select id="e-category" :value="entry.category_id" :disabled="disabled" @change="update({ category_id: Number(($event.target as HTMLSelectElement).value) })">
        <option value="0">未分类</option>
        <option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option>
      </select>

      <label class="field" for="e-summary">摘要</label>
      <textarea id="e-summary" :value="entry.summary" :disabled="disabled" @input="update({ summary: ($event.target as HTMLTextAreaElement).value })" />

      <label class="field" for="e-cover">封面 URL</label>
      <input id="e-cover" type="url" :value="entry.cover_url" placeholder="https://…" :disabled="disabled" @input="update({ cover_url: ($event.target as HTMLInputElement).value })" />

      <label class="field" for="e-happened">发生时间</label>
      <input id="e-happened" type="datetime-local" :value="toFormDateTime(entry.happened_at)" :disabled="disabled" @input="update({ happened_at: ($event.target as HTMLInputElement).value === '' ? '0001-01-01T00:00:00Z' : fromFormDateTime(($event.target as HTMLInputElement).value) })" />

      <fieldset>
        <legend class="field">可见性</legend>
        <label v-for="visibility in VISIBILITIES" :key="visibility.value" class="visibility-option">
          <input type="radio" name="settings-visibility" :value="visibility.value" :checked="entry.visibility === visibility.value" :disabled="disabled" @change="update({ visibility: visibility.value })" />
          {{ visibility.label }}
        </label>
      </fieldset>

      <div class="article-settings__danger">
        <template v-if="confirmingDelete">
          <p>删除后前台不可达，且没有恢复接口。</p>
          <div class="article-settings__danger-actions">
            <UiButton variant="danger" data-delete-confirm :disabled="disabled" @click="confirmDelete">确认删除</UiButton>
            <UiButton variant="quiet" :disabled="disabled" @click="confirmingDelete = false">取消</UiButton>
          </div>
        </template>
        <UiButton v-else variant="danger" data-settings-delete :disabled="disabled" @click="confirmingDelete = true">删除文章</UiButton>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.article-settings {
  position: fixed;
  inset-block: 0;
  inset-inline-end: 0;
  z-index: 40;
  width: min(26rem, calc(100vw - var(--space-4)));
  max-width: 100vw;
  padding: var(--space-5);
  overflow-y: auto;
  border-inline-start: 1px solid var(--c-glass-border);
  border-radius: var(--radius-surface) 0 0 var(--radius-surface);
  background: var(--c-surface);
  box-shadow: var(--shadow-float);
  overscroll-behavior: contain;
}

.article-settings__header { display: flex; justify-content: space-between; align-items: center; }
.article-settings__header { margin-bottom: var(--space-5); }
.article-settings__header h2 { font-size: 1rem; }
.article-settings__body { display: grid; gap: var(--space-2); min-width: 0; }
.field { color: var(--c-ink-muted); font-size: 0.8125rem; }
.article-settings input:not([type='radio']), .article-settings select, .article-settings textarea { width: 100%; min-height: 2.5rem; padding: var(--space-2) var(--space-3); border: 1px solid var(--c-line-strong); border-radius: var(--radius-control); background: var(--c-paper); color: var(--c-ink); }
.article-settings textarea { min-height: 5rem; resize: vertical; }
.article-settings fieldset { display: grid; gap: var(--space-2); margin: var(--space-3) 0; padding: 0; border: 0; }
.visibility-option { display: flex; align-items: center; gap: var(--space-2); min-height: 2.5rem; }
.article-settings__danger { margin-top: var(--space-5); padding-top: var(--space-4); border-top: 1px solid var(--c-line); }
.article-settings__danger p { margin-bottom: var(--space-3); color: var(--c-danger); font-size: 0.8125rem; text-wrap: pretty; }
.article-settings__danger-actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }

@media (max-width: 23.4375rem) {
  .article-settings {
    width: 100vw;
    padding: var(--space-4);
    border-radius: 0;
    overflow-x: clip;
  }
}
</style>
