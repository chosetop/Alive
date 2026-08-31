<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import { entriesApi, toUserMessage } from '../api'
import TagPicker from '../components/writing/TagPicker.vue'
import type { Tag } from '../api/tags'
import type { EntryDetail, EntryVisibility } from '../types/api'

const router = useRouter()
const entry = ref<EntryDetail | null>(null)
const content = ref('')
const source = ref('')
const author = ref('')
const visibility = ref<EntryVisibility>('public')
const isSaving = ref(false)
const isPublishing = ref(false)
const error = ref<string | null>(null)
const tags = ref<Tag[]>([])
const copied = ref(false)
const isTouched = computed(() => content.value.trim() !== '' || source.value.trim() !== '' || author.value.trim() !== '')
let creating: Promise<void> | null = null

const longFormWarning = computed(() => Array.from(content.value.trim()).length > 300)

async function createDraft(): Promise<void> {
  if (entry.value || creating !== null) return creating ?? Promise.resolve()
  creating = (async () => {
  try {
    entry.value = await entriesApi.createEntry({ world: 'saying', visibility: 'public' })
  } catch (cause) {
    error.value = toUserMessage(cause)
  } finally {
    creating = null
  }
  })()
  return creating
}

async function save(): Promise<void> {
  if (!entry.value && isTouched.value) await createDraft()
  if (!entry.value || isSaving.value) return
  isSaving.value = true
  error.value = null
  try {
    entry.value = await entriesApi.updateEntry(entry.value.id, {
      revision: entry.value.revision,
      content_md: content.value,
      visibility: visibility.value,
      meta: { source: source.value.trim(), author: author.value.trim() },
    })
  } catch (cause) {
    error.value = toUserMessage(cause)
  } finally {
    isSaving.value = false
  }
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

async function leave(): Promise<void> {
  if (!isTouched.value) await router.push({ name: 'entry-new' })
  else {
    await save()
    if (!error.value) await router.push({ name: 'entry-new' })
  }
}

async function copyPermalink(): Promise<void> {
  if (!entry.value?.slug) return
  await navigator.clipboard.writeText(`${window.location.origin}/sayings/${entry.value.slug}`)
  copied.value = true
  window.setTimeout(() => { copied.value = false }, 1600)
}

</script>

<template>
  <main class="saying-editor">
    <header class="header">
      <button type="button" class="back" @click="leave">返回</button>
      <span class="eyebrow">片语</span>
      <span v-if="entry" class="status">{{ entry.status === 'published' ? '已发布' : '草稿' }}</span>
    </header>

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
      <button type="button" :disabled="isSaving || !entry" @click="save">{{ isSaving ? '保存中…' : '保存草稿' }}</button>
      <button type="button" :disabled="isPublishing || !entry || !content.trim()" @click="publish">发布</button>
    </footer>
  </main>
</template>

<style scoped>
.saying-editor { width: min(100%, 52rem); margin: 0 auto; padding: 2rem 1.5rem 4rem; }
.header, .actions, .meta { display: flex; align-items: center; gap: 0.75rem; }
.header { justify-content: space-between; margin-bottom: 2rem; }
.eyebrow { font-weight: 700; letter-spacing: .08em; }
.status { color: var(--c-ink-muted); font-size: .875rem; }
.back, button, input, select { border: 1px solid var(--c-border); border-radius: .75rem; background: var(--c-surface); padding: .65rem .8rem; }
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
