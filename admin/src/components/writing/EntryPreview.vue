<script setup lang="ts">
import { computed, ref } from 'vue'

import { renderMarkdown } from '../../editor/render-preview'

const props = defineProps<{
  open: boolean
  title: string
  contentMd: string
}>()

const emit = defineEmits<{ 'update:open': [boolean] }>()
const mobile = ref(false)
const html = computed(() => renderMarkdown(props.contentMd))
</script>

<template>
  <aside v-if="open" class="entry-preview" role="dialog" aria-labelledby="preview-title">
    <div class="entry-preview__header">
      <h2 id="preview-title">预览</h2>
      <div>
        <button type="button" :aria-pressed="!mobile" data-preview-desktop @click="mobile = false">桌面</button>
        <button type="button" :aria-pressed="mobile" data-preview-mobile @click="mobile = true">手机</button>
        <button type="button" aria-label="关闭预览" data-preview-close @click="emit('update:open', false)">×</button>
      </div>
    </div>
    <div class="entry-preview__frame" :class="{ 'is-mobile': mobile }" data-preview-frame>
      <article class="prose">
        <h1 data-preview-title>{{ title || '无标题草稿' }}</h1>
        <div data-preview-content v-html="html" />
      </article>
    </div>
  </aside>
</template>

<style scoped>
.entry-preview { position: fixed; inset: 0; z-index: 40; padding: var(--space-5); overflow: auto; background: var(--c-surface-sunken); }
.entry-preview__header { display: flex; justify-content: space-between; align-items: center; max-width: 52rem; margin: 0 auto var(--space-4); }
.entry-preview__header h2 { margin: 0; font-size: 1rem; }
.entry-preview__header button { margin-inline-start: var(--space-2); }
.entry-preview__frame { max-width: 52rem; margin: 0 auto; padding: var(--space-6); border: 1px solid var(--c-line); background: var(--c-surface); transition: max-width 120ms ease; }
.entry-preview__frame.is-mobile { max-width: 24rem; }
.prose { max-width: 38rem; margin: 0 auto; line-height: 1.8; }
.prose h1 { line-height: 1.2; }
.prose :deep(.table-scroll) { overflow-x: auto; }
@media (prefers-reduced-motion: reduce) { .entry-preview__frame { transition: none; } }
</style>
