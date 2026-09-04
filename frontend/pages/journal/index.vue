<script setup lang="ts">
import { sortEntriesByCreatedAt, type EntryCreatedSort } from '~/utils/entry-sort'

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
const sort = computed<EntryCreatedSort>(() => route.query.sort === 'oldest' ? 'oldest' : 'newest')

const { data, error } = await useAsyncData(
  () => `journal-entries-${sort.value}`,
  async () => {
    const first = await list({ page: 1, page_size: 50 })
    const rest = await Promise.all(Array.from({ length: Math.max(0, Math.ceil(first.meta.total / first.meta.page_size) - 1) }, (_, i) => list({ page: i + 2, page_size: first.meta.page_size })))
    const all = sortEntriesByCreatedAt([first, ...rest].flatMap((part) => part.data), sort.value)
    const start = (page.value - 1) * first.meta.page_size
    return { data: all.slice(start, start + first.meta.page_size), meta: { ...first.meta, page: page.value } }
  },
  { watch: [page, sort] },
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
    <label class="sort">创建时间
      <select :value="sort" @change="navigateTo({ path: '/journal', query: { sort: ($event.target as HTMLSelectElement).value } })">
        <option value="newest">从新到旧</option>
        <option value="oldest">从旧到新</option>
      </select>
    </label>
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
.sort { display: inline-flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-4); color: var(--c-ink-muted); font-family: var(--font-ui); font-size: var(--text-sm); }
.sort select { border: 1px solid var(--c-line); border-radius: 3px; background: var(--c-paper); padding: 0.3rem 0.5rem; }
</style>
