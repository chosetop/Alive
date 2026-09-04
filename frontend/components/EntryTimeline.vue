<script setup lang="ts">
import type { EntryListItem } from '~/types'

/**
 * A quiet directory of entries. The component name remains for compatibility,
 * but the public presentation is no longer a timeline.
 *
 * Grouping is done here rather than by the API: the endpoint returns a flat
 * page ordered by date, and which date an entry belongs to is a presentation
 * question (`happened_at` when set, `published_at` otherwise -- see
 * `utils/date.ts`).
 *
 * A consequence of grouping a *paginated* list: a year can appear on two pages,
 * once at the end of one and again at the start of the next. That is correct.
 * The alternative, loading every entry to group them completely, trades a real
 * page-weight cost for a cosmetic one.
 */

defineProps<{ entries: EntryListItem[] }>()
</script>

<template>
  <div class="directory">
    <EntryRow v-for="(entry, index) in entries" :key="entry.slug" :entry="entry" :index="index" />
  </div>
</template>

<style scoped>
/*
 * Two columns: a narrow gutter holding the year, and the entries. The gutter is
 * what makes this read as a timeline rather than a list with subheadings.
 */
.directory {
  min-width: 0;
}
</style>
