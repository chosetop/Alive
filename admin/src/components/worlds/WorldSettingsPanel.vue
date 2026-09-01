<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { worldsApi, toUserMessage, type ApiClientError } from '../../api'
import type { AdminWorldDefinition } from '../../content-worlds/registry'
import type { AdminWorldSetting, WorldStatus, WorldViewMode } from '../../types/api'
import { UiButton, UiIcon, UiIconButton } from '../ui'

interface DraftState {
  status: WorldStatus
  navLabel: string
  defaultView: WorldViewMode
}

const props = defineProps<{
  definition: AdminWorldDefinition
  setting: AdminWorldSetting
  open: boolean
  reload: () => Promise<AdminWorldSetting>
}>()

const emit = defineEmits<{
  saved: [AdminWorldSetting]
  'update:open': [boolean]
}>()

const busy = ref(false)
const error = ref<string | null>(null)
const draft = ref<DraftState>(buildDraft(props.setting))

const isDirty = computed(() =>
  draft.value.status !== props.setting.status ||
  draft.value.navLabel !== props.setting.nav_label ||
  draft.value.defaultView !== props.setting.default_view,
)

function buildDraft(setting: AdminWorldSetting): DraftState {
  return {
    status: setting.status,
    navLabel: setting.nav_label,
    defaultView: setting.default_view,
  }
}

function statusHelp(status: WorldStatus): string {
  if (status === 'open') return '前台可见，也允许发布。'
  if (status === 'hidden') return '前台暂时隐藏，已发布内容仍可通过链接访问。'
  return '不出现在前台导航，发布前需要先开放。'
}

watch(
  () => [props.setting, props.open] as const,
  ([setting]) => {
    draft.value = buildDraft(setting)
    error.value = null
  },
  { immediate: true },
)

async function save(): Promise<void> {
  if (!props.open || !isDirty.value || busy.value) return

  busy.value = true
  error.value = null

  try {
    const updated = await worldsApi.updateWorld(props.setting.world, {
      revision: props.setting.revision,
      status: draft.value.status,
      nav_label: draft.value.navLabel,
      default_view: draft.value.defaultView,
    })
    draft.value = buildDraft(updated)
    emit('saved', updated)
  } catch (saveError) {
    const candidate = saveError as ApiClientError
    if (candidate?.status === 409) {
      const reloaded = await props.reload()
      draft.value = buildDraft(reloaded)
      error.value = '设置已被其他保存更新，请重新确认。'
    } else {
      error.value = toUserMessage(saveError)
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <aside
    v-if="open"
    class="world-settings"
    role="dialog"
    aria-labelledby="world-settings-title"
    data-world-settings-panel
    :data-world-key="setting.world"
    data-surface="elevated"
  >
    <div class="world-settings__header">
      <div>
        <p class="world-settings__eyebrow">{{ definition.label }}</p>
        <h2 id="world-settings-title">世界设置</h2>
      </div>
      <UiIconButton label="关闭世界设置" data-world-close @click="emit('update:open', false)">
        <UiIcon name="close" />
      </UiIconButton>
    </div>

    <p class="world-settings__description">{{ definition.description }}</p>
    <p v-if="error" class="world-settings__alert" role="alert">{{ error }}</p>

    <div class="world-settings__body">
      <label class="field">
        <span>状态</span>
        <select
          data-world-status
          :value="draft.status"
          :disabled="busy"
          @change="draft.status = ($event.target as HTMLSelectElement).value as WorldStatus"
        >
          <option value="unopened">未开放</option>
          <option value="open">已开放</option>
          <option value="hidden">暂时隐藏</option>
        </select>
        <small class="field-help">{{ statusHelp(draft.status) }}</small>
      </label>

      <label class="field">
        <span>导航名称</span>
        <input
          :value="draft.navLabel"
          :disabled="busy"
          @input="draft.navLabel = ($event.target as HTMLInputElement).value"
        />
      </label>

      <label v-if="definition.key === 'saying'" class="field">
        <span>默认浏览</span>
        <select
          data-world-default-view
          :value="draft.defaultView"
          :disabled="busy"
          @change="draft.defaultView = ($event.target as HTMLSelectElement).value as WorldViewMode"
        >
          <option value="">跟随世界默认</option>
          <option value="stream">流式</option>
          <option value="wall">纸片墙</option>
          <option value="focus">一句模式</option>
        </select>
      </label>
    </div>

    <div class="world-settings__footer">
      <UiButton variant="quiet" :disabled="busy" @click="emit('update:open', false)">收起</UiButton>
      <UiButton
        variant="primary"
        data-world-save
        :disabled="!isDirty || busy"
        :loading="busy"
        @click="save"
      >
        {{ isDirty ? '保存设置' : '已保存' }}
      </UiButton>
    </div>
  </aside>
</template>

<style scoped>
.world-settings {
  position: fixed;
  inset-block: 0;
  inset-inline-end: 0;
  z-index: 40;
  display: grid;
  align-content: start;
  gap: var(--space-4);
  width: min(28rem, calc(100vw - var(--space-4)));
  max-width: 100vw;
  padding: var(--space-5);
  overflow-y: auto;
  border-inline-start: 1px solid var(--c-glass-border);
  border-radius: var(--radius-surface) 0 0 var(--radius-surface);
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--c-paper) 86%, transparent), transparent 24%),
    var(--c-surface);
  box-shadow: var(--shadow-float);
  overscroll-behavior: contain;
}

.world-settings__header {
  display: flex;
  justify-content: space-between;
  gap: var(--space-3);
}

.world-settings__header h2 {
  margin: 0;
  font-size: 1rem;
}

.world-settings__eyebrow {
  margin: 0 0 var(--space-2);
  color: var(--c-ink-faint);
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.world-settings__description {
  margin: 0;
  color: var(--c-ink-muted);
  line-height: 1.65;
}

.world-settings__alert {
  margin: 0;
  padding: var(--space-3);
  border-radius: var(--radius-control);
  background: var(--c-danger-surface);
  color: var(--c-danger);
  font-size: 0.8125rem;
  line-height: 1.55;
}

.world-settings__body {
  display: grid;
  gap: var(--space-4);
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

.world-settings input,
.world-settings select {
  width: 100%;
  min-height: 2.75rem;
  padding: 0.5rem var(--space-3);
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-control);
  background: var(--c-paper);
  color: var(--c-ink);
}

.world-settings__footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-2);
}

@media (max-width: 23.4375rem) {
  .world-settings {
    width: 100vw;
    padding: var(--space-4);
    border-radius: 0;
    overflow-x: clip;
  }
}
</style>
