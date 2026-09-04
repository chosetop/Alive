<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { isNavigationFailure, useRouter } from 'vue-router'

import { entriesApi, toUserMessage } from '../../api'
import { resolveAdminWorld } from '../../content-worlds/registry'
import { EntryRecoveryStore } from '../../editor/recovery-store'
import { useWritingStore, writingFlushKey } from '../../stores/writing'
import type { EntryListItem, EntryStatus, WorldKey } from '../../types/api'
import { UiButton, UiIcon, UiIconButton } from '../ui'

/**
 * The article directory.
 *
 * This lists articles, not the headings inside the current one. That is the
 * distinction the spec draws against an outline pane: the thing a writer needs
 * beside the canvas is the way back to everything else they have written, and an
 * outline of a document already visible on screen is not that.
 *
 * Recent articles load once on mount. The status groups load only when opened,
 * because three extra list requests at mount would delay the first paint of the
 * pane a writer is about to read, to populate sections most sessions never
 * expand.
 */

const RECENT_PAGE_SIZE = 20

/**
 * Long enough that typing a two-character Chinese word issues one request rather
 * than two, short enough that the list still feels attached to the keyboard.
 * Every keystroke firing would put the directory's own requests in a queue with
 * the autosave PATCHes it must not delay.
 */
const SEARCH_DEBOUNCE_MS = 250

const STATUS_GROUPS: ReadonlyArray<{ status: EntryStatus; label: string }> = [
  { status: 'draft', label: '草稿' },
  { status: 'published', label: '已发布' },
  { status: 'archived', label: '已归档' },
]

const STATUS_LABEL: Record<EntryStatus, string> = {
  draft: '草稿',
  published: '已发布',
  archived: '已归档',
}

/**
 * A draft created by the new-article button has no title yet, and an empty row
 * is unclickable in practice. The fallback is shown, never saved: writing it into
 * the record would make "无标题草稿" a real title somebody has to delete.
 */
const UNTITLED = '无标题草稿'

const props = withDefaults(
  defineProps<{
    world: WorldKey
    /**
     * True below the drawer breakpoint. Drives the two behaviours that differ
     * there: the pane closes after a selection, and the collapse control is
     * labelled as closing a drawer rather than hiding a column.
     */
    drawer?: boolean
  }>(),
  { drawer: false },
)

const router = useRouter()
const writing = useWritingStore()
const recoveryStore = new EntryRecoveryStore()
const worldDefinition = computed(() => resolveAdminWorld(props.world))
const directoryLabel = computed(() => `${worldDefinition.value?.directoryNoun ?? '文章'}目录`)
const searchLabel = computed(() => worldDefinition.value?.searchPlaceholder ?? '搜索文章')
const createLabel = computed(() => worldDefinition.value?.createLabel ?? '新建文章')
const collapseLabel = computed(() => `${props.drawer ? '关闭' : '收起'}${directoryLabel.value}`)

const recent = ref<EntryListItem[]>([])
const searchResults = ref<EntryListItem[]>([])
const groups = ref<Partial<Record<EntryStatus, EntryListItem[]>>>({})
const openGroups = ref<Set<EntryStatus>>(new Set())
const loadingGroups = ref<Set<EntryStatus>>(new Set())
const unsyncedIds = ref<Set<number>>(new Set())

const isLoadingRecent = ref(true)
const isSearching = ref(false)
const isCreating = ref(false)
const isDeleting = ref(false)
const isManagingRecent = ref(false)
const selectedRecentIds = ref<Set<number>>(new Set())
const confirmingBulkDelete = ref(false)
const error = ref<string | null>(null)

let searchTimer: ReturnType<typeof setTimeout> | null = null
let searchGeneration = 0
let disposed = false

const trimmedQuery = computed(() => writing.searchQuery.trim())
const isSearchMode = computed(() => trimmedQuery.value !== '')
const visibleRecent = computed(() =>
  writing.directoryEntries,
)
const selectedRecentCount = computed(() => selectedRecentIds.value.size)

onMounted(() => {
  void loadRecent()
  void loadUnsynced()
})

onBeforeUnmount(() => {
  disposed = true
  if (searchTimer !== null) clearTimeout(searchTimer)
  searchTimer = null
})

/**
 * A whitespace-only query is treated as no query at all, matching what the
 * backend does with `q`. Without this the pane would clear itself and show an
 * empty result set for a stray space.
 */
