<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { categoriesApi, entriesApi, toUserMessage } from '../api'
import type { Category, EntryDetail, EntryListItem, EntryPatchFields, EntryStatus, WorldKey } from '../types/api'
import EntryRow from '../components/EntryRow.vue'
import ArticleSettings from '../components/writing/ArticleSettings.vue'
import { UiSelect, type UiSelectOption } from '../components/ui'

/**
 * The article list: a todo queue first, an archive second.
 *
 * `draft` and `archived` are deliberately not one "not published" bucket. A
 * draft is unfinished work that wants attention; an archived entry was finished
 * and then taken down, and wants none. Merging them would bury the first in the
 * second, and the queue is the reason to open this screen at all.
 *
 * Filtering happens server side via query parameters, not by slicing a full list
 * client side: the list is paginated, so a client-side filter would only ever
 * filter the current page and would quietly under-report.
 */

/** `null` = every status. The tab order puts the queue before the archive. */
type StatusFilter = EntryStatus | null

const TABS: ReadonlyArray<{ label: string; value: StatusFilter }> = [
  { label: '全部', value: null },
  { label: '草稿', value: 'draft' },
  { label: '已发布', value: 'published' },
  { label: '已归档', value: 'archived' },
]

const items = ref<EntryListItem[]>([])
const categories = ref<Category[]>([])
const isLoading = ref(true)
const loadError = ref<string | null>(null)
const categoriesError = ref<string | null>(null)
const settingsEntry = ref<EntryDetail | null>(null)
const settingsCategories = ref<Category[]>([])
const settingsPatch = ref<EntryPatchFields>({})
const settingsOpen = ref(false)
const settingsSaving = ref(false)
const settingsError = ref<string | null>(null)
const isOrdering = ref(false)
const orderingSaving = ref(false)
const draggedId = ref<number | null>(null)
const orderingItems = ref<EntryListItem[]>([])

const route = useRoute()
const router = useRouter()

const initialStatus = route.query.status === 'draft' || route.query.status === 'published' || route.query.status === 'archived'
  ? route.query.status
  : null
const activeStatus = ref<StatusFilter>(initialStatus)
const activeSearch = ref(typeof route.query.q === 'string' ? route.query.q : '')
const activeCategory = ref(typeof route.query.category === 'string' ? route.query.category : '')
const activeWorld = ref<WorldKey | ''>(route.query.world === 'journal' || route.query.world === 'saying' || route.query.world === 'video' ? route.query.world : '')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const totalPages = computed(() => (total.value === 0 ? 0 : Math.ceil(total.value / pageSize.value)))
const hasPrev = computed(() => page.value > 1)
const hasNext = computed(() => page.value < totalPages.value)
const displayedItems = computed(() => {
  if (!isOrdering.value) return items.value
  if (!activeCategory.value) return orderingItems.value
  return orderingItems.value.filter((item) => item.category?.slug === activeCategory.value)
})
const isEmpty = computed(() => !isLoading.value && displayedItems.value.length === 0)
const settingsDirty = computed(() => Object.keys(settingsPatch.value).length > 0)
const worldOptions: UiSelectOption[] = [
  { value: '', label: '全部世界' },
  { value: 'journal', label: '日志' },
  { value: 'saying', label: '片语' },
  { value: 'video', label: '影像' },
]
const categoryOptions = computed<UiSelectOption[]>(() => [
  { value: '', label: '全部分类' },
  ...categories.value.map((category) => ({ value: category.slug, label: category.name })),
])
const orderingWorldOptions = worldOptions.filter((option) => option.value !== '')
const orderingScopeLabel = computed(() => {
  const world = worldOptions.find((option) => option.value === activeWorld.value)?.label ?? ''
  const category = categoryOptions.value.find((option) => option.value === activeCategory.value)?.label ?? '全部分类'
  return activeCategory.value ? `「${world} / ${category}」` : `「${world}」`
})

