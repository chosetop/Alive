<script setup lang="ts">
import { onMounted, ref, nextTick } from 'vue'

import { resolvePublicWorld } from '~/content-worlds/registry'
import { useSayingsApi } from '~/composables/useSayingsApi'
import { consumeSayingAnchor } from '~/composables/useSayingView'
import { useSiteCategories } from '~/composables/useSiteCategories'
import { markdownToText } from '~/utils/markdown'

const route = useRoute()
const sayingsWorld = resolvePublicWorld('saying')
const { list } = useSayingsApi()
const { data: categories } = await useSiteCategories('saying')

const slug = computed(() => String(route.params.slug))
const page = computed(() => {
  const raw = Number(route.query.page)
  return Number.isInteger(raw) && raw > 0 ? raw : 1
})

const category = computed(() => categories.value.find((item) => item.slug === slug.value) ?? null)
const { data, error } = await useAsyncData(
  () => `sayings-category-${slug.value}-${page.value}`,
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

const items = computed(() => data.value?.data ?? [])
const meta = computed(() => data.value?.meta)
const heading = ref<HTMLElement | null>(null)
const pendingAnchor = import.meta.client && typeof window !== 'undefined' ? consumeSayingAnchor(window.history) : null

onMounted(async () => {
  if (pendingAnchor === null) return

  await nextTick()
  const selector = `[data-saying-id="${pendingAnchor.shortId.replace(/"/g, '\\"')}"]`
  const target = document.querySelector<HTMLElement>(selector)

  if (target !== null) {
    target.focus({ preventScroll: true })
    window.scrollTo({ top: pendingAnchor.offsetTop, behavior: 'auto' })
    return
  }

  heading.value?.focus({ preventScroll: true })
})

const title = computed(() => category.value?.name ?? slug.value)

useSeoMeta({
  title: () => (page.value > 1 ? `${title.value} · 第 ${page.value} 页 · 片语 · Alive` : `${title.value} · 片语 · Alive`),
  description: () => category.value?.description || `${title.value} 分类下的全部片语。`,
  robots: page.value > 1 ? 'noindex, follow' : undefined,
})

useHead({
  link: [
    {
      rel: 'canonical',
      href: page.value > 1
        ? `/sayings/categories/${slug.value}?page=${page.value}`
        : `/sayings/categories/${slug.value}`,
    },
  ],
  script: [
    {
      type: 'application/ld+json',
      innerHTML: computed(() =>
        JSON.stringify({
          '@context': 'https://schema.org',
          '@type': 'CollectionPage',
          url: page.value > 1
            ? `/sayings/categories/${slug.value}?page=${page.value}`
            : `/sayings/categories/${slug.value}`,
          description: category.value?.description || `${title.value} 分类下的全部片语。`,
          about: title.value,
          mainEntity: items.value.map((item) => ({
            '@type': 'SocialMediaPosting',
            articleBody: markdownToText(item.content_md, 280),
          })),
        }),
      ),
    },
  ],
})
</script>

<template>
  <div class="sayings-page">
    <WorldBrowseLayout>
      <template #header>
        <header class="head">
          <div>
            <h1 ref="heading" tabindex="-1" class="title">{{ title }}</h1>
            <p v-if="category?.description" class="desc">{{ category.description }}</p>
          </div>
        </header>
      </template>

      <template #navigation>
        <WorldCategoryNav world="saying" :categories="categories" :active-slug="slug" />
      </template>

      <SayingWall :items="items" />

      <p v-if="items.length === 0" class="empty">这个分类下还没有公开的片语。</p>

      <ThePager
        v-if="meta"
        :page="meta.page"
        :page-size="meta.page_size"
        :total="meta.total"
        :base-path="sayingsWorld?.categoryPath(slug) ?? `/sayings/categories/${slug}`"
      />
    </WorldBrowseLayout>
  </div>
</template>

<style scoped>
.sayings-page {
  min-width: 0;
}

.head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--space-6);
}

.title {
  margin-bottom: var(--space-2);
  font-family: var(--font-heading);
  font-size: clamp(1.75rem, 4vw, 2.6rem);
}

.desc {
  max-width: 32rem;
  color: var(--c-ink-muted);
}

.empty {
  padding-block: var(--space-9);
  color: var(--c-ink-faint);
}

@media (max-width: 42rem) {
  .head {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
