<script setup lang="ts">
import { computed, onMounted, ref } from "vue";

import { tagsApi, toUserMessage } from "../api";
import type { Tag } from "../api/tags";
import { UiButton, UiIcon } from "../components/ui";

const items = ref<Tag[]>([]);
const query = ref("");
const isLoading = ref(true);
const loadError = ref<string | null>(null);
const editing = ref<Tag | "new" | null>(null);
const formName = ref("");
const formSlug = ref("");
const formError = ref<string | null>(null);
const isSaving = ref(false);
const confirmingDelete = ref<number | null>(null);
const isDeleting = ref(false);

const isEmpty = computed(() => !isLoading.value && items.value.length === 0);
const isEditing = computed(() => editing.value !== null);

async function load(): Promise<void> {
  isLoading.value = true;
  loadError.value = null;
  try {
    items.value = await tagsApi.listTags({
      q: query.value.trim() || undefined,
      limit: 50,
    });
  } catch (error) {
    loadError.value = toUserMessage(error);
  } finally {
    isLoading.value = false;
  }
}

onMounted(() => void load());

function openCreate(): void {
  editing.value = "new";
  formName.value = "";
  formSlug.value = "";
  formError.value = null;
}

function openEdit(item: Tag): void {
  editing.value = item;
  formName.value = item.name;
  formSlug.value = item.slug;
  formError.value = null;
  confirmingDelete.value = null;
}

function closeForm(): void {
  editing.value = null;
  formError.value = null;
}

async function handleSubmit(): Promise<void> {
  if (isSaving.value || editing.value === null) return;
  const name = formName.value.trim();
  const slug = formSlug.value.trim();
  if (!name || !slug) {
    formError.value = "名称和 slug 都不能为空";
    return;
  }

  isSaving.value = true;
  formError.value = null;
  try {
    if (editing.value === "new") {
      await tagsApi.createTag({ name, slug });
    } else {
      const patch: { name?: string; slug?: string } = {};
      if (name !== editing.value.name) patch.name = name;
      if (slug !== editing.value.slug) patch.slug = slug;
      if (Object.keys(patch).length === 0) {
        closeForm();
        return;
      }
      await tagsApi.updateTag(editing.value.id, patch);
    }
    closeForm();
    await load();
  } catch (error) {
    formError.value = toUserMessage(error);
  } finally {
    isSaving.value = false;
  }
}

async function handleDelete(item: Tag): Promise<void> {
  if (isDeleting.value || item.usage_count) return;
  isDeleting.value = true;
  try {
    await tagsApi.deleteTag(item.id);
    confirmingDelete.value = null;
    await load();
  } catch (error) {
    loadError.value = toUserMessage(error);
  } finally {
    isDeleting.value = false;
  }
}
</script>

