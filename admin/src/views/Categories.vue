<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { buildCategoryPatch, categoriesApi, isEmptyPatch, toUserMessage } from '../api'
import type { Category, WorldKey } from '../types/api'
import CategoryForm from '../components/CategoryForm.vue'
import { UiButton, UiIcon } from '../components/ui'

/**
 * Category management: list, create, edit, delete.
 *
 * Categories are the simplest writable thing in the API — no body text, no
 * status machine, no soft delete — which is why this screen comes before the
 * article list. Whatever is awkward about "list + form + 409 handling" surfaces
 * here, where there is less to go wrong.
 */

const items = ref<Category[]>([])
const selectedWorld = ref<WorldKey>('journal')
const WORLD_OPTIONS: ReadonlyArray<{ key: WorldKey; label: string }> = [
  { key: 'journal', label: '日志' },
  { key: 'saying', label: '片语' },
  { key: 'video', label: '影像' },
]
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
    items.value = await categoriesApi.listCategoriesAdmin({ world: selectedWorld.value })
  } catch (error) {
    loadError.value = toUserMessage(error)
  } finally {
    isLoading.value = false
  }
}

onMounted(() => {
  void load()
})

function openCreate(): void {
  formError.value = null
  fieldErrors.value = {}
  editing.value = 'new'
}

function selectWorld(world: WorldKey): void {
  if (selectedWorld.value === world) return
  selectedWorld.value = world
  void load()
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
        world: selectedWorld.value,
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
    <header class="head" data-page-header>
      <div class="head-copy">
        <h1 class="title">分类</h1>
      </div>
      <UiButton variant="primary" data-primary-action :disabled="editing !== null" @click="openCreate">
        <UiIcon class="button-icon" name="plus" />新建分类
      </UiButton>
    </header>

    <div class="world-filter" data-world-filter role="group" aria-label="按世界筛选">
      <button
        v-for="option in WORLD_OPTIONS"
        :key="option.key"
        class="world-filter__option"
        :class="{ 'world-filter__option--active': selectedWorld === option.key }"
        type="button"
        :data-world-option="option.key"
        :aria-pressed="selectedWorld === option.key"
        @click="selectWorld(option.key)"
      >
        {{ option.label }}
      </button>
    </div>

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
            <UiButton variant="danger" :disabled="isDeleting" @click="handleDelete(item.id)">
              {{ isDeleting ? '删除中…' : '确认删除' }}
            </UiButton>
            <UiButton :disabled="isDeleting" @click="confirmingDelete = null">
              取消
            </UiButton>
          </template>
          <template v-else>
            <UiButton @click="openEdit(item)">编辑</UiButton>
            <UiButton variant="quiet" @click="confirmingDelete = item.id">删除</UiButton>
          </template>
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.page {
  max-width: 56rem;
}

.head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-4);
  margin-bottom: var(--space-7);
}

.head-copy { min-width: 0; }
.button-icon { width: 1rem; height: 1rem; }

.title {
  margin: 0 0 var(--space-2);
  font-family: var(--font-heading);
  font-size: clamp(1.5rem, 3vw, 2rem);
  font-weight: 600;
  letter-spacing: -0.02em;
}

.world-filter {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  min-height: 2.75rem;
  margin-bottom: var(--space-5);
  padding: 0.25rem;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  background: var(--c-surface-sunken);
}

.world-filter__option {
  min-width: 4rem;
  min-height: 2.25rem;
  padding: 0 var(--space-3);
  border: 0;
  border-radius: calc(var(--radius-control) - 0.25rem);
  background: transparent;
  color: var(--c-ink-muted);
  font: inherit;
  font-size: 0.875rem;
  cursor: pointer;
  transition: color var(--motion-fast) ease, background-color var(--motion-fast) ease, box-shadow var(--motion-fast) ease, transform var(--motion-fast) ease;
}

.world-filter__option:hover {
  color: var(--c-ink);
}

.world-filter__option:active {
  transform: scale(0.96);
}

.world-filter__option:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: 1px;
}

.world-filter__option--active {
  background: var(--c-surface);
  box-shadow: var(--shadow-control);
  color: var(--c-ink);
  font-weight: 500;
}

@media (max-width: 30rem) {
  .world-filter {
    display: flex;
    width: 100%;
  }

  .world-filter__option {
    flex: 1;
    min-width: 0;
  }
}

.alert {
  margin-bottom: var(--space-4);
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--c-glass-border);
  border-radius: var(--radius-surface);
  background: var(--c-danger-surface);
  color: var(--c-danger);
  font-size: 0.875rem;
}

.state {
  padding: var(--space-5);
  border-radius: var(--radius-surface);
  background: var(--c-surface-sunken);
  color: var(--c-ink-faint);
  font-size: 0.875rem;
}

.state--empty {
  border: 1px dashed var(--c-line-strong);
}
.list {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--c-glass-border);
  border-radius: var(--radius-surface);
  background: var(--c-glass);
  box-shadow: var(--shadow-control);
  backdrop-filter: blur(18px) saturate(140%);
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

@media (max-width: 40rem) {
  .head {
    flex-direction: column;
  }

  .row {
    flex-direction: column;
  }
}
</style>
