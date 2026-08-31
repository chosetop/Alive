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
    <section v-if="journals?.data?.length" aria-labelledby="journal-preview"><h2 id="journal-preview">日志</h2><EntryTimeline :entries="journals.data" /></section>
    <section v-if="sayings?.data?.length" aria-labelledby="saying-preview"><div class="section-head"><h2 id="saying-preview">片语</h2><NuxtLink to="/sayings">浏览全部</NuxtLink></div><div class="sayings"><NuxtLink v-for="item in sayings.data" :key="item.short_id" :to="`/sayings/${item.short_id}`">{{ item.content_md }}</NuxtLink></div></section>
  </div>
</template>

<style scoped>
.mixed-home{display:grid;gap:3rem}.mixed-home h2{font-size:var(--text-lg);margin-bottom:1rem}.section-head{display:flex;justify-content:space-between;align-items:center}.section-head a{font-size:var(--text-sm);color:var(--c-ink-muted)}.sayings{display:grid;gap:1rem}.sayings a{padding:1rem 0;border-bottom:1px solid var(--c-line);color:inherit;text-decoration:none;line-height:1.7}
</style>
