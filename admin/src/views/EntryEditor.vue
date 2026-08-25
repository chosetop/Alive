<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  buildEntryPatch,
  categoriesApi,
  entriesApi,
  isEmptyPatch,
  toFormDateTime,
  toUserMessage,
  type EntryFormState,
} from '../api'
import type { Category, EntryDetail, EntryStatus, EntryType, EntryVisibility } from '../types/api'
import MarkdownEditor from '../components/MarkdownEditor.vue'

/**
 * Create and edit one entry.
 *
 * Both modes share this view because they share every field. What differs is
 * which request saves it, and whether the status controls exist at all: an entry
 * that has never been created cannot be published.
 */

const props = defineProps<{
  /** Absent when creating. From the route, so it arrives as a string. */
  id?: string
}>()

const router = useRouter()

const isCreate = computed(() => props.id === undefined)
const entryId = computed(() => (props.id === undefined ? null : Number(props.id)))

/** The record as loaded, kept to diff against. Null while creating. */
const original = ref<EntryDetail | null>(null)
const categories = ref<Category[]>([])

const isLoading = ref(true)
const loadError = ref<string | null>(null)
const isSaving = ref(false)
const saveError = ref<string | null>(null)
const fieldErrors = ref<Record<string, string>>({})
/** Cleared on the next edit, so it cannot claim a stale save is still current. */
const savedAt = ref<Date | null>(null)

const isTransitioning = ref(false)
const isDeleting = ref(false)
const confirmingDelete = ref(false)

const form = ref<EntryFormState>({
  title: '',
  slug: '',
  summary: '',
  contentMd: '',
  coverUrl: '',
  type: 'journal',
  visibility: 'public',
  categoryId: 0,
  happenedAt: '',
})

const TYPES: ReadonlyArray<{ value: EntryType; label: string }> = [
  { value: 'journal', label: '日志' },
  { value: 'book', label: '书' },
  { value: 'movie', label: '影' },
  { value: 'music', label: '乐' },
  { value: 'travel', label: '行' },
  { value: 'photo', label: '影像' },
]

const VISIBILITIES: ReadonlyArray<{ value: EntryVisibility; label: string; hint: string }> = [
  { value: 'public', label: '公开', hint: '出现在列表里，凭 slug 也能打开' },
  { value: 'unlisted', label: '不列出', hint: '不在列表、计数、sitemap 里，但有链接就能打开。这不是访问控制' },
  { value: 'private', label: '私密', hint: '前台完全不可达' },
]

const STATUS_LABEL: Record<EntryStatus, string> = {
  draft: '草稿',
  published: '已发布',
  archived: '已归档',
}

const SLUG_PATTERN = /^[a-z0-9]+(-[a-z0-9]+)*$/

const localError = computed<string | null>(() => {
  if (form.value.title.trim() === '') return '标题不能为空'
  if ([...form.value.title].length > 255) return '标题最长 255 个字符'
  if (form.value.slug === '') return 'slug 不能为空'
  if (form.value.slug.length > 255) return 'slug 最长 255 个字符'
  if (!SLUG_PATTERN.test(form.value.slug)) {
    return 'slug 只能是小写字母和数字，用单个连字符分隔'
  }
  // content_md is required on create. On edit an empty body is a real edit —
  // emptying a draft you are rewriting — so it is only blocked here.
  if (isCreate.value && form.value.contentMd.trim() === '') return '正文不能为空'
  return null
})

const canSave = computed(() => !isSaving.value && localError.value === null)

const currentStatus = computed<EntryStatus | null>(() => original.value?.status ?? null)

async function load(): Promise<void> {
  isLoading.value = true
  loadError.value = null
  try {
    // Categories are needed in both modes, for the picker.
    const [cats, entry] = await Promise.all([
      categoriesApi.listCategoriesAdmin(),
      entryId.value === null ? Promise.resolve(null) : entriesApi.getEntry(entryId.value),
    ])
    categories.value = cats

    if (entry !== null) {
      original.value = entry
      form.value = {
        title: entry.title,
        slug: entry.slug,
        summary: entry.summary,
        contentMd: entry.content_md,
        coverUrl: entry.cover_url,
        type: entry.type,
        visibility: entry.visibility,
        categoryId: entry.category_id,
        happenedAt: toFormDateTime(entry.happened_at),
      }
    }
  } catch (error) {
    loadError.value = toUserMessage(error)
  } finally {
    isLoading.value = false
  }
}

onMounted(load)

function applyError(error: unknown): void {
  saveError.value = toUserMessage(error)
  fieldErrors.value = (error as { fields?: Record<string, string> }).fields ?? {}
}

