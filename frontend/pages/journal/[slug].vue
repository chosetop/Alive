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
    <header class="head">
      <h1 class="title">{{ entry.title }}</h1>

      <div class="dateline">
        <time v-if="date" :datetime="toDateAttribute(date)">{{ formatFullDate(date) }}</time>
        <span class="type">{{ journal?.label ?? '日志' }}</span>
        <NuxtLink
          v-if="entry.category"
          :to="journal?.categoryPath(entry.category.slug) ?? `/journal/categories/${entry.category.slug}`"
          class="cat"
        >
          {{ entry.category.name }}
        </NuxtLink>
        <span class="words">{{ entry.word_count }} 字</span>
      </div>
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
