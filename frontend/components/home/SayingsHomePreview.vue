<script setup lang="ts">
import { computed } from 'vue'

import type { SayingListItem } from '~/types'
import { markdownToText } from '~/utils/markdown'

const props = defineProps<{
  items: SayingListItem[]
}>()

const previewItems = computed(() => props.items.slice(0, 3))
</script>

<template>
  <section class="preview" aria-labelledby="sayings-preview">
    <div class="section-head">
      <h2 id="sayings-preview">片语</h2>
      <NuxtLink to="/sayings">浏览全部</NuxtLink>
    </div>

    <div class="stack">
      <div
        v-for="item in previewItems"
        :key="item.short_id"
        class="item"
      >
        {{ markdownToText(item.content_md, 70) }}
      </div>
    </div>
  </section>
</template>

<style scoped>
.preview {
  display: grid;
  gap: var(--space-5);
}

.section-head {
  position: relative;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-4);
  padding-bottom: var(--space-3);
  border-bottom: 1px solid var(--c-line);
}

.section-head::after {
  position: absolute;
  bottom: -1px;
  left: 0;
  width: var(--space-7);
  height: 2px;
  background: var(--c-accent);
  content: '';
}

.section-head h2 {
  color: var(--c-ink);
  font-size: var(--text-xl);
  letter-spacing: var(--tracking-display);
}

.section-head a {
  color: var(--c-ink-muted);
  font-size: var(--text-sm);
  text-decoration: none;
}

.section-head a:hover {
  color: var(--c-accent);
}

.stack {
  display: grid;
  gap: var(--space-3);
}

.item {
  width: 100%;
  padding-block: var(--space-3);
  border-inline: 0;
  border-top: 0;
  border-bottom: 1px solid var(--c-line);
  background: none;
  color: inherit;
  line-height: 1.7;
  text-align: left;
}
</style>
