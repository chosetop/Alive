<script setup lang="ts">
import { computed } from 'vue'

import { resolvePublicWorld } from '~/content-worlds/registry'
import type { SayingListItem } from '~/types'
import { renderMarkdown } from '~/utils/markdown'

const props = defineProps<{
  item: SayingListItem
  compact?: boolean
}>()

const world = resolvePublicWorld('saying')

const body = computed(() => renderMarkdown(props.item.content_md))
const categoryPath = computed(() =>
  props.item.category ? world?.categoryPath(props.item.category.slug) ?? `/sayings/categories/${props.item.category.slug}` : null,
)
</script>

<template>
  <article
    class="saying"
    :class="{ 'saying--compact': compact }"
    :data-saying-id="item.short_id"
    tabindex="-1"
  >
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div class="body prose" v-html="body" />

    <footer v-if="item.source || item.author || item.category" class="meta">
      <p v-if="item.source || item.author" class="credit">
        <span v-if="item.source">{{ item.source }}</span>
        <span v-if="item.source && item.author"> · </span>
        <span v-if="item.author">{{ item.author }}</span>
      </p>

      <NuxtLink v-if="item.category && categoryPath" class="category" :to="categoryPath">
        {{ item.category.name }}
      </NuxtLink>
    </footer>
  </article>
</template>

<style scoped>
.saying {
  min-width: 0;
}

.saying--compact .body {
  font-size: var(--text-base);
  line-height: 1.75;
}

.body {
  color: var(--c-ink);
  font-size: var(--text-lg);
  line-height: 1.85;
  white-space: normal;
}

.meta {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-top: var(--space-3);
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
}

.credit {
  margin: 0;
}

.category {
  color: var(--c-ink-muted);
  text-decoration: none;
}

.category:hover {
  color: var(--c-accent);
}
</style>