watch(trimmedQuery, (query) => {
  if (searchTimer !== null) clearTimeout(searchTimer)

  if (query === '') {
    // Cancel any in-flight search too: its response would otherwise land after
    // the box was cleared and repopulate a list the writer just dismissed.
    searchGeneration += 1
    searchResults.value = []
    isSearching.value = false
    return
  }

  isSearching.value = true
  searchTimer = setTimeout(() => {
    searchTimer = null
    void runSearch(query)
  }, SEARCH_DEBOUNCE_MS)
})

async function loadRecent(): Promise<void> {
  isLoadingRecent.value = true
  try {
    const page = await entriesApi.listEntriesAdmin({ world: props.world, page_size: RECENT_PAGE_SIZE })
    if (disposed) return
    recent.value = page.data
    writing.setDirectoryEntries(page.data)
    error.value = null
  } catch (loadFailure) {
    if (!disposed) error.value = toUserMessage(loadFailure)
  } finally {
    if (!disposed) isLoadingRecent.value = false
  }
}

/**
 * Unsynced markers come from the recovery store, not from the list response. The
 * server cannot know an edit exists that never reached it, which is exactly the
 * case the marker is for.
 */
async function loadUnsynced(): Promise<void> {
  try {
    const records = await recoveryStore.list()
    if (!disposed) unsyncedIds.value = new Set(records.map((record) => record.entryId))
  } catch {
    // A directory that refuses to render because IndexedDB is unavailable is
    // worse than one missing dots: the pane is how you reach your work.
  }
}

async function runSearch(query: string): Promise<void> {
  const generation = ++searchGeneration
  try {
    const page = await entriesApi.listEntriesAdmin({ world: props.world, q: query, page_size: RECENT_PAGE_SIZE })
    if (disposed || generation !== searchGeneration) return
    searchResults.value = page.data
    error.value = null
  } catch (searchFailure) {
    if (!disposed && generation === searchGeneration) error.value = toUserMessage(searchFailure)
  } finally {
    if (!disposed && generation === searchGeneration) isSearching.value = false
  }
}

async function toggleGroup(status: EntryStatus): Promise<void> {
  const open = new Set(openGroups.value)
  if (open.has(status)) {
    open.delete(status)
    openGroups.value = open
    return
  }
  open.add(status)
  openGroups.value = open

  // Fetched once per session. Reopening a group a second time should not spend a
  // request to redraw rows already on hand.
  if (groups.value[status] !== undefined) return

  loadingGroups.value = new Set(loadingGroups.value).add(status)
  try {
    const page = await entriesApi.listEntriesAdmin({ world: props.world, status, page_size: RECENT_PAGE_SIZE })
    if (!disposed) groups.value = { ...groups.value, [status]: page.data }
  } catch (groupFailure) {
    if (!disposed) error.value = toUserMessage(groupFailure)
  } finally {
    if (!disposed) {
      const loading = new Set(loadingGroups.value)
      loading.delete(status)
      loadingGroups.value = loading
    }
  }
}

/**
 * Switching articles.
 *
 * The pending save is flushed before navigating, not after. The editor's own
 * `onBeforeRouteUpdate` guard also flushes and can veto the switch, but waiting
 * for the guard alone would mean the click is dispatched while a debounce timer
 * is still holding the last keystroke. Awaiting here first keeps the ordering
 * observable: everything typed is on its way to the server before the canvas
 * changes underneath it.
 */
async function openEntry(entryId: number): Promise<void> {
  if (entryId === writing.activeEntryId) {
    // Already here. Still close the drawer, since on mobile the canvas is behind
    // it and tapping the current row is a plausible way to ask for the canvas.
    if (props.drawer) writing.setDirectoryOpen(false)
    return
  }

  try {
    await flushActiveEntry()
    const navigationResult = await router.push({ name: 'entry-edit', params: { id: String(entryId) } })
    if (navigationResult) throw navigationResult
    if (props.drawer) writing.setDirectoryOpen(false)
  } catch {
    // A route guard can veto a switch when the editor is offline, errored, or in
    // conflict. The rejected navigation is not user-visible by itself, so keep
    // the current article and explain why the tap did nothing.
    if (!disposed) error.value = '无法切换文章，当前编辑状态未改变。'
  }
}

