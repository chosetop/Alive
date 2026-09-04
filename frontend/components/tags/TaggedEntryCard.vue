<script setup lang="ts">
import type { TaggedEntry } from '~/types'
const props = defineProps<{ item: TaggedEntry }>()
const known = ['journal', 'saying', 'video'].includes(props.item.world)
const href = props.item.world === 'journal' ? `/journal/${props.item.slug}` : `/videos/${props.item.slug}`
</script>
<template>
  <component :is="known && item.world !== 'saying' ? 'NuxtLink' : 'UnavailableWorldCard'" v-if="known" :to="href" class="card">
    <span class="world">{{ item.world }}</span><h2>{{ item.title || item.saying?.content_md || item.slug }}</h2><p v-if="item.excerpt">{{ item.excerpt }}</p>
  </component>
  <UnavailableWorldCard v-else />
</template>
<style scoped>.card{display:block;padding:var(--space-4);border-bottom:1px solid var(--c-line);color:inherit;text-decoration:none}.card:hover{color:var(--c-accent)}.world{font:var(--text-xs) var(--font-ui);color:var(--c-ink-faint);text-transform:uppercase}.card h2{margin-top:var(--space-2);font-size:var(--text-lg);font-weight:500}.card p{margin-top:var(--space-2);color:var(--c-ink-muted)}</style>
