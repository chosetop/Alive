<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { categoriesApi, entriesApi, worldsApi, toUserMessage } from '../api'
import WorldDeskCard, { type WorldDeskSnapshot } from '../components/worlds/WorldDeskCard.vue'
import WorldSettingsPanel from '../components/worlds/WorldSettingsPanel.vue'
import { ADMIN_WORLD_REGISTRY } from '../content-worlds/registry'
import type { AdminWorldSetting, WorldKey } from '../types/api'

const items = ref<AdminWorldSetting[]>([])
const snapshots = ref<Partial<Record<WorldKey, WorldDeskSnapshot>>>({})
const activeWorld = ref<WorldKey | null>(null)
const isLoading = ref(true)
const error = ref<string | null>(null)
const router = useRouter()

async function load(): Promise<void> {
  isLoading.value = true
  error.value = null
  try {
    const settings = await worldsApi.listAdminWorlds()
    items.value = settings
    await loadSnapshots(settings)
  } catch (loadError) {
    error.value = toUserMessage(loadError)
  } finally {
    isLoading.value = false
  }
}

async function loadSnapshots(settings: AdminWorldSetting[]): Promise<void> {
  const nextEntries = await Promise.all(
    ADMIN_WORLD_REGISTRY.map(async (definition) => {
      const setting = settings.find((item) => item.world === definition.key)
      if (!setting) return null
      return [definition.key, await loadSnapshot(definition.key, setting)] as const
    }),
  )

  snapshots.value = Object.fromEntries(
    nextEntries.filter((entry): entry is readonly [WorldKey, WorldDeskSnapshot] => entry !== null),
  )
}

async function loadSnapshot(world: WorldKey, setting: AdminWorldSetting): Promise<WorldDeskSnapshot> {
  const [entriesResult, categoriesResult] = await Promise.allSettled([
    entriesApi.listEntriesAdmin({ world, page_size: 1 }),
    categoriesApi.listCategoriesAdmin({ world }),
  ])

  let recentEntry = null
  let entryCount = 0
  let categoryCount = 0
  let errorMessage: string | null = null
  let canEnter = true
  let canRetry = false

  if (entriesResult.status === 'fulfilled') {
    recentEntry = entriesResult.value.data[0] ?? null
    entryCount = entriesResult.value.meta.total
  } else {
    errorMessage = '暂时无法读取最近编辑，请先重试。'
    canEnter = false
    canRetry = true
  }

  if (categoriesResult.status === 'fulfilled') {
    categoryCount = categoriesResult.value.length
  } else if (errorMessage === null) {
    errorMessage = '暂时无法读取分类统计，但仍可进入工作台。'
  }

  return {
    setting,
    recentEntry,
    entryCount,
    categoryCount,
    error: errorMessage,
    canEnter,
    canRetry,
  }
}

onMounted(() => {
  void load()
})

function itemFor(world: WorldKey): AdminWorldSetting | null {
  return items.value.find((item) => item.world === world) ?? null
}

const activeSetting = computed(() => (activeWorld.value ? itemFor(activeWorld.value) : null))
const activeDefinition = computed(() =>
  activeWorld.value ? ADMIN_WORLD_REGISTRY.find((item) => item.key === activeWorld.value) ?? null : null,
)
const desks = computed(() =>
  ADMIN_WORLD_REGISTRY.flatMap((definition) => {
    const snapshot = snapshots.value[definition.key]
    return snapshot ? [{ definition, snapshot }] : []
  }),
)

async function enterWorld(world: WorldKey): Promise<void> {
  const definition = ADMIN_WORLD_REGISTRY.find((item) => item.key === world)
  const snapshot = snapshots.value[world]
  if (!definition || !snapshot) return
  if (!snapshot.canEnter) return

  if (snapshot.recentEntry) {
    await router.push({ name: 'entry-edit', params: { id: String(snapshot.recentEntry.id) } })
    return
  }

  if (definition.editorRouteName) {
    await router.push({ name: definition.editorRouteName, params: { world: definition.key } })
  }
}

function openSettings(world: WorldKey): void {
  activeWorld.value = world
}

async function retryWorld(world: WorldKey): Promise<void> {
  const setting = itemFor(world)
  if (setting === null) return

  const next = await loadSnapshot(world, setting)
  snapshots.value = {
    ...snapshots.value,
    [world]: next,
  }
}

function closeSettings(open: boolean): void {
  if (!open) activeWorld.value = null
}

function handleSaved(updated: AdminWorldSetting): void {
  items.value = items.value.map((item) => (item.world === updated.world ? updated : item))
  const snapshot = snapshots.value[updated.world]
  if (!snapshot) return
  snapshots.value = {
    ...snapshots.value,
    [updated.world]: {
      ...snapshot,
      setting: updated,
      error: null,
    },
  }
}

async function reloadSetting(world: WorldKey): Promise<AdminWorldSetting> {
  const settings = await worldsApi.listAdminWorlds()
  items.value = settings
  const next = settings.find((item) => item.world === world)
  if (!next) throw new Error(`missing setting for ${world}`)

  const snapshot = snapshots.value[world]
  if (snapshot) {
    snapshots.value = {
      ...snapshots.value,
      [world]: {
        ...snapshot,
        setting: next,
      },
    }
  }
  return next
}

async function reloadActiveSetting(): Promise<AdminWorldSetting> {
  if (!activeWorld.value) throw new Error('没有正在编辑的世界设置。')
  return reloadSetting(activeWorld.value)
}
</script>

<template>
  <div class="page">
    <header class="head">
      <div>
        <h1 class="title">内容世界</h1>
        <p class="subtitle">控制哪些世界已开放，以及它们在站点上的名称和默认浏览方式。</p>
      </div>
    </header>

    <p v-if="error" class="alert" role="alert">{{ error }}</p>
    <p v-if="isLoading" class="state">载入中…</p>

    <div v-else-if="items.length" class="layout">
      <div class="list">
        <WorldDeskCard
          v-for="desk in desks"
          :key="desk.definition.key"
          :definition="desk.definition"
          :snapshot="desk.snapshot"
          @enter="(nextWorld) => void enterWorld(nextWorld)"
          @settings="openSettings"
          @retry="(nextWorld) => void retryWorld(nextWorld)"
        />
      </div>

      <WorldSettingsPanel
        v-if="activeDefinition && activeSetting"
        :definition="activeDefinition"
        :setting="activeSetting"
        :open="activeWorld !== null"
        :reload="reloadActiveSetting"
        @saved="handleSaved"
        @update:open="closeSettings"
      />
    </div>

    <p v-else class="state">暂时没有可显示的世界设置。</p>
  </div>
</template>

<style scoped>
.page {
  max-width: 60rem;
}

.head {
  margin-bottom: var(--space-6);
}

.title {
  margin-bottom: var(--space-2);
  font-family: var(--font-heading);
  font-size: clamp(1.5rem, 3vw, 2rem);
}

.subtitle {
  color: var(--c-ink-muted);
}

.layout {
  position: relative;
}

.list {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-4);
}

@media (max-width: 72rem) {
  .list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 48rem) {
  .list {
    grid-template-columns: 1fr;
  }
}
</style>
