<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRouter } from 'vue-router'
import {
  categoriesApi,
  entriesApi,
  fromFormDateTime,
  toFormDateTime,
  toUserMessage,
  type EntryFormState,
} from '../api'
import MarkdownEditor from '../components/MarkdownEditor.vue'
import { EntryRecoveryStore } from '../editor/recovery-store'
import {
  createSaveCoordinator,
  type SaveCoordinator,
  type SaveSnapshot,
  type SaveStatus,
} from '../editor/save-coordinator'
import { useEntryAutosave } from '../editor/useEntryAutosave'
import type {
  Category,
  EntryDetail,
  EntryPatchFields,
  EntryStatus,
  EntryType,
  EntryVisibility,
} from '../types/api'

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

const entryId = computed(() => (props.id === undefined ? null : Number(props.id)))

/** The record as loaded, kept to diff against. Null while creating. */
const original = ref<EntryDetail | null>(null)
const categories = ref<Category[]>([])

const isLoading = ref(true)
const loadError = ref<string | null>(null)
const saveError = ref<string | null>(null)
const fieldErrors = ref<Record<string, string>>({})
const editorSession = ref(0)

const isTransitioning = ref(false)
const isDeleting = ref(false)
const confirmingDelete = ref(false)
const isRecovering = ref(false)
let bypassRouteFlush = false
let isActive = false
let loadGeneration = 0
let recoveryDraft: { sourceEntryId: number; entryId: number; revision: number } | null = null

const recoveryStore = new EntryRecoveryStore()
const coordinatorBridge = createCoordinatorBridge()
const autosave = useEntryAutosave(coordinatorBridge)

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
  return null
})

const currentStatus = computed<EntryStatus | null>(() => original.value?.status ?? null)
const controlsDisabled = computed(() => isTransitioning.value || isRecovering.value)

const saveStatusText = computed(() => {
  const labels: Record<SaveStatus, string> = {
    saved: '已保存',
    pending: '待保存',
    saving: '保存中…',
    offline: '离线，本地草稿已保留',
    error: '保存失败',
    conflict: '保存冲突，本地已保留',
  }
  return labels[autosave.status.value]
})

async function load(targetId: number | null = entryId.value): Promise<void> {
  const generation = ++loadGeneration
  isLoading.value = true
  loadError.value = null
  try {
    const [cats, entry] = await Promise.all([
      categoriesApi.listCategoriesAdmin(),
      targetId === null ? entriesApi.createEntry({}) : entriesApi.getEntry(targetId),
    ])
    if (!isCurrentLoad(generation)) return
    recoveryDraft = null

    if (targetId === null) {
      original.value = entry
      applyEntryToForm(entry)
      await router.replace({ name: 'entry-edit', params: { id: String(entry.id) } })
      if (!isCurrentLoad(generation)) return
    } else {
      original.value = entry
      applyEntryToForm(entry)
    }

    categories.value = cats
    editorSession.value += 1
    await bindCoordinator(entry)
  } catch (error) {
    if (isCurrentLoad(generation)) loadError.value = toUserMessage(error)
  } finally {
    if (isCurrentLoad(generation)) isLoading.value = false
  }
}

function isCurrentLoad(generation: number): boolean {
  return isActive && generation === loadGeneration
}

onMounted(() => {
  isActive = true
  window.addEventListener('keydown', handleSaveShortcut)
  void load()
})

onBeforeUnmount(() => {
  isActive = false
  loadGeneration += 1
  window.removeEventListener('keydown', handleSaveShortcut)
})

watch(
  () => props.id,
  (nextId) => {
    const numericId = nextId === undefined ? null : Number(nextId)
    if (numericId !== null && numericId !== original.value?.id) void load(numericId)
  },
)

watch(
  autosave.error,
  (error) => {
    fieldErrors.value = error?.fields ?? {}
  },
  { flush: 'sync' },
)

onBeforeRouteLeave(async () => ((await flushBeforeRouteChange()) ? undefined : false))
onBeforeRouteUpdate(async () => ((await flushBeforeRouteChange()) ? undefined : false))

function applyError(error: unknown): void {
  saveError.value = toUserMessage(error)
  fieldErrors.value = (error as { fields?: Record<string, string> }).fields ?? {}
}

/** Markdown arrives from the editor one way; this is the only writer of contentMd. */
function handleContentUpdate(markdown: string): void {
  form.value.contentMd = markdown
  queueUpdate({ content_md: markdown })
}

function queueUpdate(fields: EntryPatchFields): void {
  saveError.value = null
  fieldErrors.value = {}
  coordinatorBridge.update(fields)
}

function queueHappenedAt(): void {
  queueUpdate({
    happened_at:
      form.value.happenedAt === ''
        ? '0001-01-01T00:00:00Z'
        : fromFormDateTime(form.value.happenedAt),
  })
}

async function flushBeforeAction(): Promise<boolean> {
  await autosave.flush()
  return autosave.status.value === 'saved'
}

async function flushBeforeRouteChange(): Promise<boolean> {
  return bypassRouteFlush || flushBeforeAction()
}

