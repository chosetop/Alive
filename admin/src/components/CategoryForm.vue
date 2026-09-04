<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Category } from '../types/api'
import { UiButton } from './ui'

/**
 * Create/edit form for one category.
 *
 * Owns no requests. The parent holds the data and does the saving, so this
 * component stays a form and the two directions of "what changed" do not end up
 * split across both files.
 */

const props = defineProps<{
  /** `null` when creating. Otherwise the record being edited. */
  original: Category | null
  isSaving: boolean
  /** Whole-request failure message, already turned into something readable. */
  error: string | null
  /** Per-field messages from a 400/409, keyed as the API sends them. */
  fieldErrors: Record<string, string>
}>()

const emit = defineEmits<{
  submit: [{ name: string; slug: string; description: string; sortOrder: number }]
  cancel: []
}>()

const name = ref('')
const slug = ref('')
const description = ref('')
const sortOrder = ref(0)

const isEdit = computed(() => props.original !== null)

/**
 * Reset when the target changes, including between two different categories.
 * `immediate` covers the initial mount, so create and edit take one path.
 */
watch(
  () => props.original,
  (record) => {
    name.value = record?.name ?? ''
    slug.value = record?.slug ?? ''
    description.value = record?.description ?? ''
    sortOrder.value = record?.sort_order ?? 0
  },
  { immediate: true },
)

/**
 * Slug format, matching the CHECK constraint in migration 000004: lowercase
 * letters and digits in groups joined by single hyphens, no leading, trailing
 * or doubled hyphen.
 *
 * Checked here so a typo is caught before a round trip, not to replace the
 * server's validation — it stays authoritative.
 */
const SLUG_PATTERN = /^[a-z0-9]+(-[a-z0-9]+)*$/

/**
 * Local validation, deliberately thin: required, length, slug format. Anything
 * the server knows and the browser does not — whether a slug is taken — is left
 * to the server, which answers 409.
 */
const localError = computed<string | null>(() => {
  if (name.value.trim() === '') return '名称不能为空'
  if ([...name.value].length > 64) return '名称最长 64 个字符'
  if (slug.value === '') return 'slug 不能为空'
  if (slug.value.length > 64) return 'slug 最长 64 个字符'
  if (!SLUG_PATTERN.test(slug.value)) {
    return 'slug 只能是小写字母和数字，用单个连字符分隔，例如 travel 或 long-form'
  }
  return null
})

const canSubmit = computed(() => !props.isSaving && localError.value === null)

function handleSubmit(): void {
  if (!canSubmit.value) return
  emit('submit', {
    name: name.value.trim(),
    slug: slug.value,
    description: description.value,
    sortOrder: sortOrder.value,
  })
}
</script>

<template>
  <!-- A real form element, so Enter submits and the browser's own semantics
       apply. @submit.prevent stops the navigation, not the behaviour. -->
  <form class="form" novalidate @submit.prevent="handleSubmit">
    <h2 class="form-title">{{ isEdit ? `编辑「${original?.name}」` : '新建分类' }}</h2>

    <div class="grid">
      <div class="field">
        <label class="label" for="cat-name">名称</label>
        <input
          id="cat-name"
          v-model="name"
          class="input"
          :class="{ 'input--invalid': fieldErrors.name }"
          type="text"
          maxlength="64"
          autocomplete="off"
          :disabled="isSaving"
        />
        <p class="hint">导航上显示的文字。中文不需要转写。</p>
        <p v-if="fieldErrors.name" class="field-error">{{ fieldErrors.name }}</p>
      </div>

      <div class="field">
        <label class="label" for="cat-slug">slug</label>
        <input
          id="cat-slug"
          v-model="slug"
          class="input input--mono"
          :class="{ 'input--invalid': fieldErrors.slug }"
          type="text"
          maxlength="64"
          autocomplete="off"
          spellcheck="false"
          :disabled="isSaving"
        />
        <!-- Not derived from the name: deriving it from Chinese would produce
             either a percent-encoded URL nobody can read or a pinyin
             dependency. Same reasoning as the entry slug. -->
        <p class="hint">URL 里的那一段，需要自己填。小写字母、数字、单连字符。</p>
        <p v-if="fieldErrors.slug" class="field-error">{{ fieldErrors.slug }}</p>
      </div>
    </div>

    <div class="field">
      <label class="label" for="cat-desc">描述</label>
      <textarea
        id="cat-desc"
        v-model="description"
        class="input textarea"
        rows="2"
        :disabled="isSaving"
      ></textarea>
      <p class="hint">用在分类页上，同时作为它的 meta description。留空即清除。</p>
    </div>

    <div class="field field--narrow">
      <label class="label" for="cat-order">排序</label>
      <input
        id="cat-order"
        v-model.number="sortOrder"
        class="input input--mono"
        type="number"
        step="1"
        :disabled="isSaving"
      />
      <p class="hint">升序排列。0 是真实位置（最前），不是「未指定」。</p>
    </div>

    <!-- Local validation shows only once something has been typed, so an
         untouched form is not scolding anyone. -->
    <p v-if="localError && (name !== '' || slug !== '')" class="alert alert--soft">
      {{ localError }}
    </p>
    <p v-if="error" class="alert" role="alert">{{ error }}</p>

    <div class="actions">
      <UiButton :disabled="isSaving" @click="emit('cancel')">取消</UiButton>
       <UiButton type="submit" variant="primary" :disabled="!canSubmit">
        {{ isSaving ? '保存中…' : '保存' }}
      </UiButton>
    </div>
  </form>
</template>

<style scoped>
.form {
  margin-bottom: var(--space-6);
  padding: var(--space-5);
  border: 1px solid var(--c-glass-border);
  border-radius: var(--radius-surface);
  background: var(--c-glass);
  box-shadow: var(--shadow-control);
  backdrop-filter: blur(18px) saturate(140%);
}

.form-title {
  margin-bottom: var(--space-5);
  font-size: 1rem;
}

.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-4);
}

.field {
  margin-bottom: var(--space-4);
  min-width: 0;
}

.field--narrow {
  max-width: 10rem;
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
  border-radius: var(--radius-control);
  background: var(--c-surface);
  color: var(--c-ink);
  font-family: inherit;
  font-size: 0.875rem;
}

.input:focus {
  border-color: var(--c-accent);
  outline: 2px solid transparent;
  outline-offset: 1px;
}

.input:disabled {
  background: var(--c-surface-sunken);
  color: var(--c-ink-faint);
}

.input--mono {
  font-family: var(--font-mono);
}

/* Paired with the per-field message below the input. Colour alone would not
   survive greyscale or reach anyone using a screen reader. */
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

.alert {
  margin-bottom: var(--space-4);
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--c-danger);
  border-radius: var(--radius-surface);
  background: var(--c-danger-surface);
  color: var(--c-danger);
  font-size: 0.875rem;
}

/* Local validation, not a server rejection: same information, less alarm. */
.alert--soft {
  border-color: var(--c-line-strong);
  background: var(--c-surface-sunken);
  color: var(--c-ink-muted);
}

.actions {
  display: flex;
  gap: var(--space-2);
}

@media (max-width: 40rem) {
  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
