<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'

import { fromFormDateTime, toFormDateTime } from '../../api'
import type { Category, EntryDetail, EntryPatchFields, EntryType, EntryVisibility } from '../../types/api'

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

const TYPES: ReadonlyArray<{ value: EntryType; label: string }> = [
  { value: 'journal', label: '日志' },
  { value: 'book', label: '书' },
  { value: 'movie', label: '影' },
  { value: 'music', label: '乐' },
  { value: 'travel', label: '行' },
  { value: 'photo', label: '影像' },
]

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
  <aside v-if="open" class="article-settings" role="dialog" aria-labelledby="settings-title">
    <div class="article-settings__header">
      <h2 id="settings-title">文章设置</h2>
      <button type="button" aria-label="关闭文章设置" data-settings-close @click="close">×</button>
    </div>

    <div class="article-settings__body">
      <label class="field" for="e-title">标题</label>
      <input id="e-title" :value="entry.title" :disabled="disabled" @input="update({ title: ($event.target as HTMLInputElement).value })" />

      <label class="field" for="e-slug">slug</label>
      <input id="e-slug" :value="entry.slug" :disabled="disabled" @input="update({ slug: ($event.target as HTMLInputElement).value })" />

      <label class="field" for="e-type">类型</label>
      <select id="e-type" :value="entry.type" :disabled="disabled" @change="update({ type: ($event.target as HTMLSelectElement).value as EntryType })">
        <option v-for="type in TYPES" :key="type.value" :value="type.value">{{ type.label }}</option>
      </select>

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
        <label v-for="visibility in VISIBILITIES" :key="visibility.value">
          <input type="radio" name="settings-visibility" :value="visibility.value" :checked="entry.visibility === visibility.value" :disabled="disabled" @change="update({ visibility: visibility.value })" />
          {{ visibility.label }}
        </label>
      </fieldset>

      <div class="article-settings__danger">
        <template v-if="confirmingDelete">
          <p>删除后前台不可达，且没有恢复接口。</p>
          <button type="button" data-delete-confirm :disabled="disabled" @click="confirmDelete">确认删除</button>
          <button type="button" :disabled="disabled" @click="confirmingDelete = false">取消</button>
        </template>
        <button v-else type="button" data-settings-delete :disabled="disabled" @click="confirmingDelete = true">删除文章</button>
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
  width: min(24rem, 100vw);
  padding: var(--space-5);
  overflow-y: auto;
  border-inline-start: 1px solid var(--c-line);
  background: var(--c-surface);
}

.article-settings__header { display: flex; justify-content: space-between; align-items: center; }
.article-settings__header h2 { margin: 0 0 var(--space-5); font-size: 1rem; }
.article-settings__header button { border: 0; background: transparent; font-size: 1.5rem; cursor: pointer; }
.article-settings__body { display: grid; gap: var(--space-2); }
.field { color: var(--c-ink-muted); font-size: 0.8125rem; }
.article-settings input:not([type='radio']), .article-settings select, .article-settings textarea { width: 100%; padding: var(--space-2); border: 1px solid var(--c-line); border-radius: var(--radius-sm); background: var(--c-surface); color: var(--c-ink); }
.article-settings textarea { min-height: 5rem; resize: vertical; }
.article-settings fieldset { display: grid; gap: var(--space-2); margin: var(--space-3) 0; padding: 0; border: 0; }
.article-settings__danger { margin-top: var(--space-5); padding-top: var(--space-4); border-top: 1px solid var(--c-line); }
.article-settings__danger button { margin-inline-end: var(--space-2); }
</style>