async function handleSaveShortcut(event: KeyboardEvent): Promise<void> {
  if (event.key.toLowerCase() !== 's' || (!event.metaKey && !event.ctrlKey)) return
  event.preventDefault()
  await flushBeforeAction()
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
    if (!(await flushBeforeAction())) return
    const id = original.value.id
    const revision = autosave.revision.value
    const transitioned =
      action === 'publish'
        ? await entriesApi.publishEntry(id, revision)
        : action === 'unpublish'
          ? await entriesApi.unpublishEntry(id, revision)
          : await entriesApi.archiveEntry(id, revision)
    original.value = transitioned
    await bindCoordinator(transitioned)
  } catch (error) {
    applyError(error)
  } finally {
    isTransitioning.value = false
  }
}

async function reloadServerVersion(): Promise<void> {
  if (original.value === null || isRecovering.value) return
  isRecovering.value = true
  saveError.value = null
  try {
    const entry = await entriesApi.getEntry(original.value.id)
    await recoveryStore.remove(original.value.id)
    original.value = entry
    applyEntryToForm(entry)
    editorSession.value += 1
    recoveryDraft = null
    await bindCoordinator(entry)
  } catch (error) {
    applyError(error)
  } finally {
    isRecovering.value = false
  }
}

async function overwriteServerWithLocal(): Promise<void> {
  if (original.value === null || isRecovering.value) return
  isRecovering.value = true
  saveError.value = null
  const conflictedId = original.value.id
  try {
    const serverEntry = await entriesApi.getEntry(conflictedId)
    const recovery = await recoveryStore.get(conflictedId)
    if (recovery === null) throw new Error('Recovery record is unavailable')

    const overwritten = await entriesApi.updateEntry(conflictedId, {
      revision: serverEntry.revision,
      ...recovery.fields,
    })
    await recoveryStore.remove(conflictedId)
    recoveryDraft = null
    original.value = overwritten
    await bindCoordinator(overwritten)
  } catch (error) {
    applyError(error)
  } finally {
    isRecovering.value = false
  }
}

async function recoverAsDraft(): Promise<void> {
  if (original.value === null || isRecovering.value) return
  isRecovering.value = true
  saveError.value = null
  const conflictedId = original.value.id
  try {
    const recovery = await recoveryStore.get(conflictedId)
    if (recovery === null) throw new Error('Recovery record is unavailable')

    if (recoveryDraft === null || recoveryDraft.sourceEntryId !== conflictedId) {
      const created = await entriesApi.createEntry({})
      recoveryDraft = {
        sourceEntryId: conflictedId,
        entryId: created.id,
        revision: created.revision,
      }
    }
    const recovered = await entriesApi.updateEntry(recoveryDraft.entryId, {
      revision: recoveryDraft.revision,
      ...recovery.fields,
    })
    bypassRouteFlush = true
    try {
      await router.replace({ name: 'entry-edit', params: { id: String(recovered.id) } })
    } finally {
      bypassRouteFlush = false
    }
    await recoveryStore.remove(conflictedId)
    recoveryDraft = null
    original.value = recovered
    applyEntryToForm(recovered)
    editorSession.value += 1
    await bindCoordinator(recovered)
  } catch (error) {
    applyError(error)
  } finally {
    isRecovering.value = false
  }
}

