<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { categoriesApi, worldsApi, toUserMessage, type ApiClientError } from '../api'
import { ADMIN_WORLD_REGISTRY } from '../content-worlds/registry'
import type { AdminWorldSetting, WorldKey, WorldStatus, WorldViewMode } from '../types/api'
import { UiButton } from '../components/ui'

type DraftMap = Record<WorldKey, { status: WorldStatus; nav_label: string; default_view: WorldViewMode }>

const items = ref<AdminWorldSetting[]>([])
const drafts = ref<DraftMap | null>(null)
const isLoading = ref(true)
const savingWorld = ref<WorldKey | null>(null)
const error = ref<string | null>(null)
const categoryCounts = ref<Partial<Record<WorldKey, number>>>({})

async function load(): Promise<void> {
  isLoading.value = true
  error.value = null
  try {
    const settings = await worldsApi.listAdminWorlds()
    items.value = settings
    drafts.value = settings.reduce((acc, item) => {
      acc[item.world] = {
        status: item.status,
        nav_label: item.nav_label,
        default_view: item.default_view,
      }
      return acc
    }, {} as DraftMap)
  } catch (loadError) {
    error.value = toUserMessage(loadError)
  } finally {
    isLoading.value = false
  }
}

async function loadCategoryCounts(): Promise<void> {
  try {
    const grouped = await Promise.all(
      (['journal', 'saying', 'video'] as WorldKey[]).map(async (world) => [world, (await categoriesApi.listCategoriesAdmin({ world })).length] as const),
    )
    categoryCounts.value = Object.fromEntries(grouped)
  } catch {
    categoryCounts.value = {}
  }
}

onMounted(() => {
  void load()
  void loadCategoryCounts()
})

function itemFor(world: WorldKey): AdminWorldSetting | null {
  return items.value.find((item) => item.world === world) ?? null
}

function isDirty(world: WorldKey): boolean {
  const current = itemFor(world)
  const draft = drafts.value?.[world]
  return current !== null && draft !== undefined && (
    current.status !== draft.status ||
    current.nav_label !== draft.nav_label ||
    current.default_view !== draft.default_view
  )
}

function statusHelp(status: WorldStatus): string {
  if (status === 'open') return '前台可见，也允许发布。'
  if (status === 'hidden') return '前台暂时隐藏，已发布内容仍可通过链接访问。'
  return '不出现在前台导航，发布前需要先开放。'
}

async function save(world: WorldKey): Promise<void> {
  if (drafts.value === null || savingWorld.value !== null) return
  const current = itemFor(world)
  if (current === null) return

  savingWorld.value = world
  error.value = null
  try {
    const draft = drafts.value[world]
    const updated = await worldsApi.updateWorld(world, {
      revision: current.revision,
      status: draft.status,
      nav_label: draft.nav_label,
      default_view: draft.default_view,
    })
    items.value = items.value.map((item) => (item.world === world ? updated : item))
    drafts.value = {
      ...drafts.value,
      [world]: {
        status: updated.status,
        nav_label: updated.nav_label,
        default_view: updated.default_view,
      },
    }
  } catch (saveError) {
    const candidate = saveError as ApiClientError
    if (candidate?.status === 409) {
      await load()
      error.value = '设置已被其他保存更新，请先重新确认后再保存。'
    } else {
      error.value = toUserMessage(saveError)
    }
  } finally {
    savingWorld.value = null
  }
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

    <div v-else-if="drafts" class="list">
      <section
        v-for="world in ADMIN_WORLD_REGISTRY"
        :key="world.key"
        class="row"
        data-world-row
        :data-world-key="world.key"
      >
        <div class="row__copy">
          <h2>{{ world.label }}</h2>
          <p>{{ world.key }}</p>
          <small class="row__count">{{ categoryCounts[world.key] ?? '—' }} 个分类</small>
        </div>

        <label class="field">
          <span>状态</span>
          <select :data-world-status="world.key" :value="drafts[world.key].status" @change="drafts[world.key].status = ($event.target as HTMLSelectElement).value as WorldStatus">
            <option value="unopened">未开放</option>
            <option value="open">已开放</option>
            <option value="hidden">暂时隐藏</option>
          </select>
          <small class="field-help">{{ statusHelp(drafts[world.key].status) }}</small>
        </label>

        <label class="field">
          <span>导航名称</span>
          <input :value="drafts[world.key].nav_label" @input="drafts[world.key].nav_label = ($event.target as HTMLInputElement).value" />
        </label>

        <label class="field">
          <span>默认浏览</span>
          <select :value="drafts[world.key].default_view" @change="drafts[world.key].default_view = ($event.target as HTMLSelectElement).value as WorldViewMode">
            <option value="">跟随世界默认</option>
            <option v-if="world.key === 'saying'" value="stream">流式</option>
            <option v-if="world.key === 'saying'" value="wall">纸片墙</option>
            <option v-if="world.key === 'saying'" value="focus">一句模式</option>
          </select>
        </label>

        <UiButton
          variant="primary"
          :loading="savingWorld === world.key"
          :disabled="!isDirty(world.key)"
          :data-world-save="world.key"
          @click="void save(world.key)"
        >
          {{ isDirty(world.key) ? '保存修改' : '已保存' }}
        </UiButton>
      </section>
    </div>
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

.list {
  display: grid;
  gap: var(--space-4);
}

.row {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) repeat(3, minmax(0, 1fr)) auto;
  gap: var(--space-4);
  align-items: end;
  padding: var(--space-5);
  border: 1px solid var(--c-glass-border);
  border-radius: var(--radius-surface);
  background: var(--c-glass);
}

.row__copy h2 {
  margin-bottom: var(--space-1);
}

.row__copy p {
  color: var(--c-ink-faint);
  font-size: 0.75rem;
}

.row__count {
  display: block;
  margin-top: var(--space-2);
  color: var(--c-ink-muted);
  font-size: 0.75rem;
}

.field {
  display: grid;
  gap: var(--space-2);
}

.field span {
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.field-help {
  color: var(--c-ink-faint);
  font-size: 0.75rem;
  line-height: 1.45;
}

.field input,
.field select {
  min-height: 2.5rem;
  padding: 0.375rem var(--space-3);
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-control);
  background: var(--c-paper);
  color: var(--c-ink);
}

@media (max-width: 64rem) {
  .row {
    grid-template-columns: 1fr;
  }
}
</style>
