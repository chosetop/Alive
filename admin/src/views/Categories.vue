<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { buildCategoryPatch, categoriesApi, isEmptyPatch, toUserMessage } from '../api'
import type { Category } from '../types/api'
import CategoryForm from '../components/CategoryForm.vue'

/**
 * Category management: list, create, edit, delete.
 *
 * Categories are the simplest writable thing in the API — no body text, no
 * status machine, no soft delete — which is why this screen comes before the
 * article list. Whatever is awkward about "list + form + 409 handling" surfaces
 * here, where there is less to go wrong.
 */

const items = ref<Category[]>([])
const isLoading = ref(true)
/** Load failure. Kept apart from `formError` so a failed save cannot blank the list. */
const loadError = ref<string | null>(null)

/** `null` = form closed. `'new'` = creating. A Category = editing that one. */
const editing = ref<Category | 'new' | null>(null)
const isSaving = ref(false)
const formError = ref<string | null>(null)
/** Per-field messages from a 400/409, keyed by field name as the API sends them. */
const fieldErrors = ref<Record<string, string>>({})

/** Which row is awaiting delete confirmation. Inline, so no modal to trap focus. */
const confirmingDelete = ref<number | null>(null)
const isDeleting = ref(false)

const isEmpty = computed(() => !isLoading.value && items.value.length === 0)

async function load(): Promise<void> {
  isLoading.value = true
  loadError.value = null
  try {
    // Admin shape: timestamps, no entry_count. Not paginated, so this is
    // `request` under the hood, not `requestPaginated`.
    items.value = await categoriesApi.listCategoriesAdmin()
  } catch (error) {
    loadError.value = toUserMessage(error)
  } finally {
    isLoading.value = false
  }
}

onMounted(load)

function openCreate(): void {
  formError.value = null
  fieldErrors.value = {}
  editing.value = 'new'
}

function openEdit(item: Category): void {
  formError.value = null
  fieldErrors.value = {}
  confirmingDelete.value = null
  editing.value = item
}

function closeForm(): void {
  editing.value = null
  formError.value = null
  fieldErrors.value = {}
}

/** Pulls `fields` off an ApiClientError without importing the class here. */
function applyError(error: unknown): void {
  formError.value = toUserMessage(error)
  const fields = (error as { fields?: Record<string, string> }).fields
  fieldErrors.value = fields ?? {}
}

async function handleSubmit(form: {
  name: string
  slug: string
  description: string
  sortOrder: number
}): Promise<void> {
  if (isSaving.value || editing.value === null) return

  isSaving.value = true
  formError.value = null
  fieldErrors.value = {}

  try {
    if (editing.value === 'new') {
      await categoriesApi.createCategory({
        name: form.name,
        slug: form.slug,
        description: form.description,
        sort_order: form.sortOrder,
      })
    } else {
      // Diff against the loaded record so a save touches only what was typed.
      // See buildCategoryPatch for why null is never sent.
      const patch = buildCategoryPatch(editing.value, form)
      if (isEmptyPatch(patch)) {
        // The server answers an empty patch with 400 by design. Nothing was
        // edited, so closing is the honest outcome, not an error.
        closeForm()
        return
      }
      await categoriesApi.updateCategory(editing.value.id, patch)
    }

    closeForm()
    await load()
  } catch (error) {
    applyError(error)
  } finally {
    isSaving.value = false
  }
}

async function handleDelete(id: number): Promise<void> {
  if (isDeleting.value) return
  isDeleting.value = true
  try {
    await categoriesApi.deleteCategory(id)
    confirmingDelete.value = null
    await load()
  } catch (error) {
    // Surfaced on the list, not in the form: the form may not even be open.
    loadError.value = toUserMessage(error)
  } finally {
    isDeleting.value = false
  }
}
</script>

<template>
  <div class="page">
    <header class="head">
      <div>
        <h1 class="title">分类</h1>
        <p class="subtitle">分类是站点导航，不是内容。超过两层就是你自己都记不住的信号。</p>
      </div>
      <button class="btn btn--primary" type="button" :disabled="editing !== null" @click="openCreate">
        新建分类
      </button>
    </header>

    <CategoryForm
      v-if="editing !== null"
      :original="editing === 'new' ? null : editing"
      :is-saving="isSaving"
      :error="formError"
      :field-errors="fieldErrors"
      @submit="handleSubmit"
      @cancel="closeForm"
    />

    <p v-if="loadError" class="alert" role="alert">{{ loadError }}</p>

    <p v-if="isLoading" class="state">载入中…</p>

    <p v-else-if="isEmpty" class="state state--empty">
      还没有分类。新建一个之后，写文章时就能把它归进去。
    </p>

    <ul v-else class="list">
      <li v-for="item in items" :key="item.id" class="row">
        <div class="row-main">
          <div class="row-head">
            <span class="name">{{ item.name }}</span>
            <code class="slug">{{ item.slug }}</code>
            <span class="order" :title="`排序值 ${item.sort_order}`">#{{ item.sort_order }}</span>
          </div>
          <p v-if="item.description" class="desc">{{ item.description }}</p>
          <p v-else class="desc desc--empty">没有描述</p>
        </div>

        <div class="row-actions">
          <template v-if="confirmingDelete === item.id">
            <!-- Says what actually happens. "Entries become uncategorised" is a
                 different promise from "entries will be deleted", and the
                 foreign key here is ON DELETE SET NULL. -->
            <span class="confirm-text">删除「{{ item.name }}」？其中的文章会变成未分类，不会被删除。</span>
            <button class="btn btn--danger" type="button" :disabled="isDeleting" @click="handleDelete(item.id)">
              {{ isDeleting ? '删除中…' : '确认删除' }}
            </button>
            <button class="btn" type="button" :disabled="isDeleting" @click="confirmingDelete = null">
              取消
            </button>
          </template>
          <template v-else>
            <button class="btn" type="button" @click="openEdit(item)">编辑</button>
            <button class="btn btn--quiet" type="button" @click="confirmingDelete = item.id">
              删除
            </button>
          </template>
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.page {
  max-width: 48rem;
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

.subtitle {
  color: var(--c-ink-muted);
  font-size: 0.875rem;
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

.row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4);
}

.row + .row {
  border-top: 1px solid var(--c-line);
}

.row-main {
  min-width: 0;
}

.row-head {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-bottom: var(--space-1);
}

.name {
  font-size: 0.9375rem;
  font-weight: 500;
}

.slug {
  color: var(--c-ink-muted);
  font-family: var(--font-mono);
  font-size: 0.8125rem;
}

.order {
  color: var(--c-ink-faint);
  font-family: var(--font-mono);
  font-size: 0.75rem;
}

.desc {
  color: var(--c-ink-muted);
  font-size: 0.875rem;
}

.desc--empty {
  color: var(--c-ink-faint);
  font-style: italic;
}
.row-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-2);
  flex-shrink: 0;
}

.confirm-text {
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
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
    border-color 0.12s ease,
    background-color 0.12s ease;
}

.btn:hover:not(:disabled) {
  border-color: var(--c-ink-muted);
  color: var(--c-ink);
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

.btn--danger {
  border-color: var(--c-danger);
  color: var(--c-danger);
}

.btn--danger:hover:not(:disabled) {
  background: var(--c-danger-surface);
  border-color: var(--c-danger);
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
  }
}
</style>