function applyEntryToForm(entry: EntryDetail): void {
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

async function bindCoordinator(entry: EntryDetail): Promise<void> {
  const coordinator = createSaveCoordinator({
    entryId: entry.id,
    initialRevision: entry.revision,
    waitMs: 1000,
    recoveryStore,
    save: async (id, body) => {
      const saved = await entriesApi.updateEntry(id, body)
      if (original.value?.id === id) original.value = saved
      return saved
    },
  })
  await coordinatorBridge.replace(coordinator)
}

async function handleDelete(): Promise<void> {
  if (original.value === null || isDeleting.value) return
  isDeleting.value = true
  try {
    if (!(await flushBeforeAction())) return
    const id = original.value.id
    await entriesApi.deleteEntry(id)
    await coordinatorBridge.dispose()
    bypassRouteFlush = true
    try {
      await router.replace({ name: 'entries' })
    } finally {
      bypassRouteFlush = false
    }
  } catch (error) {
    applyError(error)
  } finally {
    isDeleting.value = false
  }
}

interface CoordinatorBridge extends SaveCoordinator {
  replace(coordinator: SaveCoordinator): Promise<void>
}

function createCoordinatorBridge(): CoordinatorBridge {
  let current: SaveCoordinator | null = null
  let listener: ((snapshot: SaveSnapshot) => void) | null = null
  let unsubscribe: (() => void) | null = null
  let disposed = false
  let replacement: Promise<void> = Promise.resolve()

  return {
    update(fields) {
      current?.update(fields)
    },
    flush: () => current?.flush() ?? Promise.resolve(null),
    retry: () => current?.retry() ?? Promise.resolve(null),
    subscribe(nextListener) {
      if (disposed) return () => undefined
      listener = nextListener
      unsubscribe = current?.subscribe(nextListener) ?? null
      return () => {
        unsubscribe?.()
        unsubscribe = null
        listener = null
      }
    },
    replace(coordinator) {
      replacement = replacement.then(async () => {
        if (disposed) {
          await coordinator.dispose()
          return
        }
        unsubscribe?.()
        unsubscribe = null
        const previous = current
        current = null
        await previous?.dispose()
        if (disposed) {
          await coordinator.dispose()
          return
        }
        current = coordinator
        if (listener !== null) unsubscribe = current.subscribe(listener)
      })
      return replacement
    },
    async dispose() {
      if (disposed) return
      disposed = true
      unsubscribe?.()
      unsubscribe = null
      listener = null
      await replacement
      await current?.dispose()
      current = null
    },
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
          <h1 class="title">编辑内容</h1>
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
            :disabled="controlsDisabled"
            @input="queueUpdate({ title: form.title })"
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
              :disabled="controlsDisabled"
              @input="queueUpdate({ slug: form.slug })"
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
              :disabled="controlsDisabled"
              @change="queueUpdate({ type: form.type })"
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
              :disabled="controlsDisabled"
              @change="queueUpdate({ category_id: form.categoryId })"
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
            :disabled="controlsDisabled"
            @input="queueUpdate({ summary: form.summary })"
          ></textarea>
          <p class="hint">列表和 meta description 用。留空即清除。</p>
        </div>

        <div class="field">
          <label class="label">正文</label>
          <!-- Mounted only once the content is known, and keyed by id: Milkdown
               reads its initial value once, so switching entries must build a
               new editor rather than try to swap the document underneath. -->
          <MarkdownEditor
            :key="`${original?.id ?? 'new'}:${editorSession}`"
            :initial-value="form.contentMd"
            :disabled="controlsDisabled"
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
              :disabled="controlsDisabled"
              @input="queueUpdate({ cover_url: form.coverUrl })"
            />
          </div>

          <div class="field">
            <label class="label" for="e-happened">发生时间</label>
            <input
              id="e-happened"
              v-model="form.happenedAt"
              class="input"
              type="datetime-local"
              :disabled="controlsDisabled"
              @input="queueHappenedAt"
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
              :disabled="controlsDisabled"
              @change="queueUpdate({ visibility: form.visibility })"
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
        <div
          class="save-status"
          :class="`save-status--${autosave.status.value}`"
          data-save-status
          aria-live="polite"
        >
          <span>{{ saveStatusText }}</span>
          <button
            v-if="autosave.status.value === 'offline' || autosave.status.value === 'error'"
            class="status-action"
            type="button"
            data-save-retry
            @click="flushBeforeAction"
          >
            重试
          </button>
          <template v-else-if="autosave.status.value === 'conflict'">
            <button
              class="status-action"
              type="button"
              data-conflict-action
              :disabled="isRecovering"
              @click="reloadServerVersion"
            >
              载入服务端
            </button>
            <button
              class="status-action"
              type="button"
              data-conflict-action
              :disabled="isRecovering"
              @click="overwriteServerWithLocal"
            >
              覆盖服务端
            </button>
            <button
              class="status-action"
              type="button"
              data-conflict-action
              :disabled="isRecovering"
              @click="recoverAsDraft"
            >
              另存为恢复草稿
            </button>
          </template>
        </div>

        <!-- Status controls exist only for a record that exists. Publishing
             something never created has no meaning. -->
        <template v-if="original !== null">
          <span class="spacer"></span>

          <button
            v-if="currentStatus !== 'published'"
            class="btn"
            type="button"
            :disabled="controlsDisabled"
            @click="transition('publish')"
          >
            {{ isTransitioning ? '处理中…' : '发布' }}
          </button>
          <button
            v-if="currentStatus === 'published'"
            class="btn"
            type="button"
            :disabled="controlsDisabled"
            @click="transition('unpublish')"
          >
            撤回为草稿
          </button>
          <button
            v-if="currentStatus !== 'archived'"
            class="btn"
            type="button"
            :disabled="controlsDisabled"
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

/* Pushes transitions away from the quiet save indicator. */
.spacer {
  margin-left: auto;
}

.save-status {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  height: 2rem;
  color: var(--c-ink-faint);
  font-size: 0.8125rem;
  white-space: nowrap;
}

.save-status--pending,
.save-status--saving {
  color: var(--c-ink-muted);
}

.save-status--offline,
.save-status--error,
.save-status--conflict {
  color: var(--c-danger);
}

.status-action {
  padding: 0.1875rem 0.5rem;
  border: 1px solid currentColor;
  border-radius: var(--radius-sm);
  background: transparent;
  color: inherit;
  font-family: inherit;
  font-size: 0.8125rem;
  cursor: pointer;
}

.status-action:disabled {
  opacity: 0.55;
  cursor: not-allowed;
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

  .save-status {
    width: 100%;
    max-width: 100%;
    height: 3.5rem;
    flex-wrap: wrap;
    align-content: center;
    gap: var(--space-1);
    white-space: normal;
  }

  .save-status > span,
  .status-action {
    white-space: nowrap;
  }

  .status-action {
    padding-inline: 0.375rem;
  }
}
</style>
