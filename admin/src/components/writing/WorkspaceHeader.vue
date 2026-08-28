<script setup lang="ts">
import { computed, ref } from 'vue'

import type { SaveStatus } from '../../editor/save-coordinator'
import { useWritingStore } from '../../stores/writing'
import type { EntryStatus } from '../../types/api'
import { UiButton, UiIconButton } from '../ui'

/**
 * The workspace header.
 *
 * The workspace actions stay in one fixed-height bar: directory toggle, save
 * state, settings, delete confirmation and publish.
 *
 * The save state sits in a fixed-height slot. Every status string here differs in
 * length ("已保存" against "离线，本地草稿已保留"), and the conflict state adds three
 * buttons; letting the slot size itself would shift the canvas under the cursor
 * each time a save resolved. A writer mid-sentence experiences that as the page
 * fighting them, and it would happen on the normal path, not an error path.
 */

const STATUS_TEXT: Record<SaveStatus, string> = {
  saved: '已保存',
  pending: '待保存',
  saving: '保存中…',
  offline: '离线，本地草稿已保留',
  error: '保存失败',
  conflict: '保存冲突，本地已保留',
}

const ENTRY_STATUS_TEXT: Record<EntryStatus, string> = {
  draft: '草稿',
  published: '已发布',
  archived: '已归档',
}

const props = withDefaults(
  defineProps<{
    saveStatus: SaveStatus
    /** Null before the record exists; publish and the menu are meaningless then. */
    entryStatus?: EntryStatus | null
    busy?: boolean
  }>(),
  { entryStatus: null, busy: false },
)

const emit = defineEmits<{
  publish: []
  delete: []
  retry: []
  action: [string]
}>()

const writing = useWritingStore()
const confirmingDelete = ref(false)

const statusText = computed(() => STATUS_TEXT[props.saveStatus])

/**
 * Only the two recoverable states get a retry. Offering it during `saving` would
 * let a second flush race the first, and during `conflict` the choice is not
 * "try again" -- it is which of two versions survives, which the editor owns.
 */
const canRetry = computed(() => props.saveStatus === 'offline' || props.saveStatus === 'error')

/**
 * Publish remains the only primary action. Deletion is visible for discoverability
 * but requires a second explicit confirmation before emitting to the editor.
 */
const publishLabel = computed(() =>
  props.entryStatus === 'published' ? '已发布' : '发布',
)
</script>

<template>
  <header class="header" data-workspace-header>
    <div class="left">
      <RouterLink class="brand-link" to="/dashboard" aria-label="返回 Dashboard" data-writing-brand>Alive</RouterLink>
      <!-- Rendered only while the directory is hidden. Its counterpart lives in
           the directory's own header, so the control is always beside the thing
           it acts on rather than in a fixed spot the pane may have covered. -->
      <UiIconButton
        v-if="!writing.directoryOpen"
        label="展开文章目录"
        data-directory-expand
        @click="writing.setDirectoryOpen(true)"
      >
        <span aria-hidden="true">⟩</span>
      </UiIconButton>
    </div>

    <!--
      aria-live="polite" on a slot that is always mounted. An element that appears
      only when the status changes is not observed by a screen reader in time, so
      the region has to exist before it has anything to announce.
    -->
    <div class="status" data-save-status aria-live="polite">
      <span class="status-text" :data-status="saveStatus">{{ statusText }}</span>
      <button
        v-if="canRetry"
        class="status-action"
        type="button"
        data-save-retry
        @click="emit('retry')"
      >
        重试
      </button>
      <!-- The conflict choices are the editor's: only it knows what the local
           and server versions contain. The header leaves room for them. -->
      <span v-else-if="saveStatus === 'conflict'" class="status-slot">
        <slot name="conflict" />
      </span>
    </div>

    <div class="right">
      <span v-if="entryStatus" class="entry-status">{{ ENTRY_STATUS_TEXT[entryStatus] }}</span>

      <template v-if="entryStatus !== null">
        <UiButton variant="quiet" data-header-settings @click="emit('action', 'settings')">设置</UiButton>
        <UiButton v-if="entryStatus === 'published'" variant="quiet" data-header-unpublish @click="emit('action', 'unpublish')">
          撤回
        </UiButton>
        <UiButton v-if="entryStatus !== 'archived'" variant="quiet" data-header-archive @click="emit('action', 'archive')">
          归档
        </UiButton>
      </template>

      <template v-if="entryStatus !== null && !confirmingDelete">
        <UiButton variant="quiet" data-header-delete @click="confirmingDelete = true">删除</UiButton>
      </template>
      <template v-else-if="entryStatus !== null">
        <span class="delete-confirm-text">确认删除？</span>
        <UiButton variant="danger" :disabled="busy" data-header-delete-confirm @click="emit('delete'); confirmingDelete = false">
          {{ busy ? '删除中…' : '确认删除' }}
        </UiButton>
        <UiButton variant="quiet" :disabled="busy" data-header-delete-cancel @click="confirmingDelete = false">
          取消
        </UiButton>
      </template>

      <UiButton
        v-if="entryStatus !== null"
        variant="primary"
        :disabled="busy || entryStatus === 'published'"
        data-publish
        @click="emit('publish')"
      >
        {{ publishLabel }}
      </UiButton>
    </div>
  </header>
