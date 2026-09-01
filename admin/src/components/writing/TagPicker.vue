<script setup lang="ts">
import { ref } from 'vue'
import { tagsApi } from '../../api'
import type { Tag } from '../../api/tags'

const props = withDefaults(defineProps<{ entryId: number; revision: number; selected: Tag[]; disabled?: boolean }>(), { disabled: false })
const emit = defineEmits<{ saved: [revision: number, tags: Tag[]] }>()
const query = ref('')
const results = ref<Tag[]>([])
const busy = ref(false)
const error = ref<string | null>(null)

async function search(): Promise<void> {
  busy.value = true
  try { results.value = await tagsApi.listTags({ q: query.value.trim(), limit: 20 }); error.value = null }
  catch { error.value = '标签暂时读不到' }
  finally { busy.value = false }
}
async function toggle(tag: Tag): Promise<void> {
  if (props.disabled || busy.value) return
  const ids = props.selected.some((item) => item.id === tag.id) ? props.selected.filter((item) => item.id !== tag.id).map((item) => item.id) : [...props.selected, tag].map((item) => item.id)
  if (ids.length > 20) return
  busy.value = true
  try {
    const saved = await tagsApi.replaceEntryTags(props.entryId, props.revision, ids)
    emit('saved', saved.revision, ids.map((id) => [...props.selected, tag].find((item) => item.id === id)!).filter(Boolean))
    error.value = null
  } catch { error.value = '标签保存失败，请重试' }
  finally { busy.value = false }
}
</script>
<template>
  <section class="tag-picker" aria-label="标签">
    <div class="selected"><button v-for="tag in selected" :key="tag.id" type="button" :disabled="disabled" @click="void toggle(tag)">{{ tag.name }} ×</button><span v-if="selected.length === 0">尚未添加标签</span></div>
    <input v-model="query" type="search" placeholder="搜索标签" :disabled="disabled" @focus="void search()" @input="void search()" />
    <div v-if="busy">搜索中…</div><p v-if="error" role="alert">{{ error }}</p>
    <ul v-else><li v-for="tag in results" :key="tag.id"><button type="button" :disabled="disabled" :aria-pressed="selected.some((item) => item.id === tag.id)" @click="void toggle(tag)">{{ tag.name }}</button></li></ul>
  </section>
</template>
<style scoped>.tag-picker{display:grid;gap:.6rem}.selected{display:flex;gap:.4rem;flex-wrap:wrap;color:var(--c-ink-muted)}button{border:1px solid var(--c-line);border-radius:999px;background:var(--c-surface);padding:.35rem .65rem;color:inherit}input{padding:.55rem;border:1px solid var(--c-line);border-radius:.5rem;background:var(--c-paper)}ul{display:flex;gap:.4rem;flex-wrap:wrap;padding:0;list-style:none}p{color:var(--c-danger)}</style>
