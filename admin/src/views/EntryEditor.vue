<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRouter } from 'vue-router'
import {
  categoriesApi,
  entriesApi,
  fromFormDateTime,
  toFormDateTime,
  toUserMessage,
  type EntryFormState,
} from '../api'
import { resolveAdminWorld } from '../content-worlds/registry'
import MarkdownEditor from '../components/MarkdownEditor.vue'
import WorkspaceHeader from '../components/writing/WorkspaceHeader.vue'
import ArticleSettings from '../components/writing/ArticleSettings.vue'
import PublishPanel from '../components/writing/PublishPanel.vue'
import TagPicker from '../components/writing/TagPicker.vue'
import { EntryRecoveryStore } from '../editor/recovery-store'
import {
  createSaveCoordinator,
  type SaveCoordinator,
  type SaveSnapshot,
} from '../editor/save-coordinator'
import { useEntryAutosave } from '../editor/useEntryAutosave'
import { getPublishChecks } from '../editor/publish-checks'
import { useWritingStore, writingFlushKey } from '../stores/writing'
import type {
  Category,
  EntryDetail,
  EntryPatchFields,
  EntryStatus,
  WorldKey,
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
  blank?: boolean
  world?: string
}>()

const router = useRouter()

const entryId = computed(() => (props.id === undefined ? null : Number(props.id)))
const draftWorld = computed<WorldKey>(() => resolveAdminWorld(props.world)?.key ?? 'journal')

/** The record as loaded, kept to diff against. Null while creating. */
const original = ref<EntryDetail | null>(null)
const categories = ref<Category[]>([])

const isLoading = ref(true)
const loadError = ref<string | null>(null)
const categoryError = ref<string | null>(null)
const saveError = ref<string | null>(null)
const fieldErrors = ref<Record<string, string>>({})
const editorSession = ref(0)

const isTransitioning = ref(false)
const isDeleting = ref(false)
const isRecovering = ref(false)
const settingsOpen = ref(false)
const publishOpen = ref(false)
let bypassRouteFlush = false
let isActive = false
let loadGeneration = 0
let recoveryDraft: { sourceEntryId: number; entryId: number; revision: number } | null = null

const recoveryStore = new EntryRecoveryStore()
const coordinatorBridge = createCoordinatorBridge()
const autosave = useEntryAutosave(coordinatorBridge)

const writing = useWritingStore()

function onTagsSaved(revision: number, tags: Array<{ id: number; name: string; slug: string; usage_count?: number }>): void {
  if (original.value) original.value = { ...original.value, revision, tags }
  autosave.revision.value = revision
}

/**
 * The flush gate the directory pulls before it navigates away.
 *
 * Registered here because this is the component that holds the coordinator. The
 * directory asks; it never reaches for the save queue itself, which would give
 * two components a claim on one set of pending fields.
 *
 * Null when the layout did not provide one -- the editor is also mountable on its
 * own in tests, and it must not require the shell to exist.
 */
const flushGate = inject(writingFlushKey, null)

/**
 * Kept as a stable identity so unmount can tell its own registration apart from a
 * successor's. Vue mounts the incoming editor before unmounting the outgoing one
 * on a route switch, so an unconditional clear would erase a live gate.
 */
const ownFlush = async (): Promise<void> => {
  await autosave.flush()
}

const form = ref<EntryFormState>({
  title: '',
  slug: '',
  summary: '',
  contentMd: '',
  coverUrl: '',
  visibility: 'public',
  categoryId: 0,
  happenedAt: '',
})

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
const editorEntry = computed<EntryDetail | null>(() => {
  if (original.value === null) return null
  return {
    ...original.value,
    title: form.value.title,
    slug: form.value.slug,
    summary: form.value.summary,
    content_md: form.value.contentMd,
    cover_url: form.value.coverUrl,
    visibility: form.value.visibility,
    category_id: form.value.categoryId,
    happened_at: form.value.happenedAt === '' ? null : fromFormDateTime(form.value.happenedAt),
  }
})
const publishChecks = computed(() =>
  editorEntry.value === null ? { blockers: [], reminders: [] } : getPublishChecks(editorEntry.value),
)
const controlsDisabled = computed(
  () => isLoading.value || isTransitioning.value || isRecovering.value || isDeleting.value,
)

// The status strings moved to WorkspaceHeader, which is now the only renderer of
// a save state. Two copies of that table would drift, and the drift would show up
// as the same state described two ways on one screen.

