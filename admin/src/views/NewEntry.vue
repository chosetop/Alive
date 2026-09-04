<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { worldsApi } from '../api'
import { listCreatableAdminWorlds } from '../content-worlds/registry'
import type { WorldKey, WorldStatus } from '../types/api'

const worlds = listCreatableAdminWorlds()
const statuses = ref<Partial<Record<WorldKey, WorldStatus>>>({})

onMounted(async () => {
  try {
    const settings = await worldsApi.listAdminWorlds()
    statuses.value = Object.fromEntries(settings.map((setting) => [setting.world, setting.status]))
  } catch {
    // The editor and publish panel still enforce the server's actual status.
  }
})

function statusCopy(world: WorldKey): string | null {
  const status = statuses.value[world]
  if (status === 'open') return '已开放，可发布'
  if (status === 'hidden') return '暂时隐藏，发布后不出现在导航'
  if (status === 'unopened') return '未开放，仅可保存草稿'
  return null
}
</script>

<template>
  <div class="page">
    <header class="head">
      <h1 class="title">选择内容世界</h1>
    </header>

    <div class="grid">
      <RouterLink
        v-for="world in worlds"
        :key="world.key"
        class="card"
        :to="{ name: world.editorRouteName!, params: { world: world.key } }"
      >
        <span class="card__eyebrow">{{ world.label }}</span>
        <strong class="card__title">{{ world.createLabel }}</strong>
        <span class="card__hint">
          {{ world.key === 'journal' ? '正文，可加入图片' : world.key === 'saying' ? '单句内容，可补充来源' : '主视频与封面' }}
        </span>
        <span v-if="statusCopy(world.key)" class="card__status">{{ statusCopy(world.key) }}</span>
      </RouterLink>
    </div>
  </div>
</template>

<style scoped>
.page {
  width: min(100%, 42rem);
  margin: 0 auto;
  padding: var(--space-8) var(--space-5);
}

.head {
  margin-bottom: var(--space-6);
}

.title {
  margin-bottom: var(--space-2);
  font-family: var(--font-heading);
  font-size: clamp(1.75rem, 4vw, 2.4rem);
}

.subtitle {
  color: var(--c-ink-muted);
}

.grid {
  display: grid;
  gap: var(--space-4);
}

.card {
  display: grid;
  gap: var(--space-2);
  padding: var(--space-5);
  border: 1px solid var(--c-glass-border);
  border-radius: var(--radius-surface);
  background: var(--c-glass);
  box-shadow: var(--shadow-control);
}

.card:hover {
  text-decoration: none;
  border-color: var(--c-accent);
}

.card__eyebrow {
  color: var(--c-ink-faint);
  font-size: 0.75rem;
}

.card__title {
  color: var(--c-ink);
  font-size: 1.125rem;
}

.card__hint,
.card__status {
  color: var(--c-ink-muted);
  font-size: 0.8125rem;
}

.card__status {
  color: var(--c-accent);
  font-size: 0.75rem;
}
</style>
