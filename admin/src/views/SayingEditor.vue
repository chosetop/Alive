<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'

import { entriesApi, toUserMessage } from '../api'
import { resolveAdminWorld } from '../content-worlds/registry'
import TagPicker from '../components/writing/TagPicker.vue'
import WorkspaceHeader from '../components/writing/WorkspaceHeader.vue'
import { defaultEntrySlug } from '../editor/entry-slug'
import type { Tag } from '../api/tags'
import type { SaveStatus } from '../editor/save-coordinator'
import type { EntryDetail, EntryVisibility } from '../types/api'
import { useWritingStore, writingFlushKey } from '../stores/writing'

const props = defineProps<{
  initialEntry?: EntryDetail
}>()

const writing = useWritingStore()
const entry = ref<EntryDetail | null>(null)
const content = ref('')
const slug = ref('')
const source = ref('')
const author = ref('')
const visibility = ref<EntryVisibility>('public')
const isSaving = ref(false)
const isCreating = ref(false)
const isPublishing = ref(false)
const error = ref<string | null>(null)
const errorSource = ref<'save' | 'publish' | null>(null)
const tags = ref<Tag[]>([])
const copied = ref(false)
const sayingWorldLabel = resolveAdminWorld('saying')?.label ?? '片语'
const isTouched = computed(() => content.value.trim() !== '' || source.value.trim() !== '' || author.value.trim() !== '')
const hasUnsavedChanges = computed(() => {
  if (!entry.value) return isTouched.value
  return content.value !== entry.value.content_md
    || slug.value !== entry.value.slug
    || visibility.value !== entry.value.visibility
    || source.value.trim() !== (typeof entry.value.meta.source === 'string' ? entry.value.meta.source : '')
    || author.value.trim() !== (typeof entry.value.meta.author === 'string' ? entry.value.meta.author : '')
})
let creating: Promise<void> | null = null
let saving: Promise<void> | null = null
let saveTimer: ReturnType<typeof setTimeout> | null = null

const longFormWarning = computed(() => Array.from(content.value.trim()).length > 300)
const saveStatus = computed<SaveStatus>(() => {
  if (error.value !== null && errorSource.value === 'save') return 'error'
  if (isSaving.value || isCreating.value) return 'saving'
  if (hasUnsavedChanges.value) return 'pending'
  return 'saved'
})
const flushGate = inject(writingFlushKey, null)
const ownFlush = async (): Promise<void> => {
  await save()
}

onMounted(() => {
  writing.setActiveWorld(props.initialEntry?.world ?? 'saying')
  if (flushGate !== null) flushGate.value = ownFlush
  if (props.initialEntry) applyEntry(props.initialEntry)
})

onBeforeUnmount(() => {
  if (saveTimer !== null) clearTimeout(saveTimer)
  if (flushGate !== null && flushGate.value === ownFlush) flushGate.value = null
  if (writing.activeEntryId === entry.value?.id) writing.setActiveEntry(null)
})

function scheduleSave(): void {
  if (saveTimer !== null) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    saveTimer = null
    void save()
  }, 650)
}

function handleBodyInput(): void {
  void createDraft()
  scheduleSave()
}

function applyEntry(next: EntryDetail): void {
  entry.value = next
  writing.setActiveEntry(next.id)
  content.value = next.content_md
  slug.value = next.slug || defaultEntrySlug(next.world, next.id)
  visibility.value = next.visibility
  source.value = typeof next.meta.source === 'string' ? next.meta.source : ''
  author.value = typeof next.meta.author === 'string' ? next.meta.author : ''
  tags.value = next.tags ?? []
  if (next.slug === '') scheduleSave()
}

async function createDraft(): Promise<void> {
  if (entry.value || creating !== null) return creating ?? Promise.resolve()
  creating = (async () => {
    isCreating.value = true
    const pendingDraft = {
      content: content.value,
      visibility: visibility.value,
      source: source.value,
      author: author.value,
    }
    try {
      const created = await entriesApi.createEntry({ world: 'saying', visibility: 'public' })
      applyEntry(created)
      // A fresh server draft is structural state only. The sentence currently on
      // screen is the writer's source of truth and must survive record creation.
      content.value = pendingDraft.content
      visibility.value = pendingDraft.visibility
      source.value = pendingDraft.source
      author.value = pendingDraft.author
    } catch (cause) {
      errorSource.value = 'save'
      error.value = toUserMessage(cause)
    } finally {
      isCreating.value = false
      creating = null
    }
  })()
  return creating
}

async function save(): Promise<void> {
  if (!entry.value && isTouched.value) await createDraft()
  if (!entry.value) return
  if (saving !== null) {
    await saving
    if (error.value === null && hasUnsavedChanges.value) await save()
    return
  }
  const currentEntry = entry.value
  isSaving.value = true
  error.value = null
  errorSource.value = null
  saving = (async () => {
    try {
      entry.value = await entriesApi.updateEntry(currentEntry.id, {
        revision: currentEntry.revision,
        slug: slug.value,
        content_md: content.value,
        visibility: visibility.value,
        meta: { source: source.value.trim(), author: author.value.trim() },
      })
    } catch (cause) {
      errorSource.value = 'save'
      error.value = toUserMessage(cause)
    } finally {
      isSaving.value = false
    }
  })()
  try {
    await saving
  } finally {
    saving = null
  }
}

async function flushBeforeRouteChange(): Promise<boolean> {
  if (saveTimer !== null) { clearTimeout(saveTimer); saveTimer = null }
  await save()
  return error.value === null
}

