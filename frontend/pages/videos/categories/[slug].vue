<script setup lang="ts">
import { useVideosApi } from '~/composables/useVideosApi'
import { useSiteCategories } from '~/composables/useSiteCategories'

const route = useRoute()
const slug = computed(() => String(route.params.slug))
const page = computed(() => { const value = Number(route.query.page); return Number.isInteger(value) && value > 0 ? value : 1 })
const { list } = useVideosApi()
const { data: categories } = await useSiteCategories('video')
const category = computed(() => categories.value.find((item) => item.slug === slug.value) ?? null)
const { data, error } = await useAsyncData(() => `videos-category-${slug.value}-${page.value}`, () => list({ category: slug.value, page: page.value }), { watch: [slug, page] })
if (error.value) throw createError({ statusCode: 503, message: '影像暂时读不到', fatal: true })
useSeoMeta({ title: () => `${category.value?.name ?? slug.value} · 影像 · Alive` })
</script>

<template>
  <main class="videos-category">
    <WorldBrowseLayout>
      <template #header>
        <header><NuxtLink to="/videos">返回影像</NuxtLink><h1>{{ category?.name ?? slug }}</h1></header>
      </template>
      <template #navigation>
        <WorldCategoryNav world="video" :categories="categories" :active-slug="slug" />
      </template>
      <section v-if="data?.data.length"><VideoCard v-for="item in data.data" :key="item.slug" :item="item" /></section>
      <p v-else>这个分类下还没有公开的影像。</p>
      <ThePager v-if="data?.meta" :page="data.meta.page" :page-size="data.meta.page_size" :total="data.meta.total" :base-path="`/videos/categories/${slug}`" />
    </WorldBrowseLayout>
  </main>
</template>

<style scoped>.videos-category{min-width:0}header{display:grid;gap:var(--space-2)}</style>
