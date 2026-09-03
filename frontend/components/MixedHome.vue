<script setup lang="ts">
import VideoHomePreview from '~/components/home/VideoHomePreview.vue'

const { list: listJournals } = useEntriesApi()
const { list: listSayings } = useSayingsApi()
const { list: listVideos } = useVideosApi()
const [{ data: journals }, { data: sayings }, { data: videos }] = await Promise.all([
  useAsyncData('home-journals', () => listJournals({ page: 1, page_size: 5 })),
  useAsyncData('home-sayings', () => listSayings({ page: 1, page_size: 3 })),
  useAsyncData('home-videos', () => listVideos({ page: 1, page_size: 3 })),
])
</script>

<template>
  <div class="mixed-home">
    <section v-if="journals?.data?.length" aria-labelledby="journal-preview">
      <h2 id="journal-preview">日志</h2>
      <EntryTimeline :entries="journals.data" />
    </section>

    <SayingsHomePreview v-if="sayings?.data?.length" :items="sayings.data" />
    <VideoHomePreview v-if="videos?.data?.length" :items="videos.data" />
  </div>
</template>

<style scoped>
.mixed-home{display:grid;gap:3rem}
.mixed-home h2{font-size:var(--text-lg);margin-bottom:1rem}
#journal-preview{font-size:1.5rem}
</style>
