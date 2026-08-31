<script setup lang="ts">
import { useVideosApi } from '~/composables/useVideosApi'
const route = useRoute()
const { getBySlug } = useVideosApi()
const { data, error } = await useAsyncData(() => `video-${route.params.slug}`, () => getBySlug(String(route.params.slug)))
if (error.value || !data.value) throw createError({ statusCode: 404, message: '影像不存在', fatal: true })
useSeoMeta({ title: () => `${data.value!.title} · 影像 · Alive` })
useHead({
  script: [{
    type: 'application/ld+json',
    innerHTML: computed(() => JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'VideoObject',
      name: data.value?.title,
      description: data.value?.summary || undefined,
      thumbnailUrl: data.value?.cover_url || undefined,
      contentUrl: data.value?.primary_media?.url || undefined,
      uploadDate: data.value?.published_at || undefined,
    })),
  }],
})
</script>
<template><main v-if="data" class="detail"><NuxtLink to="/videos">影像</NuxtLink><VideoPlayer :src="data.primary_media?.url" :poster="data.cover_url" /><h1>{{ data.title }}</h1><p v-if="data.summary">{{ data.summary }}</p></main></template>
<style scoped>.detail{display:grid;gap:var(--space-5)}.detail h1{font-size:var(--text-xl)}.detail p{color:var(--c-ink-muted)}</style>