async function load(): Promise<void> {
  if (isOrdering.value) return
  isLoading.value = true
  loadError.value = null
  try {
    const response = await entriesApi.listEntriesAdmin({
      page: page.value,
      world: activeWorld.value || undefined,
      // `status: null` means "no filter", and the API client drops undefined
      // rather than serialising it, so null becomes absent here.
      status: activeStatus.value ?? undefined,
      q: activeSearch.value.trim() || undefined,
      category: activeCategory.value || undefined,
    })
    items.value = response.data
    // Trust the server's echo over the local guess: `page_size` is clamped and
    // may come back smaller than what was asked for.
    page.value = response.meta.page
    pageSize.value = response.meta.page_size
    total.value = response.meta.total
  } catch (error) {
    loadError.value = toUserMessage(error)
  } finally {
    isLoading.value = false
  }
}

async function enterOrdering(): Promise<void> {
  isOrdering.value = true
  loadError.value = null
  if (!activeWorld.value) {
    activeWorld.value = 'journal'
    activeCategory.value = ''
  }
  activeStatus.value = 'published'
  activeSearch.value = ''
  page.value = 1
  syncFiltersToUrl()
  await loadCategories()
  await loadOrdering()
}

async function loadOrdering(): Promise<void> {
  if (!activeWorld.value) return
  isLoading.value = true
  try {
    const ordered = await entriesApi.listPublishedOrder(activeWorld.value)
    orderingItems.value = ordered
    total.value = ordered.length
    pageSize.value = Math.max(ordered.length, 1)
  } catch (error) {
    loadError.value = toUserMessage(error)
  } finally {
    isLoading.value = false
  }
}

function leaveOrdering(): void {
  isOrdering.value = false
  draggedId.value = null
  orderingItems.value = []
  void load()
}

async function persistOrder(previous: EntryListItem[]): Promise<void> {
  if (!activeWorld.value || orderingSaving.value) return
  orderingSaving.value = true
  loadError.value = null
  try {
    await entriesApi.reorderPublished(activeWorld.value, orderingItems.value.map((item) => item.id))
  } catch (error) {
    orderingItems.value = previous
    loadError.value = toUserMessage(error)
  } finally {
    orderingSaving.value = false
  }
}

function moveEntry(entryId: number, targetIndex: number): void {
  if (orderingSaving.value) return
  const scopedItems = displayedItems.value
  const from = scopedItems.findIndex((item) => item.id === entryId)
  if (from < 0 || targetIndex < 0 || targetIndex >= scopedItems.length || from === targetIndex) return
  const previous = [...orderingItems.value]
  const next = [...scopedItems]
  const [moved] = next.splice(from, 1)
  next.splice(targetIndex, 0, moved)
  if (activeCategory.value) {
    let scopedIndex = 0
    orderingItems.value = orderingItems.value.map((item) =>
      item.category?.slug === activeCategory.value ? next[scopedIndex++]! : item,
    )
  } else {
    orderingItems.value = next
  }
  void persistOrder(previous)
}

function moveBy(entryId: number, delta: -1 | 1): void {
  const from = displayedItems.value.findIndex((item) => item.id === entryId)
  moveEntry(entryId, from + delta)
}

function dropOn(targetId: number): void {
  if (draggedId.value === null) return
  const targetIndex = displayedItems.value.findIndex((item) => item.id === targetId)
  moveEntry(draggedId.value, targetIndex)
  draggedId.value = null
}

onMounted(load)
async function loadCategories(): Promise<void> {
  categoriesError.value = null
  try {
    if (activeWorld.value) {
      categories.value = await categoriesApi.listCategoriesAdmin({ world: activeWorld.value })
    } else {
      const grouped = await Promise.all(
        (['journal', 'saying', 'video'] as WorldKey[]).map((world) => categoriesApi.listCategoriesAdmin({ world })),
      )
      categories.value = grouped.flat().filter((category, index, all) => all.findIndex((item) => item.slug === category.slug) === index)
    }
  } catch (error) {
    categoriesError.value = toUserMessage(error)
  }
}

onMounted(loadCategories)

/** Changing the filter resets to page 1: page 3 of drafts is rarely page 3 of published. */
function selectStatus(value: StatusFilter): void {
  if (activeStatus.value === value) return
  activeStatus.value = value
  page.value = 1
  syncFiltersToUrl()
}

function queryValue(value: string | undefined): string | undefined {
  return value?.trim() || undefined
}