async function createArticle(): Promise<void> {
  if (isCreating.value) return
  isCreating.value = true
  error.value = null
  try {
    await flushActiveEntry()
    if (worldDefinition.value?.editorRouteName === null || worldDefinition.value === null) {
      throw new Error('missing world route')
    }
    const navigationResult = await router.push({
      name: worldDefinition.value.editorRouteName,
      params: { world: props.world },
      query: props.world === 'saying' ? { new: String(Date.now()) } : undefined,
    })
    if (isNavigationFailure(navigationResult)) throw navigationResult
    if (props.drawer) writing.setDirectoryOpen(false)
  } catch (createFailure) {
    if (!disposed) error.value = '无法打开新的工作台，当前编辑状态未改变。'
  } finally {
    if (!disposed) isCreating.value = false
  }
}

function toggleRecentManagement(): void {
  isManagingRecent.value = !isManagingRecent.value
  selectedRecentIds.value = new Set()
  confirmingBulkDelete.value = false
}

function toggleRecentSelection(entryId: number): void {
  if (!isManagingRecent.value) return
  const selected = new Set(selectedRecentIds.value)
  if (selected.has(entryId)) selected.delete(entryId)
  else selected.add(entryId)
  selectedRecentIds.value = selected
}

function nextEntryAfterBulkDelete(entryIds: Set<number>, activeEntryId: number): number | null {
  const entries = writing.directoryEntries
  const index = entries.findIndex((entry) => entry.id === activeEntryId)
  if (index === -1) return null
  return entries.slice(index + 1).find((entry) => !entryIds.has(entry.id))?.id
    ?? entries.slice(0, index).reverse().find((entry) => !entryIds.has(entry.id))?.id
    ?? null
}

function requestBulkDelete(): void {
  if (selectedRecentCount.value > 0) confirmingBulkDelete.value = true
}

async function deleteSelectedRecent(): Promise<void> {
  if (isDeleting.value || selectedRecentCount.value === 0) return
  isDeleting.value = true
  error.value = null
  const selectedIds = new Set(selectedRecentIds.value)
  const activeEntryId = writing.activeEntryId
  const nextEntryId = activeEntryId !== null && selectedIds.has(activeEntryId)
    ? nextEntryAfterBulkDelete(selectedIds, activeEntryId)
    : null
  try {
    await flushActiveEntry()
    for (const entryId of selectedIds) await entriesApi.deleteEntry(entryId)

    recent.value = recent.value.filter((item) => !selectedIds.has(item.id))
    writing.directoryEntries = writing.directoryEntries.filter((item) => !selectedIds.has(item.id))
    selectedRecentIds.value = new Set()
    confirmingBulkDelete.value = false

    if (activeEntryId !== null && selectedIds.has(activeEntryId)) {
      writing.setActiveEntry(null)
      if (nextEntryId === null) {
        await router.replace({ name: props.world === 'saying' ? 'saying-editor-new' : 'entry-new-world', params: { world: props.world }, query: props.world === 'saying' ? { new: String(Date.now()) } : undefined })
      } else {
        await router.replace({ name: 'entry-edit', params: { id: String(nextEntryId) } })
      }
    }
  } catch (deleteFailure) {
    error.value = toUserMessage(deleteFailure)
  } finally {
    isDeleting.value = false
  }
}

/**
 * Flushing the open article before leaving it.
 *
 * The directory does not own a coordinator and must not construct one: the save
 * queue belongs to whoever holds the open document. It asks, through the gate
 * the editor registers.
 *
 * The editor's `onBeforeRouteUpdate` guard also flushes and can veto the switch,
 * so this is not the only barrier -- but the guard runs after the navigation
 * begins, and a click landing while a debounce timer still holds the last
 * keystroke should not depend on guard ordering to be safe. Flushing here makes
 * the sequence explicit: what was typed is on its way before the canvas changes.
 */
const flushGate = inject(writingFlushKey, null)

async function flushActiveEntry(): Promise<void> {
  // Null when no editor is mounted -- the drawer over an empty canvas. Nothing
  // to flush, and awaiting a gate nobody registered would hang the click.
  if (flushGate === null || flushGate.value === null) return
  await flushGate.value()
}

