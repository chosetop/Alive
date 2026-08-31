<script setup lang="ts">
import { computed } from 'vue'

import { resolvePublicWorld } from '~/content-worlds/registry'
import { useSayingsApi } from '~/composables/useSayingsApi'
import { markdownToText } from '~/utils/markdown'

const route = useRoute()
const { getByShortId } = useSayingsApi()
const sayingsWorld = resolvePublicWorld('saying')

const shortId = computed(() => String(route.params.shortID))

const { data: entry, error } = await useAsyncData(
  () => `saying-${shortId.value}`,
  () => getByShortId(shortId.value),
  { watch: [shortId] },
)

if (error.value || !entry.value) {
  throw createError({ statusCode: 404, message: '没有这一句', fatal: true })
}

const loadedEntry = entry.value
const canonical = computed(() => sayingsWorld?.entryPath(loadedEntry.short_id) ?? `/sayings/${loadedEntry.short_id}`)
const description = computed(() => markdownToText(loadedEntry.content_md, 160))

useSeoMeta({
  title: () => `片语 · Alive`,
  description,
  ogTitle: '片语 · Alive',
  ogDescription: description,
  ogType: 'article',
})

useHead({
  link: [{ rel: 'canonical', href: canonical }],
  script: [
    {
      type: 'application/ld+json',
      innerHTML: computed(() =>
        JSON.stringify({
          '@context': 'https://schema.org',
          '@type': 'SocialMediaPosting',
          url: canonical.value,
          articleBody: markdownToText(loadedEntry.content_md, 500),
          description: description.value,
          author: loadedEntry.author ? { '@type': 'Person', name: loadedEntry.author } : undefined,
          creator: loadedEntry.source || undefined,
          mainEntityOfPage: canonical.value,
        }),
      ),
    },
  ],
})
</script>

<template>
  <main class="detail">
    <NuxtLink class="back" to="/sayings">片语</NuxtLink>
    <SayingDetail :entry="loadedEntry" />
  </main>
</template>

<style scoped>
.detail {
  display: grid;
  gap: var(--space-6);
  padding-block: var(--space-4);
}

.back {
  color: var(--c-ink-muted);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  text-decoration: none;
}

.back:hover {
  color: var(--c-accent);
}
</style>
