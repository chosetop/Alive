<script setup lang="ts">
/**
 * Pagination.
 *
 * Links, not buttons: each page is a real URL that can be shared, opened in a
 * new tab, and crawled. A click handler mutating local state would make page 2
 * unreachable except by clicking, which also costs the crawler every entry past
 * the first page.
 */

const props = defineProps<{
  page: number
  pageSize: number
  total: number
  /** Base path; the page number is added as `?page=`. */
  basePath: string
}>()

const totalPages = computed(() =>
  props.total === 0 ? 0 : Math.ceil(props.total / props.pageSize),
)

const hasPrev = computed(() => props.page > 1)
const hasNext = computed(() => props.page < totalPages.value)

/** Page 1 is the bare path: `?page=1` is a duplicate URL for the same content. */
function pageLink(page: number): string {
  return page <= 1 ? props.basePath : `${props.basePath}?page=${page}`
}
</script>

<template>
  <!-- A pager reading "1 / 1" is furniture. -->
  <nav v-if="totalPages > 1" class="pager" aria-label="分页">
    <NuxtLink v-if="hasPrev" class="step" :to="pageLink(page - 1)" rel="prev">
      <span aria-hidden="true">←</span> 上一页
    </NuxtLink>
    <span v-else class="step step--off" aria-hidden="true"><span>←</span> 上一页</span>

    <span class="state">{{ page }} / {{ totalPages }}</span>

    <NuxtLink v-if="hasNext" class="step" :to="pageLink(page + 1)" rel="next">
      下一页 <span aria-hidden="true">→</span>
    </NuxtLink>
    <span v-else class="step step--off" aria-hidden="true">下一页 <span>→</span></span>
  </nav>
</template>

<style scoped>
.pager {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-4);
  margin-top: var(--space-8);
  padding-top: var(--space-5);
  border-top: 1px solid var(--c-line);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
}

.step {
  color: var(--c-ink-muted);
  text-decoration: none;
  transition: color var(--duration-fast) var(--ease-out);
}

.step:hover {
  color: var(--c-accent);
}

/*
 * The unavailable direction keeps its space rather than disappearing, so the
 * page number stays centred instead of jumping between pages.
 *
 * `visibility: hidden` rather than a faint colour. A greyed-out "上一页" is a
 * control the reader can see and cannot use, and no colour resolves that: light
 * enough to read as disabled means too light to read at all (the previous value
 * measured 1.79:1). Reserving the space without drawing anything keeps the
 * layout stable and offers nothing that does not work.
 */
.step--off {
  visibility: hidden;
}

.state {
  color: var(--c-ink-faint);
  font-size: var(--text-xs);
  font-variant-numeric: tabular-nums;
}
</style>