/** Markdown arrives from the editor one way; this is the only writer of contentMd. */
function handleContentUpdate(markdown: string): void {
  form.value.contentMd = markdown
  savedAt.value = null
}

async function save(): Promise<void> {
  if (!canSave.value) return
  isSaving.value = true
  saveError.value = null
  fieldErrors.value = {}

  try {
    if (original.value === null) {
      const created = await entriesApi.createEntry({
        title: form.value.title.trim(),
        slug: form.value.slug,
        content_md: form.value.contentMd,
        type: form.value.type,
        visibility: form.value.visibility,
        summary: form.value.summary,
        cover_url: form.value.coverUrl,
        category_id: form.value.categoryId,
        ...(form.value.happenedAt === ''
          ? {}
          : { happened_at: new Date(form.value.happenedAt).toISOString() }),
      })
      // Redirect to the edit route so a second save patches instead of trying to
      // create the same slug again, which would be a 409.
      await router.replace({ name: 'entry-edit', params: { id: String(created.id) } })
      original.value = created
      savedAt.value = new Date()
      return
    }

    const patch = buildEntryPatch(original.value, form.value)
    if (isEmptyPatch(patch)) {
      // An empty patch is a 400 by design, and nothing changed, so there is
      // nothing to report but also nothing wrong.
      savedAt.value = new Date()
      return
    }
    original.value = await entriesApi.updateEntry(original.value.id, patch)
    savedAt.value = new Date()
  } catch (error) {
    applyError(error)
  } finally {
    isSaving.value = false
  }
}

/**
 * The three status transitions. Each is its own endpoint because PATCH refuses
 * `status` outright — see docs/api.md 4.4.
 */
async function transition(action: 'publish' | 'unpublish' | 'archive'): Promise<void> {
  if (original.value === null || isTransitioning.value) return
  isTransitioning.value = true
  saveError.value = null
  try {
    const id = original.value.id
    original.value =
      action === 'publish'
        ? await entriesApi.publishEntry(id)
        : action === 'unpublish'
          ? await entriesApi.unpublishEntry(id)
          : await entriesApi.archiveEntry(id)
  } catch (error) {
    applyError(error)
  } finally {
    isTransitioning.value = false
  }
}

async function handleDelete(): Promise<void> {
  if (original.value === null || isDeleting.value) return
  isDeleting.value = true
  try {
    await entriesApi.deleteEntry(original.value.id)
    await router.replace({ name: 'entries' })
  } catch (error) {
    applyError(error)
    isDeleting.value = false
  }
}
</script>

