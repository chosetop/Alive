<script setup lang="ts">
/**
 * One category's entries.
 *
 * The same timeline as the index, filtered. `?category=` takes a **slug**, not an
 * id, so the URL segment passes straight through.
 *
 * An unknown slug is a 404 from the backend rather than an empty page, which is
 * the behaviour worth preserving: a mistyped link and a category with nothing in
 * it are different situations and must read differently.
 */

const route = useRoute()
const { list } = useEntriesApi()

const slug = computed(() => String(route.params.slug))

const page = computed(() => {
  const raw = Number(route.query.page)
  return Number.isInteger(raw) && raw > 0 ? raw : 1
})

const { data, error } = await useAsyncData(
  () => `category-${slug.value}-${page.value}`,
  () => list({ page: page.value, category: slug.value }),
  { watch: [slug, page] },
)

if (error.value) {
  /**
   * A 404 from the backend means no such category. Anything else is a fetch
   * failure, and saying "no such category" for a network problem would send the
   * reader looking for a link that is in fact fine.
   */
  const status = (error.value as { status?: number }).status
  // `message`, not `statusMessage`: the latter is the HTTP reason phrase and h3
  // sanitises it, which would strip Chinese text.
  throw createError({
    statusCode: status === 404 ? 404 : 503,
    message: status === 404 ? '没有这个分类' : '内容暂时读不到',
    fatal: true,
  })
}

/**
 * The name and description come from the category list, which the layout has
 * already fetched under the same key, so this reuses that result rather than
 * issuing a second request.
 */
const { data: categories } = await useSiteCategories()

const category = computed(() => categories.value.find((c) => c.slug === slug.value) ?? null)

const entries = computed(() => data.value?.data ?? [])
const meta = computed(() => data.value?.meta)

/** Falls back to the slug: the entries loaded, so the page should still render. */
const title = computed(() => category.value?.name ?? slug.value)

useSeoMeta({
  title: () => (page.value > 1 ? `${title.value} · 第 ${page.value} 页 · Alive` : `${title.value} · Alive`),
  description: () => category.value?.description || `${title.value}分类下的全部内容。`,
  robots: page.value > 1 ? 'noindex, follow' : undefined,
})

useHead({
  link: [
    {
      rel: 'canonical',
      href: computed(() =>
        page.value > 1
          ? `/categories/${slug.value}?page=${page.value}`
          : `/categories/${slug.value}`,
      ),
    },
  ],
})
</script>

<template>
  <div>
    <header class="head">
      <p class="kicker">分类</p>
      <h1 class="title">{{ title }}</h1>
      <p v-if="category?.description" class="desc">{{ category.description }}</p>
    </header>

    <EntryTimeline v-if="entries.length > 0" :entries="entries" />

    <!--
      Reachable: a category can exist with nothing published in it, and the
      footer hides those, but a direct link still arrives here.
    -->
    <p v-else class="empty">这个分类下还没有公开的内容。</p>

    <ThePager
      v-if="meta"
      :page="meta.page"
      :page-size="meta.page_size"
      :total="meta.total"
      :base-path="`/categories/${slug}`"
    />
  </div>
</template>

<style scoped>
.head {
  max-width: var(--measure);
  margin-bottom: var(--space-7);
}

/*
 * States what kind of page this is, since the title alone ("旅行") could be an
 * entry. Small caps in the UI face keeps it clearly subordinate.
 */
.kicker {
  margin-bottom: var(--space-2);
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
  font-variant-caps: all-small-caps;
  letter-spacing: 0.08em;
  line-height: var(--leading-none);
}

.title {
  font-size: var(--text-2xl);
  line-height: var(--leading-tight);
  letter-spacing: var(--tracking-display);
}

.desc {
  margin-top: var(--space-3);
  color: var(--c-ink-muted);
  font-size: var(--text-sm);
}

.empty {
  padding-block: var(--space-9);
  color: var(--c-ink-faint);
}

@media (max-width: 34rem) {
  .title {
    font-size: var(--text-xl);
  }
}
</style>
