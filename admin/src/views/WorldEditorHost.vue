<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { entriesApi, toUserMessage } from '../api'
import { worldTransitionKey, writingRailKey } from '../composables/useWorldTransition'
import { resolveAdminWorld } from '../content-worlds/registry'
import { useWritingStore } from '../stores/writing'
import type { EntryDetail } from '../types/api'
import EntryEditor from './EntryEditor.vue'
import SayingEditor from './SayingEditor.vue'

const props = defineProps<{
  id: string
}>()

const writing = useWritingStore()
const router = useRouter()
const transition = inject(worldTransitionKey, null)
const railRef = inject(writingRailKey, null)
const editorSurface = ref<HTMLElement | null>(null)

const entry = ref<EntryDetail | null>(null)
const isLoading = ref(true)
const loadError = ref<string | null>(null)
const definition = computed(() => resolveAdminWorld(entry.value?.world))

let loadGeneration = 0
let workspaceEntered = false

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
  const currentEntry = entry.value
  const isSwitching = currentEntry !== null
  if (!isSwitching) {
    isLoading.value = true
    entry.value = null
  }
  loadError.value = null

  try {
    const loaded = await entriesApi.getEntry(Number(id))
    if (generation !== loadGeneration) return

    const world = resolveAdminWorld(loaded.world)
    if (world === null) throw new Error(`Unsupported world: ${loaded.world}`)

    if (
      isSwitching
      && currentEntry?.id !== loaded.id
      && transition !== null
      && editorSurface.value !== null
    ) {
      await transition.swapCanvas(editorSurface.value, async () => {
        entry.value = loaded
        writing.setActiveWorld(world.key)
        await nextTick()
      })
    } else {
      entry.value = loaded
      writing.setActiveWorld(world.key)
      await nextTick()
    }

    if (!workspaceEntered && transition !== null && editorSurface.value !== null) {
      await transition.enterWorkspace(railRef?.value ?? null, editorSurface.value)
      workspaceEntered = true
    }
  } catch (cause) {
    if (generation !== loadGeneration) return
    if (isSwitching && currentEntry !== null) {
      await router.replace({ name: 'entry-edit', params: { id: String(currentEntry.id) } })
    }
    if (!isSwitching) {
      entry.value = null
      writing.setActiveWorld(null)
    }
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
  <p v-if="isLoading && !entry" class="state page">载入中…</p>
  <p v-else-if="loadError && !entry" class="alert page" role="alert">{{ loadError }}</p>
  <section v-else-if="entry" ref="editorSurface" class="world-editor-host" data-world-editor-surface>
    <p v-if="loadError" class="alert page" role="alert">{{ loadError }}</p>
    <SayingEditor
      v-if="definition?.editorKind === 'saying'"
      :key="`saying-${entry.id}`"
      :initial-entry="entry"
    />
    <EntryEditor
      v-else-if="definition?.editorKind === 'long-form'"
      :key="`entry-${entry.id}`"
      :id="String(entry.id)"
      :initial-entry="entry"
    />
  </section>
</template>

<style scoped>
.world-editor-host {
  min-height: 100%;
}
</style>