<template>
  <div class="page">
    <p v-if="isLoading" class="state">载入中…</p>
    <p v-else-if="loadError" class="alert" role="alert">{{ loadError }}</p>

    <template v-else>
      <header class="head">
        <div class="head-main">
          <h1 class="title">{{ isCreate ? '新建内容' : '编辑内容' }}</h1>
          <div v-if="currentStatus" class="status-line">
            <span class="badge" :class="`badge--${currentStatus}`">
              {{ STATUS_LABEL[currentStatus] }}
            </span>
            <!-- published_at is written once and never moves, so it is shown as
                 the original publication date even after unpublishing. -->
            <span v-if="original?.published_at" class="hint">
              首次发布于 {{ new Date(original.published_at).toLocaleString('zh-CN') }}
            </span>
            <span class="hint">{{ original?.word_count }} 字</span>
          </div>
        </div>
        <RouterLink class="btn" :to="{ name: 'entries' }">返回列表</RouterLink>
      </header>

      <div class="fields">
        <div class="field">
          <label class="label" for="e-title">标题</label>
          <input
            id="e-title"
            v-model="form.title"
            class="input input--title"
            :class="{ 'input--invalid': fieldErrors.title }"
            type="text"
            maxlength="255"
            :disabled="isSaving"
            @input="savedAt = null"
          />
          <p v-if="fieldErrors.title" class="field-error">{{ fieldErrors.title }}</p>
        </div>

        <div class="row">
          <div class="field">
            <label class="label" for="e-slug">slug</label>
            <input
              id="e-slug"
              v-model="form.slug"
              class="input input--mono"
              :class="{ 'input--invalid': fieldErrors.slug }"
              type="text"
              maxlength="255"
              spellcheck="false"
              :disabled="isSaving"
              @input="savedAt = null"
            />
            <p class="hint">URL 里的那一段，自己填。改它不会和自己冲突。</p>
            <p v-if="fieldErrors.slug" class="field-error">{{ fieldErrors.slug }}</p>
          </div>

          <div class="field">
            <label class="label" for="e-type">类型</label>
            <select
              id="e-type"
              v-model="form.type"
              class="input"
              :disabled="isSaving"
              @change="savedAt = null"
            >
              <option v-for="t in TYPES" :key="t.value" :value="t.value">{{ t.label }}</option>
            </select>
          </div>

          <div class="field">
            <label class="label" for="e-category">分类</label>
            <select
              id="e-category"
              v-model.number="form.categoryId"
              class="input"
              :class="{ 'input--invalid': fieldErrors.category_id }"
              :disabled="isSaving"
              @change="savedAt = null"
            >
              <!-- 0 is a real value meaning uncategorised, not a placeholder. -->
              <option :value="0">未分类</option>
              <option v-for="c in categories" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
            <p v-if="fieldErrors.category_id" class="field-error">{{ fieldErrors.category_id }}</p>
          </div>
        </div>

        <div class="field">
          <label class="label" for="e-summary">摘要</label>
          <textarea
            id="e-summary"
            v-model="form.summary"
            class="input textarea"
            rows="2"
            :disabled="isSaving"
            @input="savedAt = null"
          ></textarea>
          <p class="hint">列表和 meta description 用。留空即清除。</p>
        </div>

        <div class="field">
          <label class="label">正文</label>
          <!-- Mounted only once the content is known, and keyed by id: Milkdown
               reads its initial value once, so switching entries must build a
               new editor rather than try to swap the document underneath. -->
          <MarkdownEditor
            :key="original?.id ?? 'new'"
            :initial-value="form.contentMd"
            @update="handleContentUpdate"
          />
          <p class="hint">所见即所得，存的是 Markdown 源文本。</p>
        </div>

        <div class="row">
          <div class="field">
            <label class="label" for="e-cover">封面 URL</label>
            <input
              id="e-cover"
              v-model="form.coverUrl"
              class="input input--mono"
              type="url"
              :disabled="isSaving"
              @input="savedAt = null"
            />
          </div>

          <div class="field">
            <label class="label" for="e-happened">发生时间</label>
            <input
              id="e-happened"
              v-model="form.happenedAt"
              class="input"
              type="datetime-local"
              :disabled="isSaving"
              @input="savedAt = null"
            />
            <p class="hint">事情发生的时间，不是写作时间。留空即清除。</p>
          </div>
        </div>

        <fieldset class="field fieldset">
          <legend class="label">可见性</legend>
          <label v-for="v in VISIBILITIES" :key="v.value" class="radio">
            <input
              v-model="form.visibility"
              type="radio"
              :value="v.value"
              :disabled="isSaving"
              @change="savedAt = null"
            />
            <span class="radio-label">{{ v.label }}</span>
            <span class="radio-hint">{{ v.hint }}</span>
          </label>
        </fieldset>
      </div>

      <p v-if="localError && (form.title !== '' || form.slug !== '')" class="alert alert--soft">
        {{ localError }}
      </p>
      <p v-if="saveError" class="alert" role="alert">{{ saveError }}</p>

      <div class="actions">
        <button class="btn btn--primary" type="button" :disabled="!canSave" @click="save">
          {{ isSaving ? '保存中…' : '保存' }}
        </button>
        <span v-if="savedAt" class="saved">已保存 {{ savedAt.toLocaleTimeString('zh-CN') }}</span>

        <!-- Status controls exist only for a record that exists. Publishing
             something never created has no meaning. -->
        <template v-if="original !== null">
          <span class="spacer"></span>

          <button
            v-if="currentStatus !== 'published'"
            class="btn"
            type="button"
            :disabled="isTransitioning"
            @click="transition('publish')"
          >
            {{ isTransitioning ? '处理中…' : '发布' }}
          </button>
          <button
            v-if="currentStatus === 'published'"
            class="btn"
            type="button"
            :disabled="isTransitioning"
            @click="transition('unpublish')"
          >
            撤回为草稿
          </button>
          <button
            v-if="currentStatus !== 'archived'"
            class="btn"
            type="button"
            :disabled="isTransitioning"
            @click="transition('archive')"
          >
            归档
          </button>

          <template v-if="confirmingDelete">
            <span class="confirm-text">删除后前台不可达，且没有恢复接口。</span>
            <button class="btn btn--danger" type="button" :disabled="isDeleting" @click="handleDelete">
              {{ isDeleting ? '删除中…' : '确认删除' }}
            </button>
            <button class="btn" type="button" :disabled="isDeleting" @click="confirmingDelete = false">
              取消
            </button>
          </template>
          <button v-else class="btn btn--quiet" type="button" @click="confirmingDelete = true">
            删除
          </button>
        </template>
      </div>
    </template>
  </div>
</template>

