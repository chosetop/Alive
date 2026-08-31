<script setup lang="ts">
const route = useRoute()
const api = useSayingsApi()
const { data } = await useAsyncData(`saying-${route.params.shortID}`, () => api.getByShortId(String(route.params.shortID)))
if (!data.value) throw createError({ statusCode: 404, statusMessage: 'Saying not found' })
</script>

<template>
  <main v-if="data" class="detail"><NuxtLink to="/sayings">片语</NuxtLink><p class="body">{{ data.content_md }}</p><p v-if="data.source || data.author" class="meta">{{ data.source }}<span v-if="data.source && data.author"> · </span>{{ data.author }}</p><nav class="neighbors"><NuxtLink v-if="data.previous" :to="`/sayings/${data.previous.short_id}`">上一句</NuxtLink><NuxtLink v-if="data.next" :to="`/sayings/${data.next.short_id}`">下一句</NuxtLink></nav></main>
</template>

<style scoped>
.detail{max-width:44rem;margin:auto;padding:3rem 1.25rem}.body{margin:20vh 0 2rem;font-size:clamp(1.5rem,4vw,2.4rem);line-height:1.8;white-space:pre-wrap}.meta{color:var(--c-ink-muted)}.neighbors{display:flex;justify-content:space-between;margin-top:3rem}
</style>
