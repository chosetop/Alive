<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'

import { fromFormDateTime, toFormDateTime } from '../../api'
import type { Category, EntryDetail, EntryPatchFields, EntryVisibility } from '../../types/api'
import { UiButton, UiIcon, UiIconButton, UiSelect } from '../ui'
import TagPicker from './TagPicker.vue'
import type { Tag } from '../../api/tags'
import { uploadMedia } from '../../media'

const props = withDefaults(
  defineProps<{
    open: boolean
    entry: EntryDetail
    categories?: Category[]
    tags?: Tag[]
    disabled?: boolean
    saveable?: boolean
    dirty?: boolean
    saving?: boolean
    saveError?: string | null
    showRichActions?: boolean
  }>(),
  {
    categories: () => [],
    disabled: false,
    saveable: false,
    dirty: false,
    saving: false,
    saveError: null,
    showRichActions: true,
  },
)

const emit = defineEmits<{
  'update:open': [boolean]
  update: [EntryPatchFields]
  revision: [number]
  'tags-saved': [revision: number, tags: Tag[]]
  delete: []
  save: []
}>()

const confirmingDelete = ref(false)
const returnFocus = shallowRef<HTMLElement | null>(null)
const coverProgress = ref<number | null>(null)
const coverError = ref<string | null>(null)

const VISIBILITIES: ReadonlyArray<{ value: EntryVisibility; label: string }> = [
  { value: 'public', label: '公开' },
  { value: 'unlisted', label: '不列出' },
  { value: 'private', label: '私密' },
]
const categoryOptions = computed(() => [
  { value: '0', label: '未分类' },
  ...props.categories.map((category) => ({ value: String(category.id), label: category.name })),
])

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

async function chooseCover(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  coverError.value = null
  coverProgress.value = 0
  try {
    const media = await uploadMedia(file, {
      entryId: props.entry.id,
      onProgress: (value) => { coverProgress.value = value },
    })
    update({ cover_url: media.url })
  } catch (cause) {
    coverError.value = cause instanceof Error ? cause.message : '封面上传失败'
  } finally {
    coverProgress.value = null
    input.value = ''
  }
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
      <h2 id="settings-title">{{ entry.world === 'video' ? '影像设置' : '文章设置' }}</h2>
      <UiIconButton :label="entry.world === 'video' ? '关闭影像设置' : '关闭文章设置'" data-settings-close @click="close">
        <UiIcon name="close" />
      </UiIconButton>
    </div>

    <div class="article-settings__body">
      <label class="field" for="e-title">标题</label>
      <input id="e-title" :value="entry.title" :disabled="disabled" @input="update({ title: ($event.target as HTMLInputElement).value })" />

      <template v-if="entry.world !== 'saying'">
        <label class="field" for="e-slug">slug</label>
        <input id="e-slug" :value="entry.slug" :disabled="disabled" @input="update({ slug: ($event.target as HTMLInputElement).value })" />
      </template>
      <label class="field" for="e-category">分类</label>
      <UiSelect
        :model-value="String(entry.category_id)"
        :options="categoryOptions"
        label="分类"
        :disabled="disabled"
        @change="update({ category_id: Number($event) })"
      />

      <label class="field" for="e-happened">发生时间</label>
      <input id="e-happened" type="datetime-local" :value="toFormDateTime(entry.happened_at)" :disabled="disabled" @input="update({ happened_at: ($event.target as HTMLInputElement).value === '' ? '0001-01-01T00:00:00Z' : fromFormDateTime(($event.target as HTMLInputElement).value) })" />

      <section v-if="entry.world === 'video'" class="settings-section cover-setting" aria-label="影像封面">
        <div class="cover-setting__heading">
          <h3>封面</h3>
          <span>用于影像列表与分享预览</span>
        </div>
        <img v-if="entry.cover_url" class="cover-setting__preview" :src="entry.cover_url" alt="当前影像封面" data-cover-preview />
        <label class="cover-setting__picker" for="e-cover">
          {{ entry.cover_url ? '更换封面' : '选择封面' }}
          <input id="e-cover" type="file" accept="image/jpeg,image/png,image/webp,image/gif" :disabled="disabled || coverProgress !== null" data-cover-input @change="chooseCover" />
        </label>
        <progress v-if="coverProgress !== null" :value="coverProgress" max="100" aria-label="封面上传进度">{{ coverProgress }}%</progress>
        <p v-if="coverError" class="article-settings__error" role="alert">{{ coverError }}</p>
      </section>

      <fieldset>
        <legend class="field">可见性</legend>
        <label v-for="visibility in VISIBILITIES" :key="visibility.value" class="visibility-option">
          <input type="radio" name="settings-visibility" :value="visibility.value" :checked="entry.visibility === visibility.value" :disabled="disabled" @change="update({ visibility: visibility.value })" />
          {{ visibility.label }}
        </label>
      </fieldset>

      <section v-if="showRichActions && entry.world === 'journal' && tags" class="settings-section" aria-label="文章标签">
        <h3>标签</h3>
        <TagPicker
          :entry-id="entry.id"
          :revision="entry.revision"
          :selected="tags"
          :disabled="disabled"
          @saved="(revision, nextTags) => emit('tags-saved', revision, nextTags)"
        />
      </section>

      <div v-if="saveable" class="article-settings__save">
        <p v-if="saveError" class="article-settings__error" role="alert">{{ saveError }}</p>
        <UiButton variant="primary" data-settings-save :loading="saving" :disabled="disabled || !dirty" @click="emit('save')">
          {{ saving ? '保存中…' : '保存修改' }}
        </UiButton>
      </div>

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
.settings-section { display: grid; gap: var(--space-2); margin: var(--space-3) 0; padding-top: var(--space-3); border-top: 1px solid var(--c-line); }
.settings-section h3 { font-size: 0.8125rem; font-weight: 600; }
.cover-setting { gap: var(--space-3); }
.cover-setting__heading { display: flex; align-items: baseline; justify-content: space-between; gap: var(--space-3); }
.cover-setting__heading span { color: var(--c-ink-faint); font-size: 0.75rem; }
.cover-setting__preview { display: block; width: 100%; aspect-ratio: 16 / 9; object-fit: cover; border: 1px solid var(--c-line); border-radius: var(--radius-control); background: var(--c-paper); }
.cover-setting__picker { position: relative; display: grid; place-items: center; min-height: 2.5rem; border: 1px solid var(--c-line-strong); border-radius: var(--radius-control); color: var(--c-ink); font-size: 0.8125rem; cursor: pointer; }
.cover-setting__picker:focus-within { outline: 2px solid var(--c-accent); outline-offset: 2px; }
.cover-setting__picker input { position: absolute; inset: 0; opacity: 0; cursor: pointer; }
.cover-setting progress { width: 100%; }
.visibility-option { display: flex; align-items: center; gap: var(--space-2); min-height: 2.5rem; }
.article-settings__danger { margin-top: var(--space-5); padding-top: var(--space-4); border-top: 1px solid var(--c-line); }
.article-settings__save { display: grid; gap: var(--space-2); margin-top: var(--space-4); }
.article-settings__error { color: var(--c-danger); font-size: 0.8125rem; }
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