<template>
  <div class="page">
    <header class="head" data-page-header>
      <div class="head-copy">
        <h1 class="title">标签</h1>
      </div>
    </header>

    <form class="search" @submit.prevent="void load()">
      <input
        id="tag-search"
        v-model="query"
        class="search-input"
        type="search"
        aria-label="搜索标签"
        placeholder="按名称或 slug 搜索"
      />
      <UiButton
        variant="primary"
        data-primary-action
        :disabled="isEditing"
        @click="openCreate"
      >
        <UiIcon class="button-icon" name="plus" />新建标签
      </UiButton>
    </form>

    <form
      v-if="editing !== null"
      class="form"
      data-tag-form
      @submit.prevent="void handleSubmit()"
    >
      <h2>{{ editing === "new" ? "新建标签" : `编辑「${editing.name}」` }}</h2>
      <div class="form-grid">
        <label
          >名称<input v-model="formName" maxlength="64" :disabled="isSaving"
        /></label>
        <label
          >slug<input
            v-model="formSlug"
            maxlength="64"
            spellcheck="false"
            :disabled="isSaving"
        /></label>
      </div>
      <p v-if="formError" class="alert" role="alert">{{ formError }}</p>
      <div class="actions">
        <UiButton :disabled="isSaving" @click="closeForm">取消</UiButton>
        <UiButton type="submit" variant="primary" :disabled="isSaving">{{
          isSaving ? "保存中…" : "保存"
        }}</UiButton>
      </div>
    </form>

    <p v-if="loadError" class="alert" role="alert">{{ loadError }}</p>
    <p v-if="isLoading" class="state">载入中…</p>
    <p v-else-if="isEmpty" class="state state--empty">
      还没有标签。新建一个后，就能在日志的文章设置里使用。
    </p>

    <ul v-else class="list">
      <li v-for="item in items" :key="item.id" class="row">
        <div class="row-main">
          <div class="row-head">
            <span class="name">{{ item.name }}</span
            ><code>{{ item.slug }}</code>
          </div>
          <p class="usage">{{ item.usage_count ?? 0 }} 篇文章正在使用</p>
        </div>
        <div class="row-actions">
          <template v-if="confirmingDelete === item.id">
            <span class="confirm-text">确认删除「{{ item.name }}」？</span>
            <UiButton
              variant="danger"
              :disabled="isDeleting"
              @click="void handleDelete(item)"
              >{{ isDeleting ? "删除中…" : "确认删除" }}</UiButton
            >
            <UiButton :disabled="isDeleting" @click="confirmingDelete = null"
              >取消</UiButton
            >
          </template>
          <template v-else>
            <UiButton @click="openEdit(item)">编辑</UiButton>
            <UiButton
              variant="quiet"
              :disabled="Boolean(item.usage_count)"
              :title="
                item.usage_count
                  ? '标签仍被文章使用，移除关联后才能删除'
                  : undefined
              "
              @click="confirmingDelete = item.id"
            >
              {{ item.usage_count ? "使用中不可删除" : "删除" }}
            </UiButton>
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
  margin-bottom: var(--space-6);
}
.head-copy {
  min-width: 0;
}
.title {
  margin: 0 0 var(--space-2);
  font-family: var(--font-heading);
  font-size: clamp(1.5rem, 3vw, 2rem);
  font-weight: 600;
  letter-spacing: -0.02em;
}
.subtitle,
.hint,
.usage {
  color: var(--c-ink-muted);
  font-size: 0.875rem;
}
.button-icon {
  width: 1rem;
  height: 1rem;
}
.search,
.form {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin-bottom: var(--space-5);
}
.search {
  align-items: center;
  justify-content: space-between;
}
.search-input {
  flex: 1 1 16rem;
  max-width: 24rem;
}
.form label {
  display: grid;
  gap: var(--space-1);
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}
input {
  min-height: 2.5rem;
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-control);
  background: var(--c-surface);
  color: var(--c-ink);
}
.form {
  display: block;
  padding: var(--space-5);
  border: 1px solid var(--c-glass-border);
  border-radius: var(--radius-surface);
  background: var(--c-glass);
  box-shadow: var(--shadow-control);
}
.form h2 {
  margin-bottom: var(--space-4);
  font-size: 1rem;
}
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-4);
}
.form-grid input {
  width: 100%;
  margin-top: var(--space-1);
}
.hint {
  margin: var(--space-3) 0;
  font-size: 0.8125rem;
}
.actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin-top: var(--space-4);
}
.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
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
}
.name {
  font-size: 0.9375rem;
  font-weight: 500;
}
code {
  color: var(--c-ink-muted);
  font-family: var(--font-mono);
  font-size: 0.8125rem;
}
.usage {
  margin-top: var(--space-1);
  font-size: 0.8125rem;
}
.confirm-text {
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}
@media (max-width: 40rem) {
  .head,
  .row {
    flex-direction: column;
  }
  .form-grid {
    grid-template-columns: 1fr;
  }
  .row-actions {
    width: 100%;
  }
}
</style>
