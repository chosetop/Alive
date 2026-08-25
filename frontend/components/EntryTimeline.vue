<script setup lang="ts">
import type { EntryListItem } from '~/types'
import { entryDate, formatYear } from '~/utils/date'

/**
 * The entry list, grouped by year.
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

const props = defineProps<{ entries: EntryListItem[] }>()

type YearGroup = {
  /** `''` for entries carrying no usable date; rendered without a heading. */
  year: string
  entries: EntryListItem[]
}

/**
 * Consecutive runs, not a map: the API already sorts by date, so walking the
 * list preserves that order. Bucketing into an object and reading the keys back
 * would depend on key ordering, which is a different guarantee.
 */
const groups = computed<YearGroup[]>(() => {
  const result: YearGroup[] = []
  for (const entry of props.entries) {
    const year = formatYear(entryDate(entry))
    const last = result.at(-1)
    if (last?.year === year) last.entries.push(entry)
    else result.push({ year, entries: [entry] })
  }
  return result
})
</script>

<template>
  <div class="timeline">
    <section v-for="group in groups" :key="group.year || 'undated'" class="group">
      <!--
        aria-hidden: the year is a visual anchor, and each entry's own <time>
        already carries the full machine-readable date. Announcing the year again
        as a heading would add a level to the outline that means nothing.
      -->
      <p v-if="group.year" class="year" aria-hidden="true">{{ group.year }}</p>

      <div class="entries">
        <EntryRow v-for="entry in group.entries" :key="entry.slug" :entry="entry" />
      </div>
    </section>
  </div>
</template>

<style scoped>
/*
 * Two columns: a narrow gutter holding the year, and the entries. The gutter is
 * what makes this read as a timeline rather than a list with subheadings.
 */
.group {
  display: grid;
  grid-template-columns: 4rem 1fr;
  gap: 0 var(--space-5);
}

.year {
  /*
   * Sticks while its own entries scroll past, so the year in view always matches
   * the entries in view. `start` aligns it with the first entry's text rather
   * than with the top of its padding.
   */
  position: sticky;
  top: var(--space-5);
  align-self: start;
  /* Matches EntryRow's top padding so the year sits on the first title's line. */
  padding-top: var(--space-6);
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  font-variant-numeric: tabular-nums;
  line-height: var(--leading-none);
  letter-spacing: 0.02em;
}

.entries {
  min-width: 0;
}

/*
 * The rule runs down the gutter's edge, continuous across the whole timeline
 * rather than per group, so the years read as points on one line.
 */
.entries {
  position: relative;
}

.entries::before {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  /* Sits in the gap, not on the content edge. */
  left: calc(var(--space-5) / -2);
  width: 1px;
  background: var(--c-line);
}

/* --------------------------------------------------------------- responsive */

/*
 * Below this width the gutter takes too much from the measure. The year becomes
 * a heading above its entries and the rule disappears -- keeping a 4rem gutter
 * on a 375px screen would leave the titles wrapping every three characters.
 */
@media (max-width: 34rem) {
  .group {
    display: block;
  }

  .year {
    position: static;
    padding-top: var(--space-6);
    padding-bottom: var(--space-2);
    font-size: var(--text-xs);
  }

  .entries::before {
    display: none;
  }
}
</style>
