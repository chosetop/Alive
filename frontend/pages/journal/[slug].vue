<script setup lang="ts">
import { resolvePublicWorld } from '~/content-worlds/registry'
import { entryDate, formatFullDate, toDateAttribute } from '~/utils/date'
import {
  getEntryNavigationPages,
  getEntryNeighborTitle,
  getEntryNeighbors,
} from '~/utils/entry-navigation'
import { markdownToText, renderMarkdown } from '~/utils/markdown'

/**
 * Journal detail page under the new canonical world path.
 */

const route = useRoute()
const { getBySlug, list } = useEntriesApi()
const journal = resolvePublicWorld('journal')

const slug = computed(() => String(route.params.slug))

const { data: entry, error } = await useAsyncData(
  () => `journal-entry-${slug.value}`,
  () => getBySlug(slug.value),
  { watch: [slug] },
)

if (error.value || !entry.value) {
  throw createError({ statusCode: 404, message: '没有这一篇', fatal: true })
}

const { data: navigationPage, error: navigationError } = await useAsyncData(
  () => `journal-entry-navigation-${slug.value}`,
  async () => {
    const firstPage = await list({ page: 1, page_size: 50 })
    const pages = getEntryNavigationPages(firstPage.meta.total, firstPage.meta.page_size)
    const remainingPages = await Promise.all(
      pages.slice(1).map((page) => list({ page, page_size: firstPage.meta.page_size })),
    )

    return {
      data: [firstPage, ...remainingPages].flatMap((page) => page.data),
      meta: firstPage.meta,
    }
  },
  { watch: [slug] },
)

const neighbors = computed(() =>
  getEntryNeighbors(navigationPage.value?.data ?? [], slug.value),
)

const navigationAvailable = computed(() => Boolean(navigationPage.value) && !navigationError.value)
const date = computed(() => entryDate(entry.value!))
const html = computed(() => renderMarkdown(entry.value!.content_md))
const description = computed(() => {
  const e = entry.value!
  return e.summary !== '' ? e.summary : markdownToText(e.content_md)
})
const canonical = computed(() => journal?.entryPath(entry.value!.slug) ?? `/journal/${entry.value!.slug}`)

useSeoMeta({
  title: () => `${entry.value!.title} · Alive`,
  description,
  ogTitle: () => entry.value!.title,
  ogDescription: description,
  ogType: 'article',
  ogImage: () => entry.value!.cover_url || undefined,
  twitterCard: () => (entry.value!.cover_url ? 'summary_large_image' : 'summary'),
  articlePublishedTime: () => entry.value!.published_at ?? undefined,
  articleModifiedTime: () => entry.value!.updated_at,
})

useHead({
  link: [{ rel: 'canonical', href: canonical }],
  script: [
    {
      type: 'application/ld+json',
      innerHTML: computed(() =>
        JSON.stringify({
          '@context': 'https://schema.org',
          '@type': 'BlogPosting',
          headline: entry.value!.title,
          description: description.value,
          datePublished: entry.value!.published_at ?? undefined,
          dateModified: entry.value!.updated_at,
          image: entry.value!.cover_url || undefined,
          articleSection: entry.value!.category?.name,
          wordCount: entry.value!.word_count,
        }),
      ),
    },
  ],
})
</script>

<template>
  <article v-if="entry" class="entry">
    <div class="back">
      <NuxtLink to="/journal">日志</NuxtLink>
      <span class="back__meta" aria-label="文章信息">
        <time v-if="date" class="date" :datetime="toDateAttribute(date)">{{ formatFullDate(date) }}</time>
        <NuxtLink
          v-if="entry.category"
          :to="journal?.categoryPath(entry.category.slug) ?? `/journal/categories/${entry.category.slug}`"
          class="cat"
        >
          {{ entry.category.name }}
        </NuxtLink>
        <span class="words">{{ entry.word_count }} 字</span>
      </span>
    </div>
    <header class="head">
      <h1 class="title">{{ entry.title }}</h1>
    </header>

    <!-- eslint-disable-next-line vue/no-v-html -->
    <div class="prose" v-html="html" />

    <footer class="foot">
      <nav class="neighbors" aria-label="文章导航">
        <NuxtLink
          v-if="neighbors.previous"
          class="neighbor neighbor--previous"
          :to="journal?.entryPath(neighbors.previous.slug) ?? `/journal/${neighbors.previous.slug}`"
          rel="prev"
        >
          <span class="neighbor__direction">上一篇</span>
          <span class="neighbor__title">{{ getEntryNeighborTitle('previous', neighbors.previous, navigationAvailable) }}</span>
        </NuxtLink>
        <span v-else class="neighbor neighbor--previous neighbor--unavailable" aria-disabled="true">
          <span class="neighbor__direction">上一篇</span>
          <span class="neighbor__title">{{ getEntryNeighborTitle('previous', neighbors.previous, navigationAvailable) }}</span>
        </span>
        <NuxtLink
          v-if="neighbors.next"
          class="neighbor neighbor--next"
          :to="journal?.entryPath(neighbors.next.slug) ?? `/journal/${neighbors.next.slug}`"
          rel="next"
        >
          <span class="neighbor__direction">下一篇</span>
          <span class="neighbor__title">{{ getEntryNeighborTitle('next', neighbors.next, navigationAvailable) }}</span>
        </NuxtLink>
        <span v-else class="neighbor neighbor--next neighbor--unavailable" aria-disabled="true">
          <span class="neighbor__direction">下一篇</span>
          <span class="neighbor__title">{{ getEntryNeighborTitle('next', neighbors.next, navigationAvailable) }}</span>
        </span>
      </nav>
    </footer>
  </article>