function titleOf(item: EntryListItem): string {
  if (props.world === 'saying') {
    const excerpt = (item.content_md ?? '').replace(/[#*_>`]/g, '').trim()
    if (excerpt !== '') return excerpt
  }
  return item.title.trim() === '' ? UNTITLED : item.title
}

/** Editing time, not publication time: the directory is ordered by recent work. */
function editedAt(item: EntryListItem): string {
  return new Date(item.updated_at).toLocaleString('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function isUnsynced(item: EntryListItem): boolean {
  return unsyncedIds.value.has(item.id)
}
</script>

<template>
  <!-- nav, not aside: this is how you move between articles, and the landmark is
       how a screen reader user reaches it without walking the canvas. -->
  <nav
    class="directory"
    :aria-label="directoryLabel"
    data-article-directory
    data-surface="glass"
    :data-writing-drawer="drawer ? 'true' : undefined"
  >
    <div class="brand">
      <RouterLink class="wordmark" to="/dashboard" aria-label="返回 Dashboard" data-directory-brand>Alive</RouterLink>
      <UiIconButton
        :label="collapseLabel"
        data-directory-collapse
        @click="writing.setDirectoryOpen(false)"
      >
        <UiIcon name="chevron-left" />
      </UiIconButton>
    </div>

    <UiButton
      variant="secondary"
      :loading="isCreating"
      data-directory-new
      @click="createArticle"
    >
      {{ isCreating ? '打开中…' : createLabel }}
    </UiButton>

    <div class="search">
      <label class="search-label ui-visually-hidden" for="directory-search">{{ searchLabel }}</label>
      <div class="search-control">
        <input
          id="directory-search"
          class="search-input"
          type="search"
          autocomplete="off"
          :placeholder="searchLabel"
          data-directory-search
          :value="writing.searchQuery"
          @input="writing.setSearchQuery(($event.target as HTMLInputElement).value)"
        />
      </div>
    </div>

    <p v-if="error" class="error" role="alert">{{ error }}</p>

    <!-- Search replaces the recent list rather than filtering it in place: the
         results come from the server and span every status, so presenting them
         under a "最近" heading would misdescribe them. -->
    <section v-if="isSearchMode" class="group" aria-labelledby="directory-search-heading">
      <h2 id="directory-search-heading" class="group-title">搜索结果</h2>
      <p v-if="isSearching" class="state">搜索中…</p>
      <p v-else-if="searchResults.length === 0" class="state">没有匹配的文章</p>
      <ul v-else class="list" data-directory-results>
        <li v-for="item in searchResults" :key="item.id">
          <button
            class="item"
            type="button"
            :data-entry-id="item.id"
            :aria-current="item.id === writing.activeEntryId ? 'true' : undefined"
            @click="openEntry(item.id)"
          >
            <span
              v-if="item.id === writing.activeEntryId"
              class="item-cursor"
              data-alive-cursor
              aria-hidden="true"
            />
            <span :class="['item-title', { 'item-title--excerpt': world === 'saying' }]" :data-saying-excerpt="world === 'saying' ? true : undefined">{{ titleOf(item) }}</span>
            <span class="item-meta">
              <span>{{ STATUS_LABEL[item.status] }}</span>
              <span>{{ editedAt(item) }}</span>
              <span v-if="isUnsynced(item)" class="dot" data-unsynced>
                <!-- The dot alone would be colour-and-shape only, which fails the
                     spec's rule that state must be perceivable as text. -->
                <span class="ui-visually-hidden">未同步</span>
              </span>
            </span>
          </button>
        </li>
      </ul>
    </section>

    <template v-else>
      <section class="group" aria-labelledby="directory-recent-heading">
        <div class="group-heading">
          <h2 id="directory-recent-heading" class="group-title">最近</h2>
          <div class="recent-actions">
            <template v-if="confirmingBulkDelete">
              <button type="button" class="manage-action manage-action--danger" data-recent-delete-confirm @click="deleteSelectedRecent">
                确认删除 {{ selectedRecentCount }} 条
              </button>
              <button type="button" class="manage-action" @click="confirmingBulkDelete = false">取消</button>
            </template>
            <template v-else>
              <button type="button" class="manage-action" data-recent-manage @click="toggleRecentManagement">
                {{ isManagingRecent ? '完成' : '管理' }}
              </button>
              <button
                v-if="isManagingRecent && selectedRecentCount > 0"
                type="button"
                class="manage-action manage-action--danger"
                data-recent-delete
                @click="requestBulkDelete"
              >删除</button>
            </template>
          </div>
        </div>
        <p v-if="isLoadingRecent" class="state">载入中…</p>
        <p v-else-if="visibleRecent.length === 0" class="state">还没有文章</p>
        <ul v-else class="list" data-directory-recent>
          <li v-for="item in visibleRecent" :key="item.id">
            <input
              v-if="isManagingRecent"
              class="recent-checkbox"
              type="checkbox"
              :checked="selectedRecentIds.has(item.id)"
              :data-recent-select="item.id"
              :aria-label="`选择${titleOf(item)}`"
              @change="toggleRecentSelection(item.id)"
            />
            <button
              :class="['item', { 'item--selectable': isManagingRecent }]"
              type="button"
              :data-entry-id="item.id"
              :aria-current="item.id === writing.activeEntryId ? 'true' : undefined"
              @click="isManagingRecent ? toggleRecentSelection(item.id) : openEntry(item.id)"
            >
              <span
                v-if="item.id === writing.activeEntryId"
                class="item-cursor"
                data-alive-cursor
                aria-hidden="true"
              />
                <span :class="['item-title', { 'item-title--excerpt': world === 'saying' }]" :data-saying-excerpt="world === 'saying' ? true : undefined">{{ titleOf(item) }}</span>
                <span class="item-meta">
                <span>{{ STATUS_LABEL[item.status] }}</span>
                <span>{{ editedAt(item) }}</span>
                <span v-if="isUnsynced(item)" class="dot" data-unsynced>
                  <span class="ui-visually-hidden">未同步</span>
                </span>
                </span>
              </button>
          </li>
        </ul>
      </section>

      <section class="group" aria-labelledby="directory-library-heading">
        <h2 id="directory-library-heading" class="group-title">文章库</h2>
        <div v-for="group in STATUS_GROUPS" :key="group.status" class="status-group">
          <!-- aria-expanded, not a rotated chevron alone: the open state has to
               be announced, and the group's own request depends on it. -->
          <button
            class="group-toggle"
            type="button"
            :data-status-group="group.status"
            :aria-expanded="openGroups.has(group.status) ? 'true' : 'false'"
            @click="toggleGroup(group.status)"
          >
            {{ group.label }}
          </button>
          <template v-if="openGroups.has(group.status)">
            <p v-if="loadingGroups.has(group.status)" class="state">载入中…</p>
            <p v-else-if="(groups[group.status] ?? []).length === 0" class="state">这里还是空的</p>
            <ul v-else class="list" :data-status-list="group.status">
              <li v-for="item in groups[group.status]" :key="item.id">
                <button
                  class="item"
                  type="button"
                  :data-entry-id="item.id"
                  :aria-current="item.id === writing.activeEntryId ? 'true' : undefined"
                  @click="openEntry(item.id)"
                >
                  <span
                    v-if="item.id === writing.activeEntryId"
                    class="item-cursor"
                    data-alive-cursor
                    aria-hidden="true"
                  />
                  <span :class="['item-title', { 'item-title--excerpt': world === 'saying' }]" :data-saying-excerpt="world === 'saying' ? true : undefined">{{ titleOf(item) }}</span>
                  <span class="item-meta">
                    <span>{{ editedAt(item) }}</span>
                    <span v-if="isUnsynced(item)" class="dot" data-unsynced>
                      <span class="ui-visually-hidden">未同步</span>
                    </span>
                  </span>
                </button>
              </li>
            </ul>
          </template>
        </div>
      </section>
    </template>

    <div class="footer">
      <RouterLink class="footer-link" :to="{ name: 'categories' }">分类管理</RouterLink>
      <RouterLink class="footer-link" :to="{ name: 'entries' }">文章列表</RouterLink>
    </div>
  </nav>
</template>

<style scoped>
.directory {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  height: 100%;
  /* min-height: 0 with the overflow below: without it the flex item grows to
     its content and the list scrolls the whole page instead of the pane. */
  min-height: 0;
  padding: var(--space-6) var(--space-3) var(--space-4);
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--c-line-strong) transparent;
  background: var(--c-glass);
  backdrop-filter: blur(18px) saturate(140%);
}

.directory::-webkit-scrollbar {
  width: 6px;
}

.directory::-webkit-scrollbar-track {
  background: transparent;
}

.directory::-webkit-scrollbar-thumb {
  border: 2px solid transparent;
  border-radius: 999px;
  background: var(--c-line-strong);
  background-clip: content-box;
}

.directory::-webkit-scrollbar-thumb:hover {
  background: var(--c-ink-faint);
  background-clip: content-box;
}

.brand {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.wordmark {
  font-size: 0.9375rem;
  font-weight: 600;
  letter-spacing: -0.01em;
}

.search-label {
  display: block;
  margin-bottom: var(--space-1);
  color: var(--c-ink-muted);
  font-size: 0.75rem;
}

.search-control {
  position: relative;
}

.search-icon {
  position: absolute;
  top: 50%;
  left: var(--space-2);
  width: 0.875rem;
  height: 0.875rem;
  color: var(--c-ink-faint);
  transform: translateY(-50%);
  pointer-events: none;
}

.search-input {
  width: 100%;
  min-height: 2.25rem;
  padding: 0.3125rem var(--space-2) 0.3125rem 2rem;
  border: 1px solid var(--c-glass-border);
  border-radius: var(--radius-control);
  background: var(--c-paper);
  color: var(--c-ink);
  font: inherit;
  font-size: 0.8125rem;
}

.group-title {
  margin: 0 0 var(--space-3);
  color: var(--c-ink-faint);
  font-size: 0.875rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.group-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
}

.group-heading .group-title {
  margin-bottom: 0;
}

.recent-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-2);
}

.manage-action {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--c-ink-faint);
  font: inherit;
  font-size: 0.75rem;
  cursor: pointer;
}

