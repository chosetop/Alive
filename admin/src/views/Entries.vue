<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { categoriesApi, entriesApi, toUserMessage } from '../api'
import type { Category, EntryListItem, EntryStatus } from '../types/api'
import EntryRow from '../components/EntryRow.vue'

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

const route = useRoute()
const router = useRouter()

const initialStatus = route.query.status === 'draft' || route.query.status === 'published' || route.query.status === 'archived'
  ? route.query.status
  : null
const activeStatus = ref<StatusFilter>(initialStatus)
const activeSearch = ref(typeof route.query.q === 'string' ? route.query.q : '')
const activeCategory = ref(typeof route.query.category === 'string' ? route.query.category : '')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const totalPages = computed(() => (total.value === 0 ? 0 : Math.ceil(total.value / pageSize.value)))
const hasPrev = computed(() => page.value > 1)
const hasNext = computed(() => page.value < totalPages.value)
const isEmpty = computed(() => !isLoading.value && items.value.length === 0)

async function load(): Promise<void> {
  isLoading.value = true
  loadError.value = null
  try {
    const response = await entriesApi.listEntriesAdmin({
      page: page.value,
      world: 'journal',
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

onMounted(load)
onMounted(async () => {
  try {
    categories.value = await categoriesApi.listCategoriesAdmin({ world: 'journal' })
  } catch (error) {
    categoriesError.value = toUserMessage(error)
  }
})

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
      status: activeStatus.value ?? undefined,
      page: page.value > 1 ? String(page.value) : undefined,
    },
  })
}

function selectCategory(value: string): void {
  activeCategory.value = value
  page.value = 1
  syncFiltersToUrl()
}

function updateSearch(): void {
  page.value = 1
  syncFiltersToUrl()
}

watch([activeStatus, activeSearch, activeCategory, page], load)

watch(
  () => route.query,
  (query) => {
    const nextStatus = query.status === 'draft' || query.status === 'published' || query.status === 'archived'
      ? query.status
      : null
    const nextSearch = typeof query.q === 'string' ? query.q : ''
    const nextCategory = typeof query.category === 'string' ? query.category : ''
    if (
      activeStatus.value === nextStatus &&
      activeSearch.value === nextSearch &&
      activeCategory.value === nextCategory
    ) return
    activeStatus.value = nextStatus
    activeSearch.value = nextSearch
    activeCategory.value = nextCategory
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
</script>

<template>
  <div class="page">
    <header class="head">
      <div>
        <h1 class="title">内容</h1>
        <p class="subtitle">草稿是待办队列，归档不是。这里把两者分开。</p>
      </div>
      <RouterLink class="btn btn--primary" :to="{ name: 'entry-new' }">写一篇</RouterLink>
    </header>

    <div class="filters" aria-label="内容筛选">
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
      <label class="category-field">
        <span class="sr-only">按分类筛选</span>
        <select
          :value="activeCategory"
          class="input"
          data-test="entry-category"
          aria-label="按分类筛选"
          @change="selectCategory(($event.target as HTMLSelectElement).value)"
        >
          <option value="">全部分类</option>
          <option v-for="category in categories" :key="category.id" :value="category.slug">
            {{ category.name }}
          </option>
        </select>
      </label>
    </div>
    <p v-if="categoriesError" class="filter-error" role="status">分类暂时无法载入，仍可使用搜索。</p>

    <!-- Tabs, not a select: four values, and the current one should be visible
         without opening anything. -->
    <div class="tabs" role="tablist" aria-label="按状态筛选">
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
      <ul class="list">
        <EntryRow v-for="item in items" :key="item.id" :entry="item" />
      </ul>

      <!-- Shown only when there is more than one page. A pager reading
           "1 / 1" is furniture. -->
      <nav v-if="totalPages > 1" class="pager" aria-label="分页">
        <button class="btn" type="button" :disabled="!hasPrev" @click="page -= 1">上一页</button>
        <span class="pager-state">第 {{ page }} / {{ totalPages }} 页，共 {{ total }} 条</span>
        <button class="btn" type="button" :disabled="!hasNext" @click="page += 1">下一页</button>
      </nav>
      <p v-else class="count">共 {{ total }} 条</p>
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
  margin-bottom: var(--space-5);
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
  .filters {
    flex-direction: column;
  }

  .category-field {
    min-width: 0;
  }
}
</style>
