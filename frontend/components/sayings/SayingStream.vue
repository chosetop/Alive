<script setup lang="ts">
import type { SayingListItem } from '~/types'

defineProps<{
  items: SayingListItem[]
}>()
</script>

<template>
  <div class="stream">
    <section v-for="item in items" :key="item.short_id" class="row">
      <SayingItem :item="item" />
      <SayingActions :short-id="item.short_id" />
    </section>
  </div>
</template>

<style scoped>
.stream {
  display: grid;
  gap: var(--space-7);
}

.row {
  display: grid;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  border-bottom: 1px solid var(--c-line);
}
</style>
