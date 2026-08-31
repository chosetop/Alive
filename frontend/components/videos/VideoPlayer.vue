<script setup lang="ts">
defineProps<{ src?: string; poster?: string }>()
</script>
<template><div class="player"><video v-if="src" :src="src" :poster="poster" controls playsinline preload="metadata" /><p v-else role="status">视频暂时不可用。</p></div></template>
<style scoped>.player video{display:block;width:100%;max-height:70vh;background:#000;border-radius:var(--radius-surface)}.player p{padding:var(--space-6);color:var(--c-ink-muted)}</style>
