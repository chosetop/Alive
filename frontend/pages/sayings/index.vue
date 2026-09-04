<script setup lang="ts">
import { onMounted, ref, nextTick } from 'vue'

import { resolvePublicWorld } from '~/content-worlds/registry'
import { useSayingsApi } from '~/composables/useSayingsApi'
import { consumeSayingAnchor, useSayingView } from '~/composables/useSayingView'
import { useSiteWorlds } from '~/composables/useSiteWorlds'
import { markdownToText } from '~/utils/markdown'

const route = useRoute()
const sayingsWorld = resolvePublicWorld('saying')
const { list } = useSayingsApi()
const { data: worldSettings } = await useSiteWorlds()

const sayingDefaultView = computed(() => {
  const setting = worldSettings.value.find((item) => item.world === 'saying')
  return setting?.default_view || 'stream'
})

const page = computed(() => {
  const raw = Number(route.query.page)
  return Number.isInteger(raw) && raw > 0 ? raw : 1
})

const { data, error } = await useAsyncData(
  () => `sayings-${page.value}`,
  () => list({ page: page.value }),
  { watch: [page] },
)

if (error.value) {
  throw createError({ statusCode: 503, message: '内容暂时读不到', fatal: true })
}

const items = computed(() => data.value?.data ?? [])
const meta = computed(() => data.value?.meta)
const { view, choose } = useSayingView(sayingDefaultView)
const heading = ref<HTMLElement | null>(null)
const pendingAnchor = import.meta.client && typeof window !== 'undefined' ? consumeSayingAnchor(window.history) : null

onMounted(async () => {
  if (pendingAnchor === null) return

  await nextTick()
  const selector = `[data-saying-id="${pendingAnchor.shortId.replace(/"/g, '\\"')}"]`
  const target = document.querySelector<HTMLElement>(selector)

  if (target !== null) {
    target.focus({ preventScroll: true })
    window.scrollTo({ top: pendingAnchor.offsetTop, behavior: 'auto' })
    return
  }

  heading.value?.focus({ preventScroll: true })
})

useSeoMeta({
  title: page.value > 1 ? `第 ${page.value} 页 · 片语 · Alive` : '片语 · Alive',
  description: '没有时间轴的短句、片语与轻量记录。',
  robots: page.value > 1 ? 'noindex, follow' : undefined,
})

useHead({
  link: [
    {
      rel: 'canonical',
      href: page.value > 1 ? `${sayingsWorld?.rootPath ?? '/sayings'}?page=${page.value}` : '/sayings',
    },
  ],
  script: [
    {
      type: 'application/ld+json',
      innerHTML: computed(() =>
        JSON.stringify({
          '@context': 'https://schema.org',
          '@type': 'CollectionPage',
          url: page.value > 1 ? `${sayingsWorld?.rootPath ?? '/sayings'}?page=${page.value}` : '/sayings',
          description: '没有时间轴的短句、片语与轻量记录。',
          about: '片语',
          mainEntity: items.value.map((item) => ({
            '@type': 'SocialMediaPosting',
            articleBody: markdownToText(item.content_md, 280),
          })),
        }),
      ),
    },
  ],
})
</script>

<template>
  <div class="sayings-page">
    <header class="head">
        <h1 ref="heading" tabindex="-1" class="title">片语</h1>
      <SayingViewPicker :view="view" @change="choose" />
    </header>

    <SayingStream v-if="view === 'stream'" :items="items" />
    <SayingWall v-else-if="view === 'wall'" :items="items" />
    <SayingStream v-else :items="items" />

    <p v-if="items.length === 0" class="empty">还没有公开的片语。</p>

    <ThePager
      v-if="meta"
      :page="meta.page"
      :page-size="meta.page_size"
      :total="meta.total"
      base-path="/sayings"
    />
  </div>
</template>

<style scoped>
.sayings-page {
  display: grid;
  gap: var(--space-7);
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-6);
}

.kicker {
  margin-bottom: var(--space-2);
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.title {
  margin-bottom: var(--space-2);
  font-family: var(--font-heading);
  font-size: clamp(1.75rem, 4vw, 2.6rem);
}

.desc {
  max-width: 32rem;
  color: var(--c-ink-muted);
}

.empty {
  padding-block: var(--space-9);
  color: var(--c-ink-faint);
}

@media (max-width: 42rem) {
  .head {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
