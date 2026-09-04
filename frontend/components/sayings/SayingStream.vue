<script setup lang="ts">
import type { SayingListItem } from '~/types'
import { ref } from 'vue'
import { useSayingMotion } from '~/composables/useSayingMotion'

defineProps<{
  items: SayingListItem[]
}>()
const root = ref<HTMLElement | null>(null)
useSayingMotion(root)
</script>

<template>
  <div ref="root" class="stream">
    <section v-for="item in items" :key="item.short_id" class="row">
      <SayingItem :item="item" />
      <SayingActions :item="item" />
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
.row:hover :deep(.actions) .action,
.row:focus-within :deep(.actions) .action { opacity: 1; transform: translateY(0); }
</style>
