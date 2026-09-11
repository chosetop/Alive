<script setup lang="ts">
import { resolvePublicWorld } from '~/content-worlds/registry'

/**
 * One Journal category's entries, under the new canonical world path.
 */

const route = useRoute()
const { list } = useEntriesApi()
const journal = resolvePublicWorld('journal')

const slug = computed(() => String(route.params.slug))

const page = computed(() => {
  const raw = Number(route.query.page)
  return Number.isInteger(raw) && raw > 0 ? raw : 1
})

const { data, error } = await useAsyncData(
  () => `journal-category-${slug.value}-${page.value}`,
  () => list({ page: page.value, category: slug.value }),
  { watch: [slug, page] },
)
if (error.value) {
  const status = (error.value as { status?: number }).status
  throw createError({
    statusCode: status === 404 ? 404 : 503,
    message: status === 404 ? '没有这个分类' : '内容暂时读不到',
    fatal: true,
  })
}

const { data: categories } = await useSiteCategories()
const category = computed(() => categories.value.find((c) => c.slug === slug.value) ?? null)
const entries = computed(() => data.value?.data ?? [])
const meta = computed(() => data.value?.meta)
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
          ? `/journal/categories/${slug.value}?page=${page.value}`
          : `/journal/categories/${slug.value}`,
      ),
    },
  ],
})
</script>

<template>
  <WorldBrowseLayout>
    <template #header>
      <header class="head">
        <h1 class="title">{{ title }}</h1>
        <p v-if="category?.description" class="desc">{{ category.description }}</p>
      </header>
    </template>

    <template #navigation>
      <WorldCategoryNav world="journal" :categories="categories" :active-slug="slug" />
    </template>

    <EntryTimeline v-if="entries.length > 0" :entries="entries" />

    <p v-else class="empty">这个分类下还没有公开的内容。</p>

    <ThePager
      v-if="meta"
      :page="meta.page"
      :page-size="meta.page_size"
      :total="meta.total"
      :base-path="journal?.categoryPath(slug) ?? `/journal/categories/${slug}`"
    />
  </WorldBrowseLayout>
</template>

<style scoped>
.head {
  display: grid;
  gap: var(--space-2);
}

.title {
  font-family: var(--font-heading);
  font-size: clamp(2.2rem, 4vw, 3.2rem);
  font-weight: 500;
}

.desc {
  max-width: var(--measure);
  color: var(--c-ink-muted);
}
</style>