function syncFiltersToUrl(): void {
  void router.replace({
    query: {
      ...route.query,
      q: queryValue(activeSearch.value),
      category: queryValue(activeCategory.value),
      world: activeWorld.value || undefined,
      status: activeStatus.value ?? undefined,
      page: page.value > 1 ? String(page.value) : undefined,
    },
  })
}

function selectWorld(value: string): void {
  activeWorld.value = value as WorldKey | ''
  activeCategory.value = ''
  page.value = 1
  syncFiltersToUrl()
  void loadCategories()
}

function selectCategory(value: string): void {
  activeCategory.value = value
  page.value = 1
  syncFiltersToUrl()
}

async function selectOrderingWorld(value: string): Promise<void> {
  activeWorld.value = value as WorldKey
  activeCategory.value = ''
  page.value = 1
  syncFiltersToUrl()
  await loadCategories()
  await loadOrdering()
}

function selectOrderingCategory(value: string): void {
  activeCategory.value = value
  draggedId.value = null
  syncFiltersToUrl()
}

function updateSearch(): void {
  page.value = 1
  syncFiltersToUrl()
}

watch([activeStatus, activeSearch, activeCategory, activeWorld, page], load)

watch(
  () => route.query,
  (query) => {
    const nextStatus = query.status === 'draft' || query.status === 'published' || query.status === 'archived'
      ? query.status
      : null
    const nextSearch = typeof query.q === 'string' ? query.q : ''
    const nextCategory = typeof query.category === 'string' ? query.category : ''
    const nextWorld = query.world === 'journal' || query.world === 'saying' || query.world === 'video' ? query.world : ''
    if (
      activeStatus.value === nextStatus &&
      activeSearch.value === nextSearch &&
      activeCategory.value === nextCategory &&
      activeWorld.value === nextWorld
    ) return
    activeStatus.value = nextStatus
    activeSearch.value = nextSearch
    activeCategory.value = nextCategory
    activeWorld.value = nextWorld
    void loadCategories()
    page.value = typeof query.page === 'string' && Number(query.page) > 1 ? Number(query.page) : 1
  },
  { deep: true },
)

const emptyMessage = computed(() => {
  switch (activeStatus.value) {
    case 'draft':
      return '没有草稿。'
    case 'published':
      return '还没有发布任何内容。'
    case 'archived':
      return '归档是空的。'
    default:
      if (activeSearch.value || activeCategory.value) return '没有符合当前筛选条件的内容。'
      return '还没有内容。写第一篇之后它会出现在这里。'
  }
})

async function openQuickSettings(item: EntryListItem): Promise<void> {
  settingsError.value = null
  try {
    const detail = await entriesApi.getEntry(item.id)
    const itemCategories = await categoriesApi.listCategoriesAdmin({ world: detail.world })
    settingsEntry.value = detail
    settingsCategories.value = itemCategories
    settingsPatch.value = {}
    settingsOpen.value = true
  } catch (cause) {
    settingsError.value = toUserMessage(cause)
  }
}

function updateQuickSettings(fields: EntryPatchFields): void {
  if (!settingsEntry.value) return
  settingsEntry.value = { ...settingsEntry.value, ...fields }
  settingsPatch.value = { ...settingsPatch.value, ...fields }
  settingsError.value = null
}

function closeQuickSettings(open: boolean): void {
  if (open) return
  settingsOpen.value = false
  settingsEntry.value = null
  settingsCategories.value = []
  settingsPatch.value = {}
  settingsError.value = null
}

async function saveQuickSettings(): Promise<void> {
  const entry = settingsEntry.value
  if (!entry || !settingsDirty.value || settingsSaving.value) return

  settingsSaving.value = true
  settingsError.value = null
  try {
    const saved = await entriesApi.updateEntry(entry.id, {
      revision: entry.revision,
      ...settingsPatch.value,
    })
    const category = settingsCategories.value.find((item) => item.id === saved.category_id) ?? null
    settingsEntry.value = { ...saved, category }
    settingsPatch.value = {}
    items.value = items.value.map((item) => (item.id === saved.id ? { ...saved, category } : item))
  } catch (cause) {
    settingsError.value = toUserMessage(cause)
  } finally {
    settingsSaving.value = false
  }
}

