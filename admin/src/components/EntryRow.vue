<script setup lang="ts">
import { computed } from 'vue'
import type { EntryListItem, WorldKey } from '../types/api'
import { UiIcon } from './ui'

/**
 * One row of the article list.
 *
 * Read-only for now. Publish, unpublish, archive and delete are all real
 * endpoints, but wiring them here before the editor exists would give the
 * screen buttons that change state with no way to look at what changed first.
 */

const props = defineProps<{ entry: EntryListItem }>()

const STATUS_LABEL: Record<EntryListItem['status'], string> = {
  draft: '草稿',
  published: '已发布',
  archived: '已归档',
}

const WORLD_LABEL: Record<WorldKey, string> = {
  journal: '日志',
  saying: '片语',
  video: '影像',
}

/**
 * Visibility is shown only when it is not `public`.
 *
 * `public` is the default and the overwhelming majority, so labelling it would
 * put a word on every row that carries no information. `unlisted` and `private`
 * are the ones worth seeing at a glance.
 */
const visibilityLabel = computed(() => {
  switch (props.entry.visibility) {
    case 'unlisted':
      return '不列出'
    case 'private':
      return '私密'
    default:
      return null
  }
})

/**
 * The one date that matters per row, and which date that is depends on state:
 * a published entry is defined by when it went out, an unpublished one by when
 * it was last touched.
 */
const timeLabel = computed(() => {
  if (props.entry.published_at !== null) {
    return { prefix: '发布于', value: formatDate(props.entry.published_at) }
  }
  return { prefix: '编辑于', value: formatDate(props.entry.updated_at) }
})

/**
 * Local date and time, no seconds.
 *
 * The API sends RFC 3339 with an offset, so `Date` parses it correctly and
 * renders in the reader's own zone rather than the server's.
 */
function formatDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <li class="row">
    <div class="main">
      <div class="head">
        <RouterLink class="title" :to="{ name: 'entry-edit', params: { id: entry.id } }">
          <span>{{ entry.title }}</span><UiIcon class="title-icon" name="chevron-right" />
        </RouterLink>
        <span class="badge" :class="`badge--${entry.status}`" :data-status="entry.status">{{
          STATUS_LABEL[entry.status]
        }}</span>
        <span v-if="visibilityLabel" class="badge badge--muted">{{ visibilityLabel }}</span>
      </div>

      <p v-if="entry.summary" class="summary">{{ entry.summary }}</p>

      <div class="meta">
        <span class="type">{{ WORLD_LABEL[entry.world] }}</span>
        <code class="slug">{{ entry.slug }}</code>
        <!-- Uncategorised is a normal state, so it is stated rather than left
             blank, which would read as a rendering gap. -->
        <span v-if="entry.category" class="cat">{{ entry.category.name }}</span>
        <span v-else class="cat cat--none">未分类</span>
        <span class="words">{{ entry.word_count }} 字</span>
        <span class="time">{{ timeLabel.prefix }} {{ timeLabel.value }}</span>
      </div>
    </div>
  </li>
</template>

<style scoped>
.row {
  padding: var(--space-4);
  transition: background-color var(--motion-fast) ease;
}

.row + .row {
  border-top: 1px solid var(--c-line);
}

.main {
  min-width: 0;
}

.head {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-bottom: var(--space-1);
}

.title {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  color: var(--c-ink);
  font-size: 0.9375rem;
  font-weight: 500;
}

.title:hover {
  color: var(--c-accent);
  text-decoration: none;
}

.title-icon { width: 0.875rem; height: 0.875rem; }

.badge {
  padding: 0.0625rem 0.375rem;
  border: 1px solid var(--c-glass-border);
  border-radius: var(--radius-control);
  background: var(--c-surface-sunken);
  color: var(--c-ink-muted);
  font-size: 0.6875rem;
  white-space: nowrap;
}

/* A draft is the one state that wants attention, so it is the one that gets
   colour. Published is the resting state and archived is deliberately quiet. */
.badge--draft {
  border-color: var(--c-success);
  background: var(--c-success-surface);
  color: var(--c-success);
}

.badge--archived {
  color: var(--c-ink-faint);
}

.badge--muted {
  color: var(--c-ink-faint);
}

.summary {
  margin-bottom: var(--space-2);
  color: var(--c-ink-muted);
  font-size: 0.875rem;
}

.meta {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: var(--space-3);
  color: var(--c-ink-faint);
  font-size: 0.75rem;
}

.slug {
  font-family: var(--font-mono);
}

.cat--none {
  font-style: italic;
}
</style>