async function publish(): Promise<void> {
  if (!entry.value || isPublishing.value) return
  await save()
  if (!entry.value || error.value) return
  isPublishing.value = true
  try {
    entry.value = await entriesApi.publishEntry(entry.value.id, entry.value.revision)
  } catch (cause) {
    errorSource.value = 'publish'
    error.value = toUserMessage(cause)
  } finally {
    isPublishing.value = false
  }
}

async function copyPermalink(): Promise<void> {
  if (!entry.value?.slug) return
  await navigator.clipboard.writeText(`${window.location.origin}/sayings/${entry.value.slug}`)
  copied.value = true
  window.setTimeout(() => { copied.value = false }, 1600)
}

onBeforeRouteLeave(async () => ((await flushBeforeRouteChange()) ? undefined : false))
onBeforeRouteUpdate(async () => ((await flushBeforeRouteChange()) ? undefined : false))

</script>

<template>
  <div class="saying-shell">
    <WorkspaceHeader
      :entry-id="entry?.id ?? null"
      :world-label="sayingWorldLabel"
      :entry-status="entry?.status ?? null"
      :save-status="saveStatus"
      :show-actions="false"
      :show-publish="true"
      @retry="void save()"
      @publish="void publish()"
    />

    <main class="saying-editor">
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <section class="saying-note" data-saying-note>
        <textarea v-model="content" autofocus class="body" data-saying-body placeholder="写下一句随口的话……" aria-label="片语正文" @input="handleBodyInput" />
      </section>
      <p v-if="longFormWarning" class="warning">这段话已经接近一篇日志。</p>

      <section v-if="entry" class="saying-settings" data-saying-settings aria-label="片语设置">
        <h2>片语设置</h2>
        <label class="setting-field">
          <span>slug</span>
          <input v-model="slug" aria-label="片语 slug" placeholder="例如 one-line" @input="scheduleSave" />
        </label>
        <label class="setting-field">
          <span>公开状态</span>
          <select v-model="visibility" aria-label="公开状态" @change="scheduleSave">
            <option value="public">公开</option>
            <option value="unlisted">不列出</option>
            <option value="private">私密</option>
          </select>
        </label>
        <div class="setting-fields">
          <input v-model="source" placeholder="来源（可选）" aria-label="来源" @input="scheduleSave" />
          <input v-model="author" placeholder="原作者（可选）" aria-label="原作者" @input="scheduleSave" />
        </div>
        <TagPicker
          :entry-id="entry.id"
          :revision="entry.revision"
          :selected="tags"
          @saved="(revision, nextTags) => { if (entry) { entry.revision = revision; tags = nextTags } }"
        />
        <p class="permalink-hint">发布后会生成永久链接；“不列出”不会出现在列表，但知道链接的人仍可访问。</p>
        <div v-if="entry.status === 'published' && entry.slug" class="permalink" data-permalink>
          <code>/sayings/{{ entry.slug }}</code>
          <button type="button" @click="copyPermalink">{{ copied ? '已复制' : '复制链接' }}</button>
        </div>
      </section>

    </main>
  </div>
</template>

<style scoped>
.saying-shell { display: flex; min-height: 100%; flex-direction: column; }
.saying-editor {
  width: min(100%, 40rem);
  margin: 0 auto;
  padding: var(--space-6) var(--space-5) var(--space-8);
}

.setting-fields {
  display: flex;
  gap: var(--space-3);
}

button,
input,
select {
  min-height: 2.75rem;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  background: var(--c-surface);
  padding: 0.65rem 0.8rem;
}

.saying-note {
  padding: clamp(var(--space-5), 5vw, var(--space-6));
  border: 1px solid var(--c-line);
  border-radius: 1.125rem;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--c-paper) 97%, var(--c-accent) 3%), var(--c-paper));
  box-shadow: var(--shadow-float);
}

.body {
  display: block;
  width: 100%;
  min-height: 18rem;
  resize: vertical;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--c-ink);
  font-family: var(--font-heading);
  font-size: clamp(1.75rem, 4.5vw, 2.4rem);
  line-height: 1.9;
  text-align: center;
}

.saying-settings {
  display: grid;
  gap: var(--space-4);
  margin-top: var(--space-5);
  padding: var(--space-4);
  border: 1px solid var(--c-line);
  border-radius: var(--radius-surface);
  background: var(--c-surface-sunken);
}

.saying-settings h2 {
  margin: 0;
  font-size: 0.95rem;
}

.setting-field {
  display: grid;
  gap: var(--space-2);
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.setting-fields {
  flex-wrap: wrap;
}

.setting-fields input {
  flex: 1 1 12rem;
}

.warning {
  margin-top: var(--space-3);
  color: var(--c-ink-muted);
  font-size: 0.9rem;
}

.permalink-hint {
  margin-top: var(--space-3);
  color: var(--c-ink-faint);
  font-size: 0.8125rem;
  line-height: 1.5;
}

.permalink {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-top: var(--space-2);
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.permalink code {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.error {
  margin-bottom: var(--space-3);
  color: var(--c-danger);
}

@media (max-width: 48rem) {
  .saying-editor {
    width: 100%;
    padding: var(--space-5) var(--space-4) var(--space-6);
  }

  .saying-note {
    padding: var(--space-4);
    box-shadow: none;
  }

  .body {
    min-height: 14rem;
    font-size: clamp(1.4rem, 7vw, 1.9rem);
    text-align: left;
  }

  .permalink {
    flex-wrap: wrap;
    align-items: flex-start;
  }

  .setting-fields {
    display: grid;
  }
}
</style>
