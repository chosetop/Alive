<script setup lang="ts">
import type { EntryListItem } from '~/types'
import { entryTypeStyle } from '~/composables/useEntryType'
import { entryDate, formatMonthDay, toDateAttribute } from '~/utils/date'

/**
 * One entry in the list.
 *
 * The layout is chosen by `type` rather than being uniform. See
 * `composables/useEntryType.ts` for the table of rules and why.
 */

const props = defineProps<{ entry: EntryListItem }>()

const style = computed(() => entryTypeStyle(props.entry.type))

/** Absent covers fall back to the type's text-only layout. */
const cover = computed(() => {
  if (props.entry.cover_url === '') return 'none'
  return style.value.cover
})

const date = computed(() => entryDate(props.entry))

/** Entries address by slug, at the root. Decided 2026-08-25, see progress.md. */
const href = computed(() => `/${props.entry.slug}`)
</script>

<template>
  <article class="item" :class="`item--${cover}`">
    <!--
      The image is a link too, but it carries aria-hidden and no text: the title
      link below already names the destination, and two links to the same place
      make a screen reader announce it twice.
    -->
    <NuxtLink v-if="cover === 'lead'" :to="href" class="lead" aria-hidden="true" tabindex="-1">
      <img :src="entry.cover_url" :alt="''" loading="lazy" decoding="async" />
    </NuxtLink>

    <div class="body">
      <NuxtLink v-if="cover === 'inline'" :to="href" class="thumb" aria-hidden="true" tabindex="-1">
        <img :src="entry.cover_url" :alt="''" loading="lazy" decoding="async" />
      </NuxtLink>

      <div class="text">
        <h3 class="title">
          <NuxtLink :to="href">{{ entry.title }}</NuxtLink>
        </h3>

        <p v-if="style.showSummary && entry.summary" class="summary">{{ entry.summary }}</p>

        <div class="meta">
          <!-- The date is first: it is what the timeline is built on. -->
          <time v-if="date" class="date" :datetime="toDateAttribute(date)">
            {{ formatMonthDay(date) }}
          </time>
          <span class="type">{{ style.label }}</span>
          <NuxtLink v-if="entry.category" class="cat" :to="`/categories/${entry.category.slug}`">
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

/* ------------------------------------------------------------------- images */

.lead {
  display: block;
  margin-bottom: var(--space-4);
  overflow: hidden;
  border-radius: 2px;
}

.lead img {
  width: 100%;
  /*
   * A fixed ratio, not the image's own: a list of photos with mixed ratios
   * turns into a ragged column. 3:2 is the common photographic frame.
   */
  aspect-ratio: 3 / 2;
  object-fit: cover;
  transition: scale var(--duration-mid) var(--ease-out);
}

/* The signature interaction for image-led entries. */
.item:hover .lead img {
  scale: 1.02;
}

.body {
  display: flex;
  gap: var(--space-4);
}

.thumb {
  flex: none;
  width: 4.5rem;
  overflow: hidden;
  border-radius: 2px;
  box-shadow: var(--shadow-image);
}

.thumb img {
  width: 100%;
  /* Portrait, because these are book jackets and film posters. */
  aspect-ratio: 2 / 3;
  object-fit: cover;
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

  /* The thumbnail costs more in width than it returns on a narrow screen. */
  .thumb {
    width: 3.5rem;
  }

  .title {
    font-size: var(--text-base);
  }
}
</style>
