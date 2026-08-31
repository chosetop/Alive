<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'

import { entriesApi, toUserMessage } from '../api'
import { resolveAdminWorld } from '../content-worlds/registry'
import { useWritingStore } from '../stores/writing'
import type { EntryDetail } from '../types/api'
import EntryEditor from './EntryEditor.vue'
import SayingEditor from './SayingEditor.vue'

const props = defineProps<{
  id: string
}>()

const writing = useWritingStore()

const entry = ref<EntryDetail | null>(null)
const isLoading = ref(true)
const loadError = ref<string | null>(null)
const definition = computed(() => resolveAdminWorld(entry.value?.world))

let loadGeneration = 0

watch(
  () => props.id,
  (id) => {
    void load(id)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  loadGeneration += 1
})

async function load(id: string): Promise<void> {
  const generation = ++loadGeneration
  isLoading.value = true
  loadError.value = null
  entry.value = null

  try {
    const loaded = await entriesApi.getEntry(Number(id))
    if (generation !== loadGeneration) return

    const world = resolveAdminWorld(loaded.world)
    if (world === null) throw new Error(`Unsupported world: ${loaded.world}`)

    writing.setActiveWorld(world.key)
    entry.value = loaded
  } catch (cause) {
    if (generation !== loadGeneration) return
    writing.setActiveWorld(null)
    loadError.value =
      cause instanceof Error && cause.message.startsWith('Unsupported world:')
        ? cause.message
        : toUserMessage(cause)
  } finally {
    if (generation === loadGeneration) isLoading.value = false
  }
}
</script>

<template>
  <p v-if="isLoading" class="state page">载入中…</p>
  <p v-else-if="loadError" class="alert page" role="alert">{{ loadError }}</p>
  <SayingEditor
    v-else-if="entry && definition?.editorKind === 'saying'"
    :initial-entry="entry"
  />
  <EntryEditor
    v-else-if="entry && definition?.editorKind === 'long-form'"
    :id="String(entry.id)"
    :initial-entry="entry"
  />
</template>