.manage-action:hover {
  color: var(--c-ink);
}

.manage-action--danger {
  color: var(--c-danger);
}

.manage-action--danger:hover {
  color: var(--c-danger);
}

.list {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.list > li {
  position: relative;
}

.item {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  width: 100%;
  min-height: 2.75rem;
  padding: 0.375rem var(--space-2) 0.375rem var(--space-3);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--c-ink-muted);
  font: inherit;
  font-size: 0.8125rem;
  text-align: left;
  cursor: pointer;
}

.item:hover {
  background: var(--c-surface);
  color: var(--c-ink);
}

/* aria-current, not a class: the attribute is both the announcement and the
   styling hook, so the two cannot drift apart. */
.item[aria-current='true'] {
  background: var(--c-surface);
  color: var(--c-ink);
  font-weight: 500;
  box-shadow: var(--shadow-control);
}

.item[aria-current='true'] .item-meta {
  padding-inline-start: var(--space-2);
}

.item-cursor {
  position: absolute;
  top: 50%;
  left: var(--space-1);
  width: 0.1875rem;
  height: 1.25rem;
  border-radius: var(--radius-control);
  background: var(--c-alive);
  transform: translateY(-50%);
}

.item-cursor + .item-title {
  padding-inline-start: var(--space-2);
}

