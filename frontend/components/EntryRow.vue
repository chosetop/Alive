<script setup lang="ts">
import type { EntryListItem } from '~/types'
import { resolvePublicWorld } from '~/content-worlds/registry'
import { entryDate, formatMonthDay, toDateAttribute } from '~/utils/date'

/**
 * One entry in the list.
 *
 * Journal is the only public world in this stage, so the row stays in the
 * current immersive, text-first style.
 */

const props = defineProps<{ entry: EntryListItem; index?: number }>()

const world = computed(() => resolvePublicWorld(props.entry.world))

const date = computed(() => entryDate(props.entry))

const href = computed(() => world.value?.entryPath(props.entry.slug) ?? `/${props.entry.slug}`)
</script>

<template>
  <article class="item" :data-chapter="props.index !== undefined ? `Chapter ${String(props.index + 1).padStart(2, '0')}` : undefined">
    <div class="body">
      <div class="text">
        <h3 class="title">
          <NuxtLink :to="href">{{ entry.title }}</NuxtLink>
        </h3>

        <p v-if="entry.summary" class="summary">{{ entry.summary }}</p>

        <div class="meta">
          <!-- The date is first: it is what the timeline is built on. -->
          <time v-if="date" class="date" :datetime="toDateAttribute(date)">
            {{ formatMonthDay(date) }}
          </time>
          <span class="type">{{ world?.label ?? '日志' }}</span>
          <NuxtLink
            v-if="entry.category"
            class="cat"
            :to="world?.categoryPath(entry.category.slug) ?? `/journal/categories/${entry.category.slug}`"
          >
            {{ entry.category.name }}
          </NuxtLink>
        </div>
      </div>
    </div>
  </article>
</template>

<style scoped>
.item {
  position: relative;
  padding-block: var(--space-6);
}

/*
 * A hairline between items instead of a card each. `:not(:last-child)` rather
 * than a top border on siblings, so the list never ends on a rule that has
 * nothing beneath it.
 */
.item:not(:last-child) {
  border-bottom: 1px solid var(--c-line);
}

.body {
  display: flex;
  gap: var(--space-4);
}

.text {
  /* Without this a long title stops the flex item from shrinking. */
  min-width: 0;
}

/* --------------------------------------------------------------------- text */

.title {
  font-size: var(--text-lg);
  line-height: var(--leading-tight);
}

.title a {
  text-decoration: none;
}

/*
 * The interaction for text-only entries: an ink rule grows from the left as the
 * title is approached. It replaces the underline a link would normally show,
 * which is why the anchor above drops its own decoration.
 */
.title a::after {
  content: '';
  display: block;
  width: 0;
  height: 1px;
  margin-top: 0.15em;
  background: var(--c-accent);
  transition: width var(--duration-mid) var(--ease-out);
}

.item:hover .title a::after,
.title a:focus-visible::after {
  width: 2.5rem;
}

.summary {
  margin-top: var(--space-2);
  color: var(--c-ink-muted);
  font-size: var(--text-sm);
  /* Two lines is enough to judge whether to open it. */
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
}

/*
 * Metadata is interface, not prose, so it uses the UI face. Mixing kaishu into
 * small labels makes them hard to read at this size.
 */
.meta {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-top: var(--space-3);
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
  line-height: var(--leading-none);
}

.date {
  /* Aligns the dates down the column even at proportional widths. */
  font-variant-numeric: tabular-nums;
}

.cat {
  color: var(--c-ink-muted);
  text-decoration: none;
}

.cat:hover {
  color: var(--c-accent);
}

/* --------------------------------------------------------------- responsive */

@media (max-width: 34rem) {
  .item {
    padding-block: var(--space-5);
  }

  .title {
    font-size: var(--text-base);
  }
}
</style>