async function load(targetId: number | null = entryId.value): Promise<void> {
  if (props.blank) {
    resetToBlank()
    return
  }
  const generation = ++loadGeneration
  settingsOpen.value = false
  publishOpen.value = false
  isLoading.value = true
  loadError.value = null
  categoryError.value = null
  try {
    const entry =
      targetId === null
        ? await entriesApi.createEntry({ world: draftWorld.value })
        : await entriesApi.getEntry(targetId)
    if (!isCurrentLoad(generation)) return
    recoveryDraft = null

    if (targetId === null) {
      original.value = entry
      applyEntryToForm(entry)
      // Set before the replace, not after. The directory highlights whatever this
      // holds, and the blank draft is a real row the moment it exists -- waiting
      // for navigation would leave nothing current for the length of the round
      // trip, on the one path where the writer just asked for a new article.
      writing.setActiveEntry(entry.id)
      await router.replace({ name: 'entry-edit', params: { id: String(entry.id) } })
      if (!isCurrentLoad(generation)) return
    } else {
      original.value = entry
      applyEntryToForm(entry)
      writing.setActiveEntry(entry.id)
    }

    editorSession.value += 1
    await bindCoordinator(entry)
    if (!isCurrentLoad(generation)) return
    isLoading.value = false

    try {
      const cats = await categoriesApi.listCategoriesAdmin({
        world: entry.world ?? draftWorld.value,
      })
      if (isCurrentLoad(generation)) categories.value = cats
    } catch (error) {
      if (isCurrentLoad(generation)) categoryError.value = toUserMessage(error)
    }
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
  if (flushGate !== null) flushGate.value = ownFlush
  void load()
})

onBeforeUnmount(() => {
  isActive = false
  loadGeneration += 1
  window.removeEventListener('keydown', handleSaveShortcut)
  // Cleared, and only if it is still ours: a route switch mounts the next editor
  // before this one unmounts, so blindly nulling would drop the gate the
  // incoming editor just registered and let the following switch skip its flush.
  if (flushGate !== null && flushGate.value === ownFlush) flushGate.value = null
  // Nothing is open, so nothing in the directory should read as current.
  if (writing.activeEntryId === original.value?.id) writing.setActiveEntry(null)
})