.recent-checkbox {
  position: absolute;
  inset-inline-start: var(--space-2);
  top: 50%;
  z-index: 1;
  width: 1rem;
  height: 1rem;
  margin: 0;
  accent-color: var(--c-alive);
  transform: translateY(-50%);
}

.item--selectable {
  padding-inline-start: 2.75rem;
}

.item--selectable .item-cursor {
  display: none;
}

.item-title {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.item-title--excerpt {
  position: relative;
  display: -webkit-box;
  overflow: hidden;
  white-space: normal;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-height: 1.35;
  line-clamp: 2;
  text-overflow: clip;
}

.item-title--excerpt::after {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 45%;
  height: 1.35em;
  background: linear-gradient(90deg, transparent, var(--c-surface));
  content: '';
  pointer-events: none;
}

.item-meta {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  color: var(--c-ink-faint);
  font-size: 0.6875rem;
}

.dot {
  width: 0.375rem;
  height: 0.375rem;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--c-accent);
}

.group-toggle {
  display: block;
  width: 100%;
  padding: 0.25rem var(--space-2);
  border: none;
  background: transparent;
  color: var(--c-ink-muted);
  font: inherit;
  font-size: 0.8125rem;
  text-align: left;
  cursor: pointer;
}

.group-toggle:hover {
  color: var(--c-ink);
}

.status-group {
  margin-bottom: var(--space-1);
}

.state {
  padding: 0.25rem var(--space-2);
  color: var(--c-ink-faint);
  font-size: 0.75rem;
}

.error {
  padding: var(--space-2);
  border-radius: var(--radius-sm);
  background: var(--c-danger-surface);
  color: var(--c-danger);
  font-size: 0.75rem;
}

.footer {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  /* Pinned to the bottom of the pane, not to the end of the list: the groups
     above scroll, and these two links should stay where they were found. */
  margin-top: auto;
  padding-top: var(--space-3);
  border-top: 1px solid var(--c-line);
}

.footer-link {
  color: var(--c-ink-faint);
  font-size: 0.75rem;
}

@media (max-width: 23.4375rem) {
  .directory {
    width: 100%;
    max-width: 100vw;
    overflow-x: clip;
  }
}
</style>
