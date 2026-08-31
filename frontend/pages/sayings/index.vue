<script setup lang="ts">
import { useSayingsApi } from '~/composables/useSayingsApi'
import { useSayingView, resolveSayingView } from '~/composables/useSayingView'

const api = useSayingsApi()
const { data } = await useAsyncData('sayings', () => api.list())
const { view, choose } = useSayingView('stream')
const items = computed(() => data.value?.data ?? [])
</script>

<template>
  <main class="sayings-page" :data-view="resolveSayingView(view)">
    <header class="head"><h1>片语</h1><nav aria-label="片语浏览方式"><button v-for="mode in ['stream','wall','focus']" :key="mode" :aria-pressed="view === mode" @click="choose(mode as any)">{{ mode }}</button></nav></header>
    <section v-if="view === 'wall'" class="wall"><NuxtLink v-for="item in items" :key="item.short_id" :to="`/sayings/${item.short_id}`" class="card">{{ item.content_md }}</NuxtLink></section>
    <section v-else-if="view === 'focus'" class="focus"><NuxtLink v-if="items[0]" :to="`/sayings/${items[0].short_id}`">{{ items[0].content_md }}</NuxtLink></section>
    <section v-else class="stream"><NuxtLink v-for="item in items" :key="item.short_id" :to="`/sayings/${item.short_id}`" class="item">{{ item.content_md }}</NuxtLink></section>
  </main>
</template>

<style scoped>
.sayings-page{max-width:54rem;margin:auto;padding:2rem 1.25rem}.head{display:flex;justify-content:space-between;align-items:center;margin-bottom:2rem}.head nav{display:flex;gap:.4rem}.head button{border:1px solid var(--c-border);background:transparent;border-radius:999px;padding:.35rem .7rem}.stream{display:grid;gap:1.5rem}.item,.card,.focus a{color:inherit;text-decoration:none;line-height:1.8}.item{display:block;border-bottom:1px solid var(--c-border);padding-bottom:1.5rem}.wall{display:grid;grid-template-columns:repeat(auto-fit,minmax(13rem,1fr));gap:1rem}.card{padding:1.25rem;border:1px solid var(--c-border);border-radius:1rem}.focus{min-height:60vh;display:grid;place-items:center;text-align:center;font-size:clamp(1.5rem,4vw,2.5rem)}
</style>