</template>

<style scoped>
.header {
  display: grid;
  /* Three tracks with the middle one taking the slack, so the status sits in the
     optical centre of the canvas regardless of how wide the action cluster grows.
     A flex row with margin auto would recentre every time an action appeared. */
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  align-items: center;
  gap: var(--space-3);
  height: var(--header-height);
  flex-shrink: 0;
  padding: 0 var(--space-4);
  border-bottom: 1px solid var(--c-line);
  background: var(--c-surface);
}

.left {
  display: flex;
  align-items: center;
  /* Reserved whether or not the toggle is rendered: collapsing this track would
     shift the whole bar sideways the moment the directory opened. */
  min-height: 1.75rem;
}

.brand-link {
  color: var(--c-ink);
  font-size: 0.875rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  text-decoration: none;
}

.brand-link:hover,
.brand-link:focus-visible {
  color: var(--c-accent);
}

/**
 * The fixed-height status slot.
 *
 * Every state here is a different length, and the conflict state adds three
 * buttons. A slot that sized itself would move the canvas under the cursor each
 * time a save resolved -- on the normal path, not an error path. The height is
 * pinned and the content is clipped rather than allowed to wrap.
 */
.status {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  height: 1.75rem;
  overflow: hidden;
  white-space: nowrap;
}

.status-text {
  color: var(--c-ink-muted);
  font-size: 0.75rem;
}

/* Weight and colour together. Colour alone would not survive greyscale, and the
   text itself already names the state. */
.status-text[data-status='error'],
.status-text[data-status='conflict'] {
  color: var(--c-danger);
  font-weight: 500;
}

.status-slot {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.status-action {
  padding: 0 var(--space-1);
  border: none;
  background: transparent;
  color: var(--c-accent);
  font: inherit;
  font-size: 0.75rem;
  cursor: pointer;
}

.right {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-2);
}

.entry-status {
  color: var(--c-ink-faint);
  font-size: 0.75rem;
}

.delete-confirm-text {
  color: var(--c-danger);
  font-size: 0.75rem;
  white-space: nowrap;
}

@media (max-width: 48rem) {
  .header {
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-areas:
      'left right'
      'status status';
    height: auto;
    min-height: var(--header-height);
    row-gap: var(--space-2);
    padding: 0 var(--space-3);
  }

  .left {
    grid-area: left;
  }

  .status {
    grid-area: status;
    justify-content: flex-start;
    height: auto;
    min-height: 1.75rem;
    overflow: visible;
    flex-wrap: wrap;
  }

  .status-slot {
    min-width: 0;
    flex-wrap: wrap;
  }

  .right {
    grid-area: right;
    min-width: 0;
  }

  /* The word is redundant next to a publish button that already reads 已发布. */
  .entry-status {
    display: none;
  }
}
</style>