async function deleteFromSettings(): Promise<void> {
  const entry = settingsEntry.value
  if (!entry || settingsSaving.value) return
  settingsSaving.value = true
  settingsError.value = null
  try {
    await entriesApi.deleteEntry(entry.id)
    closeQuickSettings(false)
    await load()
  } catch (cause) {
    settingsError.value = toUserMessage(cause)
  } finally {
    settingsSaving.value = false
  }
}
</script>

<template>
  <div class="page">
    <header class="head">
      <div>
        <h1 class="title">内容</h1>
      </div>
      <div class="head-actions">
        <button class="btn" type="button" data-entry-order-toggle @click="isOrdering ? leaveOrdering() : enterOrdering()">
          {{ isOrdering ? '完成排序' : '调整前台顺序' }}
        </button>
        <RouterLink class="btn btn--primary" :to="{ name: 'entry-new' }">写一篇</RouterLink>
      </div>
    </header>

    <div v-if="isOrdering" class="order-panel">
      <p class="order-note" role="status">
        正在调整{{ orderingScopeLabel }}的前台顺序。拖动条目，或使用上下按钮；每次移动都会自动保存。
      </p>
      <div class="order-scope" aria-label="排序范围">
        <UiSelect
          :model-value="activeWorld"
          :options="orderingWorldOptions"
          label="排序世界"
          size="compact"
          @change="(value) => void selectOrderingWorld(value)"
        />
        <UiSelect
          :model-value="activeCategory"
          :options="categoryOptions"
          label="排序分类"
          size="compact"
          @change="selectOrderingCategory"
        />
      </div>
    </div>

    <div v-if="!isOrdering" class="filters" aria-label="内容筛选">
      <div class="world-field" data-test="entry-world">
        <UiSelect :model-value="activeWorld" :options="worldOptions" label="按世界筛选" size="compact" @change="selectWorld" />
      </div>
      <label class="search-field">
        <span class="sr-only">搜索文章</span>
        <input
          v-model="activeSearch"
          class="input"
          data-test="entry-search"
          type="search"
          placeholder="搜索标题、摘要或 slug"
          @input="updateSearch"
        />
      </label>
      <div class="category-field" data-test="entry-category">
        <UiSelect :model-value="activeCategory" :options="categoryOptions" label="按分类筛选" size="compact" @change="selectCategory" />
      </div>
    </div>
    <p v-if="categoriesError" class="filter-error" role="status">分类暂时无法载入，仍可使用搜索。</p>

    <!-- Tabs, not a select: four values, and the current one should be visible
         without opening anything. -->
    <div v-if="!isOrdering" class="tabs" role="tablist" aria-label="按状态筛选">
      <button
        v-for="tab in TABS"
        :key="tab.label"
        class="tab"
        :class="{ 'tab--active': activeStatus === tab.value }"
        type="button"
        role="tab"
        :aria-selected="activeStatus === tab.value"
        @click="selectStatus(tab.value)"
      >
        {{ tab.label }}
      </button>
    </div>

    <p v-if="loadError" class="alert" role="alert">{{ loadError }}</p>

    <p v-if="isLoading" class="state">载入中…</p>

    <p v-else-if="isEmpty" class="state state--empty">{{ emptyMessage }}</p>

    <template v-else>
      <ul class="list" :aria-busy="orderingSaving ? 'true' : undefined">
        <EntryRow
          v-for="(item, index) in displayedItems"
          :key="item.id"
          :entry="item"
          :reorderable="isOrdering"
          :dragging="draggedId === item.id"
          :first="index === 0"
          :last="index === displayedItems.length - 1"
          :draggable="isOrdering"
          @dragstart="draggedId = item.id"
          @dragend="draggedId = null"
          @dragover.prevent
          @drop.prevent="dropOn(item.id)"
          @move="moveBy(item.id, $event)"
          @settings="openQuickSettings"
        />
      </ul>

      <!-- Shown only when there is more than one page. A pager reading
           "1 / 1" is furniture. -->
      <nav v-if="!isOrdering && totalPages > 1" class="pager" aria-label="分页">
        <button class="btn" type="button" :disabled="!hasPrev" @click="page -= 1">上一页</button>
        <span class="pager-state">第 {{ page }} / {{ totalPages }} 页，共 {{ total }} 条</span>
        <button class="btn" type="button" :disabled="!hasNext" @click="page += 1">下一页</button>
      </nav>
      <p v-else class="count">共 {{ isOrdering ? displayedItems.length : total }} 条</p>
    </template>

    <p v-if="settingsError && !settingsOpen" class="alert" role="alert">{{ settingsError }}</p>

    <ArticleSettings
      v-if="settingsEntry"
      :open="settingsOpen"
      :entry="settingsEntry"
      :categories="settingsCategories"
      :disabled="settingsSaving"
      :dirty="settingsDirty"
      :saving="settingsSaving"
      :save-error="settingsError"
      :show-rich-actions="false"
      saveable
      @update:open="closeQuickSettings"
      @update="updateQuickSettings"
      @save="saveQuickSettings"
      @delete="deleteFromSettings"
    />
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
  margin-bottom: var(--space-5);
}

