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
    <section v-if="journals?.data?.length" class="preview" aria-labelledby="journal-preview">
      <div class="section-head">
        <h2 id="journal-preview">日志</h2>
        <NuxtLink to="/journal">浏览全部</NuxtLink>
      </div>
      <EntryTimeline :entries="journals.data" />
    </section>

    <SayingsHomePreview v-if="sayings?.data?.length" :items="sayings.data" />
    <VideoHomePreview v-if="videos?.data?.length" :items="videos.data" />
  </div>
</template>

<style scoped>
.mixed-home{display:grid;gap:3rem}

.preview {
  display: grid;
  gap: var(--space-5);
  padding-top: var(--space-4);
}

.section-head {
  position: relative;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-4);
  padding-bottom: var(--space-3);
  border-bottom: 1px solid var(--c-line);
}

.section-head::after {
  position: absolute;
  bottom: -1px;
  left: 0;
  width: var(--space-7);
  height: 2px;
  background: var(--c-accent);
  content: '';
}

.section-head h2 {
  color: var(--c-ink);
  font-size: var(--text-xl);
  letter-spacing: var(--tracking-display);
}

.section-head a {
  color: var(--c-ink-muted);
  font-size: var(--text-sm);
  text-decoration: none;
}

.section-head a:hover {
  color: var(--c-accent);
}
</style>
