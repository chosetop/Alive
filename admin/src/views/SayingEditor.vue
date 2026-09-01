<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'

import { entriesApi, toUserMessage } from '../api'
import { resolveAdminWorld } from '../content-worlds/registry'
import TagPicker from '../components/writing/TagPicker.vue'
import WorkspaceHeader from '../components/writing/WorkspaceHeader.vue'
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
const source = ref('')
const author = ref('')
const visibility = ref<EntryVisibility>('public')
const isSaving = ref(false)
const isCreating = ref(false)
const isPublishing = ref(false)
const error = ref<string | null>(null)
const tags = ref<Tag[]>([])
const copied = ref(false)
const sayingWorldLabel = resolveAdminWorld('saying')?.label ?? '片语'
const isTouched = computed(() => content.value.trim() !== '' || source.value.trim() !== '' || author.value.trim() !== '')
const hasUnsavedChanges = computed(() => {
  if (!entry.value) return isTouched.value
  return content.value !== entry.value.content_md
    || visibility.value !== entry.value.visibility
    || source.value.trim() !== (typeof entry.value.meta.source === 'string' ? entry.value.meta.source : '')
    || author.value.trim() !== (typeof entry.value.meta.author === 'string' ? entry.value.meta.author : '')
})
let creating: Promise<void> | null = null
let saving: Promise<void> | null = null

const longFormWarning = computed(() => Array.from(content.value.trim()).length > 300)
const saveStatus = computed<SaveStatus>(() => {
  if (error.value !== null) return 'error'
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
  if (flushGate !== null && flushGate.value === ownFlush) flushGate.value = null
  if (writing.activeEntryId === entry.value?.id) writing.setActiveEntry(null)
})

function applyEntry(next: EntryDetail): void {
  entry.value = next
  writing.setActiveEntry(next.id)
  content.value = next.content_md
  visibility.value = next.visibility
  source.value = typeof next.meta.source === 'string' ? next.meta.source : ''
  author.value = typeof next.meta.author === 'string' ? next.meta.author : ''
  tags.value = next.tags ?? []
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
  saving = (async () => {
    try {
      entry.value = await entriesApi.updateEntry(currentEntry.id, {
        revision: currentEntry.revision,
        content_md: content.value,
        visibility: visibility.value,
        meta: { source: source.value.trim(), author: author.value.trim() },
      })
    } catch (cause) {
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
      @retry="void save()"
    />

    <main class="saying-editor">
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <textarea v-model="content" autofocus class="body" placeholder="写下一句随口的话……" aria-label="片语正文" @input="void createDraft()" />
      <p v-if="longFormWarning" class="warning">这段话已经接近一篇日志。</p>

      <section class="meta" aria-label="片语信息">
        <input v-model="source" placeholder="来源（可选）" aria-label="来源" />
        <input v-model="author" placeholder="原作者（可选）" aria-label="原作者" />
        <select v-model="visibility" aria-label="可见性">
          <option value="public">公开</option>
          <option value="unlisted">不列出</option>
          <option value="private">私密</option>
        </select>
      </section>
      <p class="permalink-hint">发布后会生成永久链接；“不列出”不会出现在列表，但知道链接的人仍可访问。</p>
      <div v-if="entry?.status === 'published' && entry.slug" class="permalink" data-permalink>
        <code>/sayings/{{ entry.slug }}</code>
        <button type="button" @click="copyPermalink">{{ copied ? '已复制' : '复制链接' }}</button>
      </div>

      <TagPicker
        v-if="entry"
        :entry-id="entry.id"
        :revision="entry.revision"
        :selected="tags"
        @saved="(revision, nextTags) => { if (entry) { entry.revision = revision; tags = nextTags } }"
      />

      <footer class="actions">
        <button type="button" :disabled="isSaving || isCreating || isPublishing || !entry" @click="save">{{ isSaving ? '保存中…' : '保存草稿' }}</button>
        <button type="button" :disabled="isPublishing || !entry || !content.trim()" @click="publish">发布</button>
      </footer>
    </main>
  </div>
</template>

<style scoped>
.saying-shell { display: flex; min-height: 100%; flex-direction: column; }
.saying-editor { width: min(100%, 52rem); margin: 0 auto; padding: 2rem 1.5rem 4rem; }
.actions, .meta { display: flex; align-items: center; gap: 0.75rem; }
button, input, select { border: 1px solid var(--c-border); border-radius: .75rem; background: var(--c-surface); padding: .65rem .8rem; }
.body { display: block; width: 100%; min-height: 18rem; resize: vertical; border: 0; outline: 0; background: transparent; color: var(--c-ink); font: inherit; font-size: 1.5rem; line-height: 1.8; }
.meta { margin-top: 1.5rem; flex-wrap: wrap; }
.meta input { flex: 1 1 12rem; }
.actions { justify-content: flex-end; margin-top: 1.5rem; }
.warning { color: var(--c-ink-muted); font-size: .9rem; }
.permalink-hint { margin-top: 0.75rem; color: var(--c-ink-faint); font-size: .8125rem; line-height: 1.5; }
.permalink { display: flex; align-items: center; gap: .5rem; margin-top: .5rem; color: var(--c-ink-muted); font-size: .8125rem; }
.permalink code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.error { color: var(--c-danger, #b42318); }
</style>
