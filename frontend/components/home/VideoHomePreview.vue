<script setup lang="ts">
import { computed } from 'vue'
import type { VideoListItem } from '~/types'

const props = defineProps<{ items: VideoListItem[] }>()
const previewItems = computed(() => props.items.slice(0, 3))
</script>

<template>
  <section class="preview" aria-labelledby="videos-preview">
    <div class="section-head">
      <h2 id="videos-preview">影像</h2>
      <NuxtLink to="/videos">浏览全部</NuxtLink>
    </div>

    <div class="strip">
      <NuxtLink
        v-for="item in previewItems"
        :key="item.slug"
        :to="`/videos/${item.slug}`"
        class="item"
      >
        <img v-if="item.cover_url" :src="item.cover_url" :alt="`${item.title}封面`" loading="lazy">
        <span class="title">{{ item.title }}</span>
      </NuxtLink>
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

.strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-4);
}

.item {
  display: grid;
  gap: var(--space-2);
  min-width: 0;
  color: inherit;
  text-decoration: none;
}

.item img {
  width: 100%;
  aspect-ratio: 16 / 9;
  object-fit: cover;
  border-radius: var(--radius-control);
  background: var(--c-surface-sunken);
}

.title {
  line-height: var(--leading-tight);
}

.item:hover .title {
  color: var(--c-accent);
}

@media (max-width: 34rem) {
  .strip {
    grid-template-columns: 1fr;
  }
}
</style>
