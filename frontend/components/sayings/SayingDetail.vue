<script setup lang="ts">
import { computed } from 'vue'

import { resolvePublicWorld } from '~/content-worlds/registry'
import type { SayingDetail } from '~/types'

const props = defineProps<{
  entry: SayingDetail
}>()

const world = resolvePublicWorld('saying')

const previousHref = computed(() =>
  props.entry.previous ? world?.entryPath(props.entry.previous.short_id) ?? `/sayings/${props.entry.previous.short_id}` : null,
)
const nextHref = computed(() =>
  props.entry.next ? world?.entryPath(props.entry.next.short_id) ?? `/sayings/${props.entry.next.short_id}` : null,
)
</script>

<template>
  <article class="detail">
    <SayingItem :item="entry" />

    <div class="tools">
      <SayingActions :short-id="entry.short_id" />
    </div>

    <nav class="neighbors" aria-label="片语前后文">
      <NuxtLink v-if="previousHref" class="neighbor" :to="previousHref" rel="prev">上一句</NuxtLink>
      <span v-else class="neighbor neighbor--off" aria-disabled="true">上一句</span>

      <NuxtLink v-if="nextHref" class="neighbor" :to="nextHref" rel="next">下一句</NuxtLink>
      <span v-else class="neighbor neighbor--off" aria-disabled="true">下一句</span>
    </nav>
  </article>
</template>

<style scoped>
.detail {
  display: grid;
  gap: var(--space-6);
}

.tools {
  display: flex;
  justify-content: flex-start;
}

.neighbors {
  display: flex;
  justify-content: space-between;
  gap: var(--space-4);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
}

.neighbor {
  color: var(--c-ink-muted);
  text-decoration: none;
}

.neighbor:hover {
  color: var(--c-accent);
}

.neighbor--off {
  visibility: hidden;
}
</style>
