<script setup lang="ts">
import type { VideoListItem } from '~/types'
defineProps<{ item: VideoListItem }>()
</script>
<template><NuxtLink class="card" :to="`/videos/${item.slug}`"><img v-if="item.cover_url" :src="item.cover_url" alt="" loading="lazy"><div><h2>{{ item.title }}</h2><p v-if="item.summary">{{ item.summary }}</p></div></NuxtLink></template>
<style scoped>.card{display:grid;grid-template-columns:minmax(10rem,18rem) 1fr;gap:var(--space-4);padding-block:var(--space-4);border-bottom:1px solid var(--c-line);color:inherit;text-decoration:none}.card img{width:100%;aspect-ratio:16/9;object-fit:cover;border-radius:var(--radius-control)}.card h2{font-size:var(--text-lg);font-weight:500}.card p{margin-top:var(--space-2);color:var(--c-ink-muted)}@media(max-width:40rem){.card{grid-template-columns:1fr}}</style>
