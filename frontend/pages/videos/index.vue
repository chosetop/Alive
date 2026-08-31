<script setup lang="ts">
import { useVideosApi } from '~/composables/useVideosApi'
const { list } = useVideosApi()
const { data, error } = await useAsyncData('videos', () => list())
if (error.value) throw createError({ statusCode: 503, message: '影像暂时读不到', fatal: true })
useSeoMeta({ title: '影像 · Alive' })
</script>
<template><main class="videos"><header><p>影像</p><h1>影像</h1></header><section v-if="data?.data.length"><VideoCard v-for="item in data.data" :key="item.slug" :item="item" /></section><p v-else>还没有公开的影像。</p></main></template>
<style scoped>.videos{display:grid;gap:var(--space-6)}header{display:grid;gap:var(--space-2)}header p{color:var(--c-ink-faint);font:var(--text-xs) var(--font-ui);letter-spacing:.1em}</style>
