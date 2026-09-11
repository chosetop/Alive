<script setup lang="ts">
import { useVideosApi } from '~/composables/useVideosApi'
import { defaultWorldCategoryPath } from '~/content-worlds/registry'
const { list } = useVideosApi()
const { data: categories } = await useSiteCategories('video')
const defaultCategoryPath = defaultWorldCategoryPath('video', categories.value)
if (defaultCategoryPath) {
  await navigateTo(defaultCategoryPath, { redirectCode: 302, replace: true })
}
const { data, error } = await useAsyncData('videos', () => list())
if (error.value) throw createError({ statusCode: 503, message: '影像暂时读不到', fatal: true })
useSeoMeta({ title: '影像 · Alive' })
</script>
<template>
  <main class="videos">
    <WorldBrowseLayout>
      <template v-if="categories.length > 0" #navigation>
        <WorldCategoryNav world="video" :categories="categories" />
      </template>
      <section v-if="data?.data.length">
        <VideoCard v-for="item in data.data" :key="item.slug" :item="item" />
      </section>
      <p v-else>还没有公开的影像。</p>
    </WorldBrowseLayout>
  </main>
</template>
<style scoped>
.videos {
  min-width: 0;
}
</style>
