<script setup lang="ts">
/**
 * The Journal world index.
 *
 * This keeps the current immersive list style while moving the canonical route
 * under `/journal`.
 */

const route = useRoute()
const { list } = useEntriesApi()

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
  <div>
    <EntryTimeline v-if="entries.length > 0" :entries="entries" />

    <p v-else class="empty">还没有公开的内容。</p>

    <ThePager
      v-if="meta"
      :page="meta.page"
      :page-size="meta.page_size"
      :total="meta.total"
      base-path="/journal"
    />
  </div>
</template>

<style scoped>
.empty {
  padding-block: var(--space-9);
  color: var(--c-ink-faint);
}
</style>