</template>

<style scoped>
.entry {
  max-width: 52rem;
  margin-inline: auto;
}

.back {
  display: inline-flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-bottom: var(--space-7);
  color: var(--c-ink-muted);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  text-decoration: none;
}

.back__meta {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: var(--space-2) var(--space-3);
  color: var(--c-ink-faint);
  font-size: var(--text-xs);
}

.back__meta .date {
  color: var(--c-ink-muted);
}

.back__meta .cat {
  color: var(--c-ink-faint);
}

.back > a:hover {
  color: var(--c-accent);
}

.head {
  display: grid;
  gap: var(--space-5);
  margin-bottom: var(--space-8);
}

.title {
  max-width: 38rem;
  font-size: clamp(1.85rem, 5vw, 3rem);
  line-height: 1.2;
}

.date {
  color: var(--c-ink);
  font-size: var(--text-sm);
  letter-spacing: 0.04em;
}

.meta-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
  color: var(--c-ink-faint);
  font-size: var(--text-xs);
}

.cat {
  color: var(--c-ink-muted);
}

.words {
  font-variant-numeric: tabular-nums;
}

.prose {
  margin-inline: auto;
  max-width: 46rem;
  font-size: clamp(1.05rem, 1.3vw, 1.18rem);
  line-height: 1.95;
}

.foot {
  margin-top: var(--space-9);
  padding-top: var(--space-6);
  border-top: 1px solid var(--c-line);
}

.neighbors {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-5);
}

.neighbor {
  position: relative;
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-4);
  border-radius: 3px;
  border: 1px solid var(--c-line);
  background: color-mix(in srgb, var(--c-paper) 72%, var(--c-paper-sunken));
  color: var(--c-ink-muted);
  text-decoration: none;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    background-color var(--duration-fast) var(--ease-out),
    box-shadow var(--duration-mid) var(--ease-out),
    transform var(--duration-mid) var(--ease-out);
}

.neighbor--next {
  text-align: right;
}

.neighbor:not(.neighbor--unavailable)::after {
  position: absolute;
  top: var(--space-4);
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  transition: color var(--duration-fast) var(--ease-out), transform var(--duration-mid) var(--ease-out);
}

.neighbor--previous:not(.neighbor--unavailable)::after {
  right: var(--space-4);
  content: '←';
}

.neighbor--next:not(.neighbor--unavailable)::after {
  left: var(--space-4);
  content: '→';
}

.neighbor:not(.neighbor--unavailable):hover {
  transform: translateY(-2px);
  border-color: var(--c-line-strong);
  background: var(--c-paper);
  color: var(--c-accent);
  box-shadow: 0 8px 18px color-mix(in srgb, var(--c-accent) 12%, transparent);
}

.neighbor--previous:not(.neighbor--unavailable):hover::after {
  color: var(--c-accent);
  transform: translateX(-3px);
}

.neighbor--next:not(.neighbor--unavailable):hover::after {
  color: var(--c-accent);
  transform: translateX(3px);
}

.neighbor__direction {
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
}

.neighbor__title {
  overflow-wrap: anywhere;
  color: var(--c-ink);
  font-size: var(--text-sm);
  line-height: var(--leading-tight);
}

.neighbor--unavailable {
  border-color: transparent;
  background: var(--c-paper-sunken);
  color: var(--c-ink-faint);
  box-shadow: none;
}

.neighbor--unavailable .neighbor__title {
  color: var(--c-ink-faint);
}

@media (max-width: 34rem) {
  .head {
    gap: var(--space-4);
    margin-bottom: var(--space-7);
  }

  .title {
    font-size: clamp(1.875rem, 10vw, 2.75rem);
  }

  .neighbors {
    grid-template-columns: 1fr;
  }

  .neighbor--next {
    text-align: left;
  }
}
</style>