<style scoped>
.page {
  max-width: 52rem;
}

.head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-4);
  margin-bottom: var(--space-6);
}

.title {
  margin-bottom: var(--space-2);
  font-size: 1.375rem;
}

.status-line {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: var(--space-3);
}

.badge {
  padding: 0.0625rem 0.375rem;
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-sm);
  color: var(--c-ink-muted);
  font-size: 0.6875rem;
  white-space: nowrap;
}

.badge--draft {
  border-color: var(--c-accent);
  color: var(--c-accent);
}

.badge--archived {
  color: var(--c-ink-faint);
}

.fields {
  margin-bottom: var(--space-5);
}

.field {
  margin-bottom: var(--space-5);
  min-width: 0;
}

.row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4);
}

.row > .field {
  flex: 1 1 12rem;
}
.label {
  display: block;
  margin-bottom: var(--space-1);
  font-size: 0.8125rem;
  font-weight: 500;
}

.input {
  width: 100%;
  padding: 0.375rem var(--space-3);
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-sm);
  background: var(--c-paper);
  color: var(--c-ink);
  font-family: inherit;
  font-size: 0.875rem;
}

.input:focus {
  border-color: var(--c-accent);
  outline: 2px solid transparent;
}

.input:disabled {
  background: var(--c-surface-sunken);
  color: var(--c-ink-faint);
}

/* The title is the one field that is also the page's subject, so it reads at
   heading size rather than as one more form row. */
.input--title {
  font-size: 1.125rem;
  font-weight: 500;
}

.input--mono {
  font-family: var(--font-mono);
}

.input--invalid {
  border-color: var(--c-danger);
}

.textarea {
  resize: vertical;
}

.hint {
  margin-top: var(--space-1);
  color: var(--c-ink-faint);
  font-size: 0.75rem;
}

.field-error {
  margin-top: var(--space-1);
  color: var(--c-danger);
  font-size: 0.75rem;
}

.fieldset {
  padding: 0;
  border: none;
}

.radio {
  display: flex;
  align-items: baseline;
  gap: var(--space-2);
  margin-top: var(--space-2);
  font-size: 0.875rem;
  cursor: pointer;
}

.radio-hint {
  color: var(--c-ink-faint);
  font-size: 0.75rem;
}
.alert {
  margin-bottom: var(--space-4);
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--c-danger);
  border-radius: var(--radius-sm);
  background: var(--c-danger-surface);
  color: var(--c-danger);
  font-size: 0.875rem;
}

.alert--soft {
  border-color: var(--c-line-strong);
  background: var(--c-surface-sunken);
  color: var(--c-ink-muted);
}

.actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-2);
  padding-top: var(--space-4);
  border-top: 1px solid var(--c-line);
}

/* Pushes what follows to the right edge, so destructive actions do not sit
   next to Save where a mis-aimed click lands on the wrong one. */
.spacer {
  margin-left: auto;
}

.saved {
  color: var(--c-ink-faint);
  font-size: 0.8125rem;
}

.confirm-text {
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.state {
  padding: var(--space-5);
  color: var(--c-ink-faint);
  font-size: 0.875rem;
}

.btn {
  padding: 0.375rem 0.875rem;
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--c-ink-muted);
  font-family: inherit;
  font-size: 0.875rem;
  cursor: pointer;
  transition:
    color 0.12s ease,
    border-color 0.12s ease,
    background-color 0.12s ease;
}

.btn:hover:not(:disabled) {
  border-color: var(--c-ink-muted);
  color: var(--c-ink);
  text-decoration: none;
}

.btn:disabled {
  color: var(--c-ink-faint);
  cursor: not-allowed;
}

.btn--primary {
  border-color: var(--c-accent);
  background: var(--c-accent);
  color: var(--c-paper);
}

.btn--primary:hover:not(:disabled) {
  border-color: var(--c-accent-hover);
  background: var(--c-accent-hover);
  color: var(--c-paper);
}

.btn--primary:disabled {
  border-color: var(--c-line-strong);
  background: var(--c-surface-sunken);
  color: var(--c-ink-faint);
}

.btn--danger {
  border-color: var(--c-danger);
  color: var(--c-danger);
}

.btn--danger:hover:not(:disabled) {
  background: var(--c-danger-surface);
  color: var(--c-danger);
}

.btn--quiet {
  border-color: transparent;
  color: var(--c-ink-faint);
}

.btn--quiet:hover:not(:disabled) {
  border-color: var(--c-danger);
  color: var(--c-danger);
}

@media (max-width: 40rem) {
  .head {
    flex-direction: column;
  }

  .row {
    flex-direction: column;
    gap: 0;
  }
}
</style>
