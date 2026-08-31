<script setup lang="ts">
/**
 * The index: every published entry, newest first, grouped by year.
 *
 * `page` comes from the query string rather than component state so each page is
 * a shareable, crawlable URL. `watch` on the route means changing it refetches
 * without a full reload.
 */

const route = useRoute()
const { list } = useEntriesApi()

/** A non-numeric or out-of-range `?page=` reads as page 1 rather than erroring. */
const page = computed(() => {
  const raw = Number(route.query.page)
  return Number.isInteger(raw) && raw > 0 ? raw : 1
})

const { data, error } = await useAsyncData(
  () => `entries-${page.value}`,
  () => list({ page: page.value }),
  { watch: [page] },
)

/**
 * A failed fetch here is the whole page, so it becomes a real error response
 * rather than an empty list: an empty list says "nothing written yet", which is
 * a different and misleading claim.
 */
if (error.value) {
  // `message`, not `statusMessage`: h3 reserves statusMessage for the HTTP
  // reason phrase and will sanitise it, which would strip Chinese text.
  throw createError({ statusCode: 503, message: '内容暂时读不到', fatal: true })
}

const entries = computed(() => data.value?.data ?? [])
const meta = computed(() => data.value?.meta)

/**
 * Page 2 and beyond are marked `noindex`.
 *
 * Paginated listings compete with the entries they link to for the same terms,
 * and the entries are what should rank. `follow` is kept so crawlers still walk
 * through to every entry.
 */
useSeoMeta({
  title: page.value > 1 ? `第 ${page.value} 页 · Alive` : 'Alive',
  description: '一个人的记录：日志、书、影、乐、旅行与影像。',
  robots: page.value > 1 ? 'noindex, follow' : undefined,
})

/** Canonical points at the bare path for page 1, so `?page=1` does not duplicate it. */
useHead({
  link: [{ rel: 'canonical', href: page.value > 1 ? `/?page=${page.value}` : '/' }],
})
</script>

<template>
  <div>
    <MixedHome v-if="page === 1" />
    <EntryTimeline v-else-if="entries.length > 0" :entries="entries" />

    <p v-else class="empty">还没有公开的内容。</p>

    <ThePager
      v-if="meta"
      :page="meta.page"
      :page-size="meta.page_size"
      :total="meta.total"
      base-path="/"
    />
  </div>
</template>

<style scoped>
.empty {
  padding-block: var(--space-9);
  color: var(--c-ink-faint);
}
</style>