watch(
  () => props.id,
  (nextId) => {
    const numericId = nextId === undefined ? null : Number(nextId)
    if (numericId !== null && (numericId !== original.value?.id || loadError.value !== null)) {
      void load(numericId)
    }
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
  if (controlsDisabled.value) return
  saveError.value = null
  fieldErrors.value = {}
  coordinatorBridge.update(fields)
}

async function flushBeforeAction(): Promise<boolean> {
  await autosave.flush()
  return autosave.status.value === 'saved'
}

async function flushBeforeRouteChange(): Promise<boolean> {
  return bypassRouteFlush || flushBeforeAction()
}

/**
 * The header's more-actions menu, dispatched by item id.
 *
 * The settings action opens the drawer. Destructive deletion is handled by the
 * header's dedicated two-step control, so it is no longer hidden in the menu.
 */
function handleHeaderAction(action: string): void {
  if (action === 'settings') settingsOpen.value = true
  else if (action === 'unpublish') void transition('unpublish')
  else if (action === 'archive') void transition('archive')
}

function handleSettingsUpdate(fields: EntryPatchFields): void {
  if ('title' in fields && fields.title !== undefined) form.value.title = fields.title
  if ('slug' in fields && fields.slug !== undefined) form.value.slug = fields.slug
  if ('summary' in fields && fields.summary !== undefined) form.value.summary = fields.summary
  if ('cover_url' in fields && fields.cover_url !== undefined) form.value.coverUrl = fields.cover_url
  if ('visibility' in fields && fields.visibility !== undefined) form.value.visibility = fields.visibility
  if ('category_id' in fields && fields.category_id !== undefined) form.value.categoryId = fields.category_id
  if ('happened_at' in fields && fields.happened_at !== undefined) {
    form.value.happenedAt = toFormDateTime(fields.happened_at)
  }
  queueUpdate(fields)
}

async function publishFromPanel(): Promise<void> {
  publishOpen.value = false
  await transition('publish')
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
    await autosave.flush()
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
    await autosave.flush()
    const recovery = await recoveryStore.get(conflictedId)
    if (recovery === null) throw new Error('Recovery record is unavailable')

    if (recoveryDraft === null || recoveryDraft.sourceEntryId !== conflictedId) {
      const created = await entriesApi.createEntry({
        world: original.value.world ?? draftWorld.value,
      })
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
    visibility: entry.visibility,
    categoryId: entry.category_id,
    happenedAt: toFormDateTime(entry.happened_at),
  }
}

function resetToBlank(): void {
  original.value = null
  form.value = {
    title: '',
    slug: '',
    summary: '',
    contentMd: '',
    coverUrl: '',
    visibility: 'public',
    categoryId: 0,
    happenedAt: '',
  }
  loadError.value = null
  saveError.value = null
  fieldErrors.value = {}
  settingsOpen.value = false
  publishOpen.value = false
  editorSession.value += 1
  writing.setActiveEntry(null)
  isLoading.value = false
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
    const nextEntryId = writing.nextEntryAfterDelete(id)
    await entriesApi.deleteEntry(id)
    writing.removeDirectoryEntry(id)
    await coordinatorBridge.dispose()
    bypassRouteFlush = true
    try {
      if (nextEntryId === null) {
        resetToBlank()
        await router.replace({ name: 'entry-blank' })
      } else {
        await router.replace({ name: 'entry-edit', params: { id: String(nextEntryId) } })
      }
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
  <!--
    The header spans the canvas; the page beneath it is centred and measured. They
    are siblings rather than nested for that reason -- a header inside the measured
    column would stop short of the canvas edges and read as a floating toolbar.
  -->
  <div class="canvas" :data-writing-blank="original === null ? 'true' : undefined">
    <p v-if="isLoading" class="state page">载入中…</p>
    <p v-else-if="loadError" class="alert page" role="alert">{{ loadError }}</p>

    <template v-else>
      <!--
        The header owns the save state and the primary actions; this view owns the
        conflict choices, because only it knows what the local and server versions
        hold. The status text itself lives in the header so there is exactly one
        place a save state is rendered.
      -->
      <WorkspaceHeader
        :save-status="autosave.status.value"
        :entry-status="currentStatus"
        :busy="controlsDisabled"
        @retry="flushBeforeAction"
          @publish="publishOpen = true"
          @delete="void handleDelete()"
        @action="handleHeaderAction"
      >
        <template #conflict>
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
      </WorkspaceHeader>

      <div class="page">
        <div class="fields">
          <!-- Mounted only once the content is known, and keyed by id: Milkdown
               reads its initial value once, so switching entries must build a
               new editor rather than try to swap the document underneath. -->
          <MarkdownEditor
            :key="`${original?.id ?? 'new'}:${editorSession}`"
            :initial-value="form.contentMd"
            :disabled="controlsDisabled"
            @update="handleContentUpdate"
          />
        </div>

        <p v-if="categoryError" class="alert" role="alert">{{ categoryError }}</p>
        <p v-if="localError && (form.title !== '' || form.slug !== '')" class="alert alert--soft">
          {{ localError }}
        </p>
        <p v-if="saveError" class="alert" role="alert">{{ saveError }}</p>

      </div>

      <ArticleSettings
        v-if="editorEntry"
        :open="settingsOpen"
        :entry="editorEntry"
        :categories="categories"
        :disabled="controlsDisabled"
        @update:open="settingsOpen = $event"
        @update="handleSettingsUpdate"
        @delete="handleDelete"
      />
      <TagPicker
        v-if="original"
        :entry-id="original.id"
        :revision="original.revision"
        :selected="original.tags ?? []"
        @saved="onTagsSaved"
      />
      <PublishPanel
        v-if="editorEntry"
        :open="publishOpen"
        :entry="editorEntry"
        :checks="publishChecks"
        :busy="controlsDisabled"
        @update:open="publishOpen = $event"
        @publish="publishFromPanel"
      />
    </template>
  </div>
</template>

<style scoped>
/**
 * The full width of the canvas column, so the header can reach both edges while
 * the prose below it stays measured.
 */
.canvas {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}

/**
 * Centred in the canvas rather than left-aligned in a content pane.
 *
 * The workspace grid zeroes the directory track on collapse, so `auto` margins are
 * what makes the canvas actually recentre -- a left-aligned page would just sit
 * further left with the same amount of empty space to its right, which is the
 * opposite of what collapsing the pane is for.
 *
 * The measure stays at 52rem here. Narrowing to the spec's 34-38 Chinese
 * characters is Task 5's work, together with the field removal that makes the
 * canvas mostly prose -- doing it now would set a body width around a page that is
 * still largely a form.
 */
.page {
  box-sizing: border-box;
  width: min(100%, 52rem);
  max-width: 52rem;
  margin-inline: auto;
  padding: var(--space-6) var(--space-5);
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

.input--mono {
  font-family: var(--font-mono);
}

.input--invalid {
  border-color: var(--c-danger);
}

.textarea {
  resize: vertical;
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
  .row {
    flex-direction: column;
    gap: 0;
  }
}
</style>
