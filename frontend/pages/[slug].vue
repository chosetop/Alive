<script setup lang="ts">
import { entryTypeStyle } from '~/composables/useEntryType'
import { entryDate, formatFullDate, toDateAttribute } from '~/utils/date'
import { getEntryNeighbors } from '~/utils/entry-navigation'
import { markdownToText, renderMarkdown } from '~/utils/markdown'

/**
 * One entry, addressed by slug at the root of the site.
 *
 * Root-level URLs were chosen on 2026-08-25 (see progress.md): `/kyoto-spring`
 * rather than `/entries/kyoto-spring`. Nuxt gives static routes priority over
 * this dynamic one, so `/categories/travel` still resolves to the category page.
 * The one real hazard is an entry whose slug collides with a static path, which
 * would be unreachable.
 */

const route = useRoute()
const { getBySlug, list } = useEntriesApi()

const slug = computed(() => String(route.params.slug))

const { data: entry, error } = await useAsyncData(
  () => `entry-${slug.value}`,
  () => getBySlug(slug.value),
  { watch: [slug] },
)

/**
 * Any failure becomes a 404.
 *
 * The backend already answers 404 identically for a draft, an archived entry, a
 * private one, a deleted one and a slug that never existed, so that a reader
 * cannot probe for unpublished work. Collapsing every error to the same page
 * here keeps that property instead of leaking the difference through the UI.
 */
if (error.value || !entry.value) {
  // `message`, not `statusMessage`: the latter is the HTTP reason phrase and h3
  // sanitises it, which would strip Chinese text.
  throw createError({ statusCode: 404, message: '没有这一篇', fatal: true })
}

/**
 * The detail request remains authoritative for the article. Navigation is an
 * enhancement, so a list failure leaves the reader on a healthy detail page.
 */
const { data: navigationPage } = await useAsyncData(
  () => `entry-navigation-${slug.value}`,
  () => list({ page_size: 50 }),
  { watch: [slug] },
)

const neighbors = computed(() =>
  getEntryNeighbors(navigationPage.value?.data ?? [], slug.value),
)

const style = computed(() => entryTypeStyle(entry.value!.type))
const date = computed(() => entryDate(entry.value!))

/** Rendered once, during SSR. See utils/markdown.ts for why `v-html` is safe. */
const html = computed(() => renderMarkdown(entry.value!.content_md))

/**
 * A hand-written summary beats a truncated body, so the fallback only runs when
 * `summary` is empty.
 */
const description = computed(() => {
  const e = entry.value!
  return e.summary !== '' ? e.summary : markdownToText(e.content_md)
})

const canonical = computed(() => `/${entry.value!.slug}`)

useSeoMeta({
  title: () => `${entry.value!.title} · Alive`,
  description,
  ogTitle: () => entry.value!.title,
  ogDescription: description,
  ogType: 'article',
  /** Absent rather than empty: an empty og:image is worse than none. */
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
      /**
       * JSON-LD for the entry. `innerHTML` with a stringified object rather than
       * a template, so the values are JSON-escaped and cannot break out of the
       * script element.
       */
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
    <header class="head">
      <h1 class="title">{{ entry.title }}</h1>

      <div class="dateline">
        <time v-if="date" :datetime="toDateAttribute(date)">{{ formatFullDate(date) }}</time>
        <span class="type">{{ style.label }}</span>
        <NuxtLink v-if="entry.category" :to="`/categories/${entry.category.slug}`" class="cat">
          {{ entry.category.name }}
        </NuxtLink>
        <span class="words">{{ entry.word_count }} 字</span>
      </div>

      <!--
        The cover is shown only for the types whose layout leads with it. A book
        jacket at full width above the text would dominate a review it only
        illustrates.
      -->
      <img
        v-if="entry.cover_url && style.cover === 'lead'"
        class="cover"
        :src="entry.cover_url"
        alt=""
        fetchpriority="high"
      />
    </header>

    <!-- eslint-disable-next-line vue/no-v-html -->
    <div class="prose" v-html="html" />

    <footer class="foot">
      <nav v-if="neighbors.previous || neighbors.next" class="neighbors" aria-label="文章导航">
        <NuxtLink
          v-if="neighbors.previous"
          class="neighbor neighbor--previous"
          :to="`/${neighbors.previous.slug}`"
          rel="prev"
        >
          <span class="neighbor__direction">上一篇</span>
          <span class="neighbor__title">{{ neighbors.previous.title }}</span>
        </NuxtLink>
        <span v-else class="neighbor neighbor--empty" aria-hidden="true" />

        <NuxtLink
          v-if="neighbors.next"
          class="neighbor neighbor--next"
          :to="`/${neighbors.next.slug}`"
          rel="next"
        >
          <span class="neighbor__direction">下一篇</span>
          <span class="neighbor__title">{{ neighbors.next.title }}</span>
        </NuxtLink>
        <span v-else class="neighbor neighbor--empty" aria-hidden="true" />
      </nav>
      <NuxtLink to="/" class="back">← 回到全部</NuxtLink>
    </footer>
  </article>
</template>

<style scoped>
/*
 * The detail page gets a wider reading column than the shared prose default.
 * Keeping the whole article in one centered column aligns the title, metadata,
 * cover and body without making the page feel left-heavy on large screens.
 */
.entry {
  width: min(100%, 48rem);
  max-width: 48rem;
  margin-inline: auto;
}

.entry > .prose {
  max-width: none;
}

.head {
  margin-bottom: var(--space-7);
}

/*
 * The largest type on the site. With one weight available, size is what marks
 * this as the page's subject.
 */
.title {
  font-size: var(--text-2xl);
  line-height: var(--leading-tight);
  /* Chinese titles do not need tracking; the Latin in a mixed title does. */
  letter-spacing: var(--tracking-display);
}

.dateline {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: var(--space-2) var(--space-4);
  margin-top: var(--space-4);
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
  line-height: var(--leading-none);
}

.dateline time {
  font-variant-numeric: tabular-nums;
}

.cat {
  color: var(--c-ink-muted);
  text-decoration: none;
}

.cat:hover {
  color: var(--c-accent);
}

.cover {
  width: 100%;
  margin-top: var(--space-6);
  border-radius: 2px;
  box-shadow: var(--shadow-image);
}

.foot {
  margin-top: var(--space-9);
  padding-top: var(--space-5);
  border-top: 1px solid var(--c-line);
}

.neighbors {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-5);
  margin-bottom: var(--space-6);
}

.neighbor {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: var(--space-1);
  color: var(--c-ink-muted);
  text-decoration: none;
}

.neighbor--next {
  align-items: flex-end;
  text-align: right;
}

.neighbor__direction {
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
}

.neighbor__title {
  max-width: 100%;
  overflow: hidden;
  color: var(--c-ink);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.neighbor:hover .neighbor__title {
  color: var(--c-accent);
}

.back {
  color: var(--c-ink-muted);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  text-decoration: none;
}

.back:hover {
  color: var(--c-accent);
}

@media (max-width: 34rem) {
  .title {
    font-size: var(--text-xl);
  }

  .neighbors {
    gap: var(--space-3);
  }
}
</style>