.head-actions { display: flex; align-items: center; gap: var(--space-2); }
.order-panel {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  background: var(--c-surface-sunken);
}

.order-note {
  margin: 0;
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.order-scope {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.btn--primary {
  border-color: var(--c-accent);
  background: var(--c-accent);
  color: var(--c-paper);
  padding: 0.375rem 0.875rem;
  font-size: 0.875rem;
}

.btn--primary:hover {
  border-color: var(--c-accent-hover);
  background: var(--c-accent-hover);
  color: var(--c-paper);
  text-decoration: none;
}

.title {
  margin-bottom: var(--space-2);
  font-size: 1.375rem;
}

.subtitle {
  color: var(--c-ink-muted);
  font-size: 0.875rem;
}

.tabs {
  display: flex;
  gap: var(--space-1);
  margin-bottom: var(--space-5);
  border-bottom: 1px solid var(--c-line);
}

.filters {
  display: flex;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}

.search-field {
  flex: 1;
}

.category-field {
  min-width: 10rem;
}

.input {
  width: 100%;
  min-height: 2.5rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-sm);
  background: var(--c-paper);
  color: var(--c-ink);
  font: inherit;
}

.input:focus {
  border-color: var(--c-accent);
  outline: 2px solid color-mix(in srgb, var(--c-accent) 25%, transparent);
  outline-offset: 1px;
}

.filter-error {
  margin: calc(var(--space-3) * -1) 0 var(--space-4);
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.tab {
  padding: 0.375rem var(--space-3);
  border: none;
  /* Sits on the container's border so the active tab can cover it. */
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  background: transparent;
  color: var(--c-ink-muted);
  font-family: inherit;
  font-size: 0.875rem;
  cursor: pointer;
  transition:
    color 0.12s ease,
    border-color 0.12s ease;
}

.tab:hover {
  color: var(--c-ink);
}

/* Weight shifts along with the underline: the state survives greyscale. */
.tab--active {
  border-bottom-color: var(--c-accent);
  color: var(--c-ink);
  font-weight: 500;
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

.state {
  padding: var(--space-5);
  color: var(--c-ink-faint);
  font-size: 0.875rem;
}

.state--empty {
  border: 1px dashed var(--c-line-strong);
  border-radius: var(--radius-md);
}

.list {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-md);
}

.pager {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  margin-top: var(--space-4);
}

.pager-state,
.count {
  color: var(--c-ink-faint);
  font-size: 0.8125rem;
}

.count {
  margin-top: var(--space-3);
}

.btn {
  padding: 0.25rem 0.625rem;
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
  cursor: pointer;
  transition:
    color 0.12s ease,
    border-color 0.12s ease;
}

.btn:hover:not(:disabled) {
  border-color: var(--c-ink-muted);
  color: var(--c-ink);
}

.btn:disabled {
  color: var(--c-ink-faint);
  cursor: not-allowed;
}

@media (max-width: 40rem) {
  .order-panel,
  .order-scope {
    align-items: stretch;
    grid-template-columns: minmax(0, 1fr);
  }

  .order-scope {
    display: grid;
  }

  .filters {
    flex-direction: column;
  }

  .category-field {
    min-width: 0;
  }

  .head .btn--primary {
    min-height: 2.75rem;
    padding-inline: 1rem;
    white-space: nowrap;
  }
}
</style>
