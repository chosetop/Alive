<script setup lang="ts">
import { computed, ref } from 'vue'

import { THEMES, type ThemeName } from '@alive/theme'
import { toUserMessage } from '../../api'
import { useThemeStore } from '../../stores/theme'
import { UiButton, UiMenu, type UiMenuItem } from '../ui'

const theme = useThemeStore()
const saving = ref(false)
const error = ref<string | null>(null)

const items = computed<UiMenuItem[]>(() =>
  THEMES.map((item) => ({
    id: item.name,
    label: item.label,
    hint: item.name === theme.preview ? '当前' : undefined,
  })),
)

function selectTheme(name: string): void {
  if (!THEMES.some((item) => item.name === name)) return
  theme.previewTheme(name as ThemeName)
  error.value = null
}

async function save(): Promise<void> {
  if (!theme.isDirty || saving.value) return
  saving.value = true
  error.value = null
  try {
    await theme.saveDefault()
  } catch (saveFailure) {
    error.value = toUserMessage(saveFailure)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="theme-picker" data-theme-picker>
    <UiMenu :items="items" label="选择主题" @select="selectTheme">
      <template #trigger>
        <button class="theme-trigger" type="button" data-theme-trigger>
          <span class="theme-swatch" aria-hidden="true" />
          <span>主题：{{ THEMES.find((item) => item.name === theme.preview)?.label }}</span>
        </button>
      </template>
    </UiMenu>

    <div v-if="theme.isDirty" class="theme-actions" data-theme-actions>
      <UiButton variant="primary" :loading="saving" data-theme-save @click="save">
        {{ saving ? '保存中…' : '设为站点默认' }}
      </UiButton>
      <UiButton variant="quiet" :disabled="saving" data-theme-cancel @click="theme.cancelPreview">
        取消预览
      </UiButton>
    </div>
    <p v-if="error" class="theme-error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.theme-picker {
  display: grid;
  gap: var(--space-2);
}

.theme-trigger {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  padding: var(--space-2) 0;
  border: 0;
  background: transparent;
  color: var(--c-ink-muted);
  font: inherit;
  font-size: 0.8125rem;
  text-align: left;
  cursor: pointer;
}

.theme-trigger:hover {
  color: var(--c-ink);
}

.theme-swatch {
  width: 0.75rem;
  height: 0.75rem;
  border: 1px solid var(--c-line-strong);
  border-radius: 50%;
  background: var(--c-accent);
}

.theme-actions {
  display: grid;
  gap: var(--space-2);
}

.theme-error {
  margin: 0;
  color: var(--c-danger);
  font-size: 0.8125rem;
}
</style>
