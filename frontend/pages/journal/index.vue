<script setup lang="ts">
import { defaultWorldCategoryPath } from '~/content-worlds/registry'

/**
 * The Journal world index.
 *
 * This keeps the current immersive list style while moving the canonical route
 * under `/journal`.
 */

const route = useRoute()
const { list } = useEntriesApi()
const { data: categories } = await useSiteCategories('journal')
const defaultCategoryPath = defaultWorldCategoryPath('journal', categories.value)

if (defaultCategoryPath) {
  await navigateTo(defaultCategoryPath, { redirectCode: 302, replace: true })
}

const page = computed(() => {
  const raw = Number(route.query.page)
  return Number.isInteger(raw) && raw > 0 ? raw : 1
})
const { data, error } = await useAsyncData(
  () => `journal-entries-${page.value}`,
  () => list({ page: page.value }),
  { watch: [page] },
)

if (error.value) {
  throw createError({ statusCode: 503, message: '内容暂时读不到', fatal: true })
}

const entries = computed(() => data.value?.data ?? [])
const meta = computed(() => data.value?.meta)

useSeoMeta({
  title: page.value > 1 ? `第 ${page.value} 页 · Journal · Alive` : 'Journal · Alive',
  description: '一个人的记录：日志。',
  robots: page.value > 1 ? 'noindex, follow' : undefined,
})

useHead({
  link: [{ rel: 'canonical', href: page.value > 1 ? `/journal?page=${page.value}` : '/journal' }],
})
</script>

<template>
  <WorldBrowseLayout>
    <template #navigation><WorldCategoryNav world="journal" :categories="categories" /></template>

    <EntryTimeline v-if="entries.length > 0" :entries="entries" />

    <p v-else class="empty">还没有公开的内容。</p>

    <ThePager
      v-if="meta"
      :page="meta.page"
      :page-size="meta.page_size"
      :total="meta.total"
      base-path="/journal"
    />
  </WorldBrowseLayout>
</template>

<style scoped>
.empty {
  padding-block: var(--space-9);
  color: var(--c-ink-faint);
}
</style>
