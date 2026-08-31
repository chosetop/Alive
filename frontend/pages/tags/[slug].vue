<script setup lang="ts">
import { computed } from 'vue'
import { useTagsApi } from '~/composables/useTagsApi'
const route = useRoute()
const { getEntries } = useTagsApi()
const slug = computed(() => String(route.params.slug))
const { data, error } = await useAsyncData(() => `tag-${slug.value}`, () => getEntries(slug.value), { watch: [slug] })
if (error.value) throw createError({ statusCode: 404, message: '标签不存在', fatal: true })
const page = computed(() => data.value)
useSeoMeta({ title: () => `${page.value?.tag.name ?? slug.value} · 标签 · Alive` })
</script>
<template><main class="tag-page"><NuxtLink to="/">返回</NuxtLink><header><p class="kicker">标签</p><h1>{{ page?.tag.name || slug }}</h1></header><section v-if="page?.items.length" class="items"><TaggedEntryCard v-for="item in page.items" :key="`${item.world}:${item.slug}`" :item="item" /></section><p v-else class="empty">这个标签还没有公开内容。</p></main></template>
<style scoped>.tag-page{display:grid;gap:var(--space-6)}header{display:grid;gap:var(--space-2)}.kicker{color:var(--c-ink-faint);font:var(--text-xs) var(--font-ui);letter-spacing:.1em;text-transform:uppercase}.items{display:grid}.empty{color:var(--c-ink-muted)}</style>
