<script setup lang="ts">
import { onMounted, nextTick } from 'vue'

import { defaultWorldCategoryPath, resolvePublicWorld } from '~/content-worlds/registry'
import { useSayingsApi } from '~/composables/useSayingsApi'
import { consumeSayingAnchor } from '~/composables/useSayingView'
import { useSiteCategories } from '~/composables/useSiteCategories'
import { markdownToText } from '~/utils/markdown'

const route = useRoute()
const sayingsWorld = resolvePublicWorld('saying')
const { list } = useSayingsApi()
const { data: categories } = await useSiteCategories('saying')
const defaultCategoryPath = defaultWorldCategoryPath('saying', categories.value)

if (defaultCategoryPath) {
  await navigateTo(defaultCategoryPath, { redirectCode: 302, replace: true })
}

const page = computed(() => {
  const raw = Number(route.query.page)
  return Number.isInteger(raw) && raw > 0 ? raw : 1
})

const { data, error } = await useAsyncData(
  () => `sayings-${page.value}`,
  () => list({ page: page.value }),
  { watch: [page] },
)

if (error.value) {
  throw createError({ statusCode: 503, message: '内容暂时读不到', fatal: true })
}

const items = computed(() => data.value?.data ?? [])
const meta = computed(() => data.value?.meta)
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

})

useSeoMeta({
  title: page.value > 1 ? `第 ${page.value} 页 · 片语 · Alive` : '片语 · Alive',
  description: '没有时间轴的短句、片语与轻量记录。',
  robots: page.value > 1 ? 'noindex, follow' : undefined,
})

useHead({
  link: [
    {
      rel: 'canonical',
      href: page.value > 1 ? `${sayingsWorld?.rootPath ?? '/sayings'}?page=${page.value}` : '/sayings',
    },
  ],
  script: [
    {
      type: 'application/ld+json',
      innerHTML: computed(() =>
        JSON.stringify({
          '@context': 'https://schema.org',
          '@type': 'CollectionPage',
          url: page.value > 1 ? `${sayingsWorld?.rootPath ?? '/sayings'}?page=${page.value}` : '/sayings',
          description: '没有时间轴的短句、片语与轻量记录。',
          about: '片语',
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
      <template v-if="categories.length > 0" #navigation>
        <WorldCategoryNav world="saying" :categories="categories" />
      </template>

      <SayingWall :items="items" />

      <p v-if="items.length === 0" class="empty">还没有公开的片语。</p>

      <ThePager
        v-if="meta"
        :page="meta.page"
        :page-size="meta.page_size"
        :total="meta.total"
        base-path="/sayings"
      />
    </WorldBrowseLayout>
  </div>
</template>

<style scoped>
.sayings-page {
  min-width: 0;
}

.empty {
  padding-block: var(--space-9);
  color: var(--c-ink-faint);
}
</style>
