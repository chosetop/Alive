<script setup lang="ts">
const { list: listJournals } = useEntriesApi()
const { list: listSayings } = useSayingsApi()
const [{ data: journals }, { data: sayings }] = await Promise.all([
  useAsyncData('home-journals', () => listJournals({ page: 1, page_size: 5 })),
  useAsyncData('home-sayings', () => listSayings({ page: 1, page_size: 3 })),
])
</script>

<template>
  <div class="mixed-home">
    <section v-if="journals?.data?.length" aria-labelledby="journal-preview">
      <h2 id="journal-preview">日志</h2>
      <EntryTimeline :entries="journals.data" />
    </section>

    <SayingsHomePreview v-if="sayings?.data?.length" :items="sayings.data" />
  </div>
</template>

<style scoped>
.mixed-home{display:grid;gap:3rem}
.mixed-home h2{font-size:var(--text-lg);margin-bottom:1rem}
</style>
